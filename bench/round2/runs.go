package round2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The run-directory protocol. The freeze lands before any arm 3 or arm 4
// turn, so a run directory that appears without a marker is a freeze break,
// and a marker written against a different freeze is stale. T10.4's first
// action is BeginRuns; the freeze check and the gate accept a run directory
// only once that marker names the freeze it was opened against.

// RunsMarkerFile is the marker's file name inside the run directory.
const RunsMarkerFile = "FREEZE.json"

// ErrNoRunsMarker means a run directory exists with no marker, or a marker
// that does not open the current freeze.
var ErrNoRunsMarker = errors.New("round2: run directory has no marker for this freeze")

// commitPattern is a full git commit id: forty lowercase hexadecimal
// characters.
var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// RunsMarker is the file that opens the run directory. It names the commit
// that landed the freeze and pins the freeze manifest by hash, so a marker
// written against one freeze cannot open a later one.
type RunsMarker struct {
	// FreezeCommit is the commit that landed bench/round2/FROZEN.md.
	FreezeCommit string `json:"freeze_commit"`
	// FrozenManifestSHA256 is the hash of bench/round2/FROZEN.md at that
	// commit. The check compares it with the file on disk, which binds the
	// marker to the freeze without depending on git history (a shallow
	// clone has none).
	FrozenManifestSHA256 string `json:"frozen_manifest_sha256"`
	// Note says who opened the runs and for what task.
	Note string `json:"note,omitempty"`
}

// FrozenManifestSHA256 returns the lowercase hex SHA-256 of the freeze
// manifest as it stands in this tree.
func FrozenManifestSHA256() (string, error) {
	raw, err := os.ReadFile(fixturePath("FROZEN.md"))
	if err != nil {
		return "", fmt.Errorf("%w: read freeze manifest: %v", ErrNoRunsMarker, err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// BeginRuns opens the run directory: it creates dir and writes the marker
// against the freeze manifest in this tree. freezeCommit must be the full
// commit id that landed bench/round2/FROZEN.md. Calling it twice rewrites
// the same marker, so a re-run of T10.4's setup is harmless.
func BeginRuns(dir, freezeCommit, note string) (RunsMarker, error) {
	commit := strings.TrimSpace(freezeCommit)
	if !commitPattern.MatchString(commit) {
		return RunsMarker{}, fmt.Errorf("%w: freeze commit %q is not a full commit id", ErrNoRunsMarker, freezeCommit)
	}
	manifest, err := FrozenManifestSHA256()
	if err != nil {
		return RunsMarker{}, err
	}
	marker := RunsMarker{FreezeCommit: commit, FrozenManifestSHA256: manifest, Note: note}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return RunsMarker{}, fmt.Errorf("round2: open run directory: %w", err)
	}
	raw, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return RunsMarker{}, fmt.Errorf("round2: encode runs marker: %w", err)
	}
	path := filepath.Join(dir, RunsMarkerFile)
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return RunsMarker{}, fmt.Errorf("round2: write runs marker: %w", err)
	}
	return marker, nil
}

// ReadRunsMarker reads the run directory's marker and checks it against the
// freeze manifest in this tree. It returns ErrNoRunsMarker when the marker
// is missing, malformed, names no commit, or was written against a
// different freeze.
func ReadRunsMarker(dir string) (RunsMarker, error) {
	path := filepath.Join(dir, RunsMarkerFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return RunsMarker{}, fmt.Errorf("%w: %s is missing; create it with BeginRuns before the first turn", ErrNoRunsMarker, path)
		}
		return RunsMarker{}, fmt.Errorf("%w: read %s: %v", ErrNoRunsMarker, path, err)
	}
	var marker RunsMarker
	if err := json.Unmarshal(raw, &marker); err != nil {
		return RunsMarker{}, fmt.Errorf("%w: decode %s: %v", ErrNoRunsMarker, path, err)
	}
	if !commitPattern.MatchString(marker.FreezeCommit) {
		return RunsMarker{}, fmt.Errorf("%w: %s names freeze commit %q, want a full commit id", ErrNoRunsMarker, path, marker.FreezeCommit)
	}
	manifest, err := FrozenManifestSHA256()
	if err != nil {
		return RunsMarker{}, err
	}
	if marker.FrozenManifestSHA256 != manifest {
		return RunsMarker{}, fmt.Errorf("%w: %s was written against a different freeze (manifest %s, this tree's is %s)",
			ErrNoRunsMarker, path, marker.FrozenManifestSHA256, manifest)
	}
	return marker, nil
}
