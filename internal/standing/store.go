package standing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/memory"
)

// The store's sentinel conditions. Callers branch on these with errors.Is
// and never on message text.
var (
	// ErrBadShopper is returned when a shopper id cannot name a file: it
	// is empty or carries a character outside the file-safe set.
	ErrBadShopper = errors.New("standing: shopper id cannot name a file")
	// ErrBadDir is returned when the store directory is missing.
	ErrBadDir = errors.New("standing: missing store directory")
	// ErrCorrupt is returned when a stored file does not decode.
	ErrCorrupt = errors.New("standing: stored journal does not decode")
)

// snapshotVersion is the file layout version. A reader that meets a
// newer version refuses rather than guess.
const snapshotVersion = 1

// Snapshot is one shopper's durable layer-owned state: the pooling
// choice, the correction views recall serves, and the undeclared journal
// entries a later belief may declare against. Beliefs, evidence and the
// graph session are not in it: they are rebuilt from new writes, and the
// journal's standings are re-declared then.
type Snapshot struct {
	// Version is the file layout version.
	Version int `json:"version"`
	// Pooling is the shopper's pooling choice. Pooling is opt-out, so a
	// snapshot that never recorded a choice pools.
	Pooling bool `json:"pooling"`
	// Corrections are the correction views recall serves, newest last:
	// declared corrections with the belief they contradict, named
	// standings waiting for a matching belief, and unscoped standings
	// that apply to every brand until one is known.
	Corrections []contract.CorrectionView `json:"corrections"`
	// Journal holds the undeclared standing corrections, oldest first.
	// Every entry stands on its own: none carries an against declaration.
	Journal []memory.Correction `json:"journal"`
}

// Store persists one snapshot per shopper. It is safe for concurrent use.
type Store interface {
	// Save replaces one shopper's snapshot. It is called after every
	// mutation, so a crash loses at most the write in flight.
	Save(ctx context.Context, shopper string, snap Snapshot) error
	// Load returns one shopper's snapshot. Found is false when the
	// shopper has no snapshot yet, which is not an error: the caller
	// starts from the defaults.
	Load(ctx context.Context, shopper string) (Snapshot, bool, error)
}

// validShopper reports whether a shopper id can name a file: non-empty,
// with no path separator or dot-dot games.
func validShopper(shopper string) bool {
	if shopper == "" || shopper == "." || shopper == ".." {
		return false
	}
	for i := 0; i < len(shopper); i++ {
		c := shopper[i]
		switch {
		case c >= 'A' && c <= 'Z':
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '.' || c == '_' || c == '-' || c == ':':
		default:
			return false
		}
	}
	return !strings.Contains(shopper, "..")
}

// checkShopper refuses a shopper id that cannot name a file.
func checkShopper(shopper string) error {
	if !validShopper(shopper) {
		return fmt.Errorf("%w: %q", ErrBadShopper, shopper)
	}
	return nil
}

// encode renders one snapshot: the version first, then the pooling
// choice, the corrections and the journal.
func encode(snap Snapshot) ([]byte, error) {
	snap.Version = snapshotVersion
	raw, err := json.Marshal(snap)
	if err != nil {
		return nil, fmt.Errorf("standing: encode snapshot: %w", err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return nil, fmt.Errorf("standing: encode snapshot: %w", err)
	}
	return append(compact.Bytes(), '\n'), nil
}

// decode reads one snapshot and refuses a version it does not know.
func decode(raw []byte) (Snapshot, error) {
	var snap Snapshot
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snap); err != nil {
		return Snapshot{}, fmt.Errorf("%w: %w", ErrCorrupt, err)
	}
	if snap.Version != snapshotVersion {
		return Snapshot{}, fmt.Errorf("%w: version %d", ErrCorrupt, snap.Version)
	}
	if snap.Corrections == nil {
		snap.Corrections = []contract.CorrectionView{}
	}
	if snap.Journal == nil {
		snap.Journal = []memory.Correction{}
	}
	return snap, nil
}

// DirStore keeps one file per shopper in a directory: the pooling choice,
// the correction views and the undeclared journal survive a restart.
// Writes are atomic: the file is written whole to a temporary sibling and
// renamed over the old one, so a crash leaves the last complete snapshot.
type DirStore struct {
	mu  sync.Mutex
	dir string
}

// NewDirStore builds a file store in a directory. The directory must
// exist: the store creates files, never directories.
func NewDirStore(dir string) (*DirStore, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, ErrBadDir
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w: %q", ErrBadDir, dir)
	}
	return &DirStore{dir: dir}, nil
}

// path names one shopper's file. The caller holds the lock.
func (s *DirStore) path(shopper string) string {
	return filepath.Join(s.dir, shopper+".json")
}

// Save replaces one shopper's snapshot atomically.
func (s *DirStore) Save(ctx context.Context, shopper string, snap Snapshot) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("standing: save snapshot for %q: %w", shopper, err)
	}
	if err := checkShopper(shopper); err != nil {
		return err
	}
	raw, err := encode(snap)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tmp, err := os.CreateTemp(s.dir, shopper+".*.tmp")
	if err != nil {
		return fmt.Errorf("standing: save snapshot for %q: %w", shopper, err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("standing: save snapshot for %q: %w", shopper, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("standing: save snapshot for %q: %w", shopper, err)
	}
	if err := os.Rename(tmpName, s.path(shopper)); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("standing: save snapshot for %q: %w", shopper, err)
	}
	return nil
}

// Load returns one shopper's snapshot, or found false when the shopper
// has none yet.
func (s *DirStore) Load(ctx context.Context, shopper string) (Snapshot, bool, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, false, fmt.Errorf("standing: load snapshot for %q: %w", shopper, err)
	}
	if err := checkShopper(shopper); err != nil {
		return Snapshot{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(s.path(shopper))
	if err != nil {
		if os.IsNotExist(err) {
			return Snapshot{}, false, nil
		}
		return Snapshot{}, false, fmt.Errorf("standing: load snapshot for %q: %w", shopper, err)
	}
	snap, err := decode(raw)
	if err != nil {
		return Snapshot{}, false, fmt.Errorf("standing: load snapshot for %q: %w", shopper, err)
	}
	return snap, true, nil
}

// MemStore keeps snapshots in memory for tests. It persists nothing: a
// fresh MemStore forgets every shopper, which is how the tests prove a
// restart loses the journal without a store.
type MemStore struct {
	mu    sync.Mutex
	snaps map[string]Snapshot
}

// NewMemStore builds an empty in-memory store.
func NewMemStore() *MemStore {
	return &MemStore{snaps: map[string]Snapshot{}}
}

// Save stores one shopper's snapshot.
func (s *MemStore) Save(ctx context.Context, shopper string, snap Snapshot) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("standing: save snapshot for %q: %w", shopper, err)
	}
	if err := checkShopper(shopper); err != nil {
		return err
	}
	raw, err := encode(snap)
	if err != nil {
		return err
	}
	decoded, err := decode(raw)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snaps[shopper] = decoded
	return nil
}

// Load returns one shopper's snapshot, or found false when absent.
func (s *MemStore) Load(ctx context.Context, shopper string) (Snapshot, bool, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, false, fmt.Errorf("standing: load snapshot for %q: %w", shopper, err)
	}
	if err := checkShopper(shopper); err != nil {
		return Snapshot{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, ok := s.snaps[shopper]
	if !ok {
		return Snapshot{}, false, nil
	}
	return snap, true, nil
}
