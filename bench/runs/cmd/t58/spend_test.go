// Offline test for the cumulative spend guard. No key, no network, no
// spend: it proves a re-run into its own out dir still counts the batches
// committed beside it, so the cumulative cap holds across batches.
package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// writeRunFile writes a minimal graded record carrying one trace cost.
func writeRunFile(t *testing.T, dir, name string, cost float64) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	raw := fmt.Sprintf(`{"run_id":%q,"trace_cost_usd":%v}`, name, cost)
	if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(raw), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestCommittedSpendCountsEarlierBatches(t *testing.T) {
	base := t.TempDir()
	writeRunFile(t, base, "t55-a1-broken-r1", 0.01)
	writeRunFile(t, filepath.Join(base, "t58"), "t58-a3-brands-split-r1", 0.02)
	outDir := filepath.Join(base, "t58b")
	writeRunFile(t, outDir, "t58b-a3-brands-split-r1", 0.04)

	got, err := committedSpend(outDir)
	if err != nil {
		t.Fatalf("committedSpend: %v", err)
	}
	if math.Abs(got-0.07) > 1e-9 {
		t.Errorf("committedSpend = %v, want 0.07 (T5.5 + T5.8 + T5.8b)", got)
	}

	// The T5.8 out dir is also a batch root: its own files are counted
	// once, not twice, when it is the dir a run writes to.
	got, err = committedSpend(filepath.Join(base, "t58"))
	if err != nil {
		t.Fatalf("committedSpend from t58: %v", err)
	}
	if math.Abs(got-0.07) > 1e-9 {
		t.Errorf("committedSpend from t58 = %v, want 0.07", got)
	}
}
