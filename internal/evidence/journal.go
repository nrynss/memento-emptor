package evidence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
)

// journalVersion is the file layout version. A reader that meets a newer
// version refuses rather than guess.
const journalVersion = 1

// Snapshot is one shopper's durable evidence state: the documents behind
// the graph's stubs, the ledger rows with their ids, reasons, links, write
// operations and feedback markers, the open questions, the evidence order
// each belief cites, the timelines the evidence wrote, the mint counters,
// and the last brand context. It is the partition a restart rebuilds
// before the boot seed replays.
type Snapshot struct {
	// Version is the file layout version.
	Version int `json:"version"`
	// Documents are the raw-evidence texts behind the doc ids, in index
	// order, so a restored store mints the next id after them and search
	// answers as it did before.
	Documents []Document `json:"documents"`
	// Rows are the ledger's evidence rows. A row keeps its id, its
	// outcome, item, reason and link, what it counts toward, how it was
	// written, the document behind its graph stub, the beliefs it cites
	// and its feedback marker. Rows are ordered by id so a save is
	// deterministic; the per-belief citations carry their own order.
	Rows []Row `json:"rows"`
	// Pending are the open questions, so a refund awaiting its reason
	// can still be answered after a restart.
	Pending []Pending `json:"pending"`
	// Links carry each belief's evidence citations in order and when the
	// last one changed the belief. A boot replay rebuilds the belief and
	// appends exactly these rows, so recall cites them in the same order
	// and the ledger shows the same evidence count and change time.
	Links []BeliefEvidence `json:"links"`
	// Timelines carry each belief's history, so explain_belief keeps the
	// rows the live evidence wrote.
	Timelines []BeliefTimeline `json:"timelines"`
	// Seq is the per-prefix mint counter, so no id a live write minted
	// is minted again after a restart.
	Seq map[string]int `json:"seq"`
	// LastBrand is the shopper's brand context as of the last write that
	// saved the partition, so a correction recorded after a restart keeps
	// the brand the previous boot was serving.
	LastBrand domain.Subject `json:"last_brand"`
}

// Row is one ledger evidence row in its durable form.
type Row struct {
	// ID is the row's wire id.
	ID string `json:"id"`
	// Evidence is the domain row: outcome, item, reason and link.
	Evidence domain.Evidence `json:"evidence"`
	// CountsThatToward is the belief kind the row counts toward, or none.
	CountsThatToward domain.BeliefKind `json:"counts_toward"`
	// WriteOp is how the signal was written.
	WriteOp domain.WriteOp `json:"write_op"`
	// DocID is the document behind the row's graph stub, or empty while
	// no write indexed one.
	DocID string `json:"doc_id"`
	// BeliefIDs are the beliefs the row counts toward.
	BeliefIDs []string `json:"belief_ids"`
	// Feedback is the feedback marker a capture row carries, or nil.
	Feedback *Feedback `json:"feedback"`
}

// Feedback is one purchase's recorded feedback, durable: the id it was
// recorded under, the verdict that recorded it, and the belief it counts
// toward.
type Feedback struct {
	// ID is the feedback id.
	ID string `json:"id"`
	// Verdict is the verdict the feedback recorded.
	Verdict contract.FeedbackVerdict `json:"verdict"`
	// BeliefID is the belief the feedback counts toward.
	BeliefID string `json:"belief_id"`
}

// Pending is one refund still missing its reason, durable: the question,
// the refund behind it, when it was first asked, and how often.
type Pending struct {
	// ID is the question's wire id.
	ID string `json:"id"`
	// EventID is the webhook event that filed the question.
	EventID string `json:"event_id"`
	// Outcome is the refund awaiting its reason.
	Outcome domain.Outcome `json:"outcome"`
	// Items are the refund's order items.
	Items []domain.Item `json:"items"`
	// Link ties the refund to its transaction.
	Link domain.TransactionLink `json:"link"`
	// Seeded marks generated history.
	Seeded bool `json:"seeded"`
	// AskedCount counts how often the agent asked.
	AskedCount int `json:"asked_count"`
	// LastAskedAt is when the agent last asked, or nil.
	LastAskedAt *time.Time `json:"last_asked_at"`
	// AwaitingSince is the refund's event time.
	AwaitingSince time.Time `json:"awaiting_since"`
}

// BeliefEvidence is one belief's evidence citations in order, with when
// the last of them changed the belief. A boot replay rebuilds the belief
// and appends the rows it does not already cite, so the order the live
// writes produced survives.
type BeliefEvidence struct {
	// BeliefID identifies the belief.
	BeliefID string `json:"belief_id"`
	// EvidenceIDs are the row ids the belief cites, oldest first.
	EvidenceIDs []string `json:"evidence_ids"`
	// UpdatedAt is when the belief last changed.
	UpdatedAt time.Time `json:"updated_at"`
}

