// Offline test for the T5.5 history-to-seed mapping. No key, no network,
// no spend: it proves each frozen comparison history files exactly the
// way the live path would, before any live Lambo session is seeded.
package main

import (
	"testing"

	"github.com/nrynss/memento-emptor/bench/harness"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/seed"
)

func mustHistory(t *testing.T, id string) seed.History {
	t.Helper()
	histories, err := harness.LoadHistories("../../../../testdata/seed/histories")
	if err != nil {
		t.Fatalf("load histories: %v", err)
	}
	for _, h := range histories {
		if h.ShopperID == id {
			return h
		}
	}
	t.Fatalf("unknown shopper %q", id)
	return seed.History{}
}

// TestSeedShapes pins the live-path filing per comparison shopper: which
// beliefs exist, what they count toward, and which questions stay open.
// The live path would file the same rows the same way (attachRow,
// beliefForReason, beliefForFeedback, RecordReason close rules); any
// divergence here means the driver misfiles, not that the layer moved.
func TestSeedShapes(t *testing.T) {
	broken := mustHistory(t, "cmp-broken")
	s, err := buildSeed(broken)
	if err != nil {
		t.Fatalf("seed cmp-broken: %v", err)
	}
	// A damaged refund counts toward seller (the domain rule), so the
	// live path files one Claimed seller belief. Sizing recall never
	// serves it.
	if len(s.Beliefs) != 1 || s.Beliefs[0].Kind != domain.BeliefKindSeller {
		t.Fatalf("cmp-broken: %+v, want one seller belief", s.Beliefs)
	}
	if len(s.Pending.Questions) != 0 {
		t.Errorf("cmp-broken: %d pending, want 0 (classified answers close)", len(s.Pending.Questions))
	}

	changed := mustHistory(t, "cmp-changed-mind")
	changedSeed, err := buildSeed(changed)
	if err != nil {
		t.Fatalf("seed cmp-changed-mind: %v", err)
	}
	if len(changedSeed.Beliefs) != 0 {
		t.Errorf("cmp-changed-mind: %d beliefs, want 0 (changed mind counts toward nothing)", len(changedSeed.Beliefs))
	}
	if len(changedSeed.Pending.Questions) != 0 {
		t.Errorf("cmp-changed-mind: %d pending, want 0 (classified answers close)", len(changedSeed.Pending.Questions))
	}

	never := mustHistory(t, "cmp-never-arrived")
	neverSeed, err := buildSeed(never)
	if err != nil {
		t.Fatalf("seed cmp-never-arrived: %v", err)
	}
	if len(neverSeed.Beliefs) != 1 || neverSeed.Beliefs[0].Kind != domain.BeliefKindSeller {
		t.Fatalf("cmp-never-arrived: %+v, want one seller belief", neverSeed.Beliefs)
	}
	if got := neverSeed.Beliefs[0].Value; got != "ships reliably for this shopper" {
		t.Errorf("seller value %q", got)
	}
	if len(neverSeed.Beliefs[0].Evidence) != 1 {
		t.Fatalf("seller evidence rows: %d, want 1", len(neverSeed.Beliefs[0].Evidence))
	}

	sized := mustHistory(t, "cmp-size-changed")
	sizedSeed, err := buildSeed(sized)
	if err != nil {
		t.Fatalf("seed cmp-size-changed: %v", err)
	}
	if len(sizedSeed.Beliefs) != 1 || sizedSeed.Beliefs[0].Kind != domain.BeliefKindFit {
		t.Fatalf("cmp-size-changed: %+v, want one feedback fit belief", sizedSeed.Beliefs)
	}
	// The two too-small returns oppose the fits-well direction and land
	// unattached below the rival-birth threshold: only the capture is
	// evidence.
	if len(sizedSeed.Beliefs[0].Evidence) != 1 || sizedSeed.Beliefs[0].Evidence[0].Outcome.Kind != domain.OutcomeCapture {
		t.Errorf("size-changed evidence: %+v, want only the day-1 capture", sizedSeed.Beliefs[0].Evidence)
	}
	if len(sizedSeed.Pending.Questions) != 0 {
		t.Errorf("size-changed pending: %d, want 0", len(sizedSeed.Pending.Questions))
	}

	unclear := mustHistory(t, "cmp-unclear")
	unclearSeed, err := buildSeed(unclear)
	if err != nil {
		t.Fatalf("seed cmp-unclear: %v", err)
	}
	if len(unclearSeed.Beliefs) != 0 {
		t.Errorf("cmp-unclear: %d beliefs, want 0", len(unclearSeed.Beliefs))
	}
	if len(unclearSeed.Pending.Questions) != 1 {
		t.Fatalf("cmp-unclear pending: %d, want the 1 vague question", len(unclearSeed.Pending.Questions))
	}
	if got := unclearSeed.Pending.Questions[0].QuestionID; got != "q_000001" {
		t.Errorf("question id %q, want q_000001 (live mint order)", got)
	}
}