// BeliefTimeline is one belief's history in order.
type BeliefTimeline struct {
	// BeliefID identifies the belief.
	BeliefID string `json:"belief_id"`
	// Events are the timeline rows, oldest first.
	Events []contract.TimelineEvent `json:"events"`
}

// validJournalShopper reports whether a shopper id can name a file:
// non-empty, with no path separator or dot-dot games.
func validJournalShopper(shopper string) bool {
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

// checkJournalShopper refuses a shopper id that cannot name a file.
func checkJournalShopper(shopper string) error {
	if !validJournalShopper(shopper) {
		return fmt.Errorf("%w: %q", ErrBadJournalShopper, shopper)
	}
	return nil
}

// encodeJournal renders one snapshot: the version first, then the
// documents, rows and the partition state, compact on one line.
func encodeJournal(snap Snapshot) ([]byte, error) {
	snap.Version = journalVersion
	raw, err := json.Marshal(snap)
	if err != nil {
		return nil, fmt.Errorf("evidence: encode journal: %w", err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return nil, fmt.Errorf("evidence: encode journal: %w", err)
	}
	return append(compact.Bytes(), '\n'), nil
}

// decodeJournal reads one snapshot and refuses a version it does not
// know.
func decodeJournal(raw []byte) (Snapshot, error) {
	var snap Snapshot
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snap); err != nil {
		return Snapshot{}, fmt.Errorf("%w: %w", ErrCorruptJournal, err)
	}
	if snap.Version != journalVersion {
		return Snapshot{}, fmt.Errorf("%w: version %d", ErrCorruptJournal, snap.Version)
	}
	if snap.Documents == nil {
		snap.Documents = []Document{}
	}
	if snap.Rows == nil {
		snap.Rows = []Row{}
	}
	if snap.Pending == nil {
		snap.Pending = []Pending{}
	}
	if snap.Links == nil {
		snap.Links = []BeliefEvidence{}
	}
	if snap.Timelines == nil {
		snap.Timelines = []BeliefTimeline{}
	}
	if snap.Seq == nil {
		snap.Seq = map[string]int{}
	}
	return snap, nil
}

// Journal keeps one evidence snapshot per shopper in a directory, beside
// the corrections journal: the documents, rows and partition state that
// survive a restart. Writes are atomic: the file is written whole to a
// temporary sibling and renamed over the old one, so a crash leaves the
// last complete snapshot.
type Journal struct {
	mu  sync.Mutex
	dir string
}

// NewJournal builds a journal in a directory. The directory must exist:
// the journal creates files, never directories.
func NewJournal(dir string) (*Journal, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, ErrBadJournalDir
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w: %q", ErrBadJournalDir, dir)
	}
	return &Journal{dir: dir}, nil
}

// path names one shopper's journal file. The caller holds the lock.
func (j *Journal) path(shopper string) string {
	return filepath.Join(j.dir, shopper+".evidence.json")
}

// Save replaces one shopper's evidence snapshot atomically.
func (j *Journal) Save(ctx context.Context, shopper string, snap Snapshot) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("evidence: save journal for %q: %w", shopper, err)
	}
	if err := checkJournalShopper(shopper); err != nil {
		return err
	}
	raw, err := encodeJournal(snap)
	if err != nil {
		return err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	tmp, err := os.CreateTemp(j.dir, shopper+".*.tmp")
	if err != nil {
		return fmt.Errorf("evidence: save journal for %q: %w", shopper, err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("evidence: save journal for %q: %w", shopper, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("evidence: save journal for %q: %w", shopper, err)
	}
	if err := os.Rename(tmpName, j.path(shopper)); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("evidence: save journal for %q: %w", shopper, err)
	}
	return nil
}

// Load returns one shopper's evidence snapshot, or found false when the
// shopper has none yet.
func (j *Journal) Load(ctx context.Context, shopper string) (Snapshot, bool, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, false, fmt.Errorf("evidence: load journal for %q: %w", shopper, err)
	}
	if err := checkJournalShopper(shopper); err != nil {
		return Snapshot{}, false, err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	raw, err := os.ReadFile(j.path(shopper))
	if err != nil {
		if os.IsNotExist(err) {
			return Snapshot{}, false, nil
		}
		return Snapshot{}, false, fmt.Errorf("evidence: load journal for %q: %w", shopper, err)
	}
	snap, err := decodeJournal(raw)
	if err != nil {
		return Snapshot{}, false, fmt.Errorf("evidence: load journal for %q: %w", shopper, err)
	}
	return snap, true, nil
}
