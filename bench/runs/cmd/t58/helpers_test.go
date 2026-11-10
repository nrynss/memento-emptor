// Offline test for the T5.8 history-to-seed mapping. No key, no network,
// no spend: it proves each hardened comparison history files exactly the
// way the live path would, before any live Lambo session is seeded.
package main

import (
	"testing"

	"github.com/nrynss/memento-emptor/bench/harness"
	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/api/server"
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

// fitBelief returns the fit belief for one brand, failing when it is not
// exactly one.
func fitBelief(t *testing.T, shopper string, s server.Seed, brandID string) contract.BeliefView {
	t.Helper()
	var found []contract.BeliefView
	for _, b := range s.Beliefs {
		if b.Kind == domain.BeliefKindFit && b.Subject.ID == brandID {
			found = append(found, b)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s: %d fit beliefs for brand %s, want 1", shopper, len(found), brandID)
	}
	return found[0]
}

func TestHardenedSeedShapes(t *testing.T) {
	noisySmall := mustHistory(t, "cmp-noisy-small")
	smallSeed, err := buildSeed(noisySmall)
	if err != nil {
		t.Fatalf("seed cmp-noisy-small: %v", err)
	}
	// Three too-small returns birth one Brand H fit belief; the
	// changed-mind, vague (open), damage, non-arrival and unauthorized
	// rows never join it. Damage and non-arrival file one Brand H
	// seller belief; the Brand J fits-well capture files its own fit
	// belief; only the below-floor unclear row stays an open question.
	if len(smallSeed.Beliefs) != 3 {
		t.Fatalf("cmp-noisy-small: %d beliefs, want 3 (BRH fit, BRH seller, BRJ feedback)", len(smallSeed.Beliefs))
	}
	smallFit := fitBelief(t, "cmp-noisy-small", smallSeed, "BRH")
	if got := smallFit.Value; got != "runs small, take size L" {
		t.Errorf("cmp-noisy-small fit value %q, want runs small, take size L", got)
	}
	if len(smallFit.Evidence) != 3 {
		t.Errorf("cmp-noisy-small fit evidence rows: %d, want 3", len(smallFit.Evidence))
	}
	if len(smallSeed.Pending.Questions) != 1 {
		t.Fatalf("cmp-noisy-small pending: %d, want the 1 vague question", len(smallSeed.Pending.Questions))
	}
	if got := smallSeed.Pending.Questions[0].QuestionID; got != "q_000001" {
		t.Errorf("question id %q, want q_000001 (live mint order)", got)
	}
	if brand, _, err := brandForRecall(noisySmall); err != nil || brand != "BRH" {
		t.Errorf("cmp-noisy-small recall brand %q, %v; want BRH", brand, err)
	}

	noisyLarge := mustHistory(t, "cmp-noisy-large")
	largeSeed, err := buildSeed(noisyLarge)
	if err != nil {
		t.Fatalf("seed cmp-noisy-large: %v", err)
	}
	if len(largeSeed.Beliefs) != 3 {
		t.Fatalf("cmp-noisy-large: %d beliefs, want 3 (BRI fit, BRI seller, BRL feedback)", len(largeSeed.Beliefs))
	}
	largeFit := fitBelief(t, "cmp-noisy-large", largeSeed, "BRI")
	if got := largeFit.Value; got != "runs large, take size S" {
		t.Errorf("cmp-noisy-large fit value %q, want runs large, take size S", got)
	}
	if len(largeFit.Evidence) != 3 {
		t.Errorf("cmp-noisy-large fit evidence rows: %d, want 3", len(largeFit.Evidence))
	}
	if len(largeSeed.Pending.Questions) != 1 {
		t.Fatalf("cmp-noisy-large pending: %d, want the 1 vague question", len(largeSeed.Pending.Questions))
	}
	if brand, _, err := brandForRecall(noisyLarge); err != nil || brand != "BRI" {
		t.Errorf("cmp-noisy-large recall brand %q, %v; want BRI", brand, err)
	}

	steady := mustHistory(t, "cmp-brands-steady")
	steadySeed, err := buildSeed(steady)
	if err != nil {
		t.Fatalf("seed cmp-brands-steady: %v", err)
	}
	// Two Brand P too-small returns birth one Brand P fit belief; the
	// Brand Q fits-well capture files its own; the Brand Q non-arrival
	// files a seller belief that must never move the Brand Q sizing
	// recommendation. Nothing stays pending.
	if len(steadySeed.Beliefs) != 3 {
		t.Fatalf("cmp-brands-steady: %d beliefs, want 3 (BRP fit, BRQ feedback, BRQ seller)", len(steadySeed.Beliefs))
	}
	steadyFit := fitBelief(t, "cmp-brands-steady", steadySeed, "BRP")
	if len(steadyFit.Evidence) != 2 {
		t.Errorf("cmp-brands-steady BRP fit evidence rows: %d, want 2", len(steadyFit.Evidence))
	}
	steadyQ := fitBelief(t, "cmp-brands-steady", steadySeed, "BRQ")
	if len(steadyQ.Evidence) != 1 {
		t.Errorf("cmp-brands-steady BRQ feedback evidence rows: %d, want 1", len(steadyQ.Evidence))
	}
	if len(steadySeed.Pending.Questions) != 0 {
		t.Errorf("cmp-brands-steady pending: %d, want 0", len(steadySeed.Pending.Questions))
	}
	if brand, _, err := brandForRecall(steady); err != nil || brand != "BRQ" {
		t.Errorf("cmp-brands-steady recall brand %q, %v; want BRQ", brand, err)
	}

	split := mustHistory(t, "cmp-brands-split")
	splitSeed, err := buildSeed(split)
	if err != nil {
		t.Fatalf("seed cmp-brands-split: %v", err)
	}
	// Two Brand R too-large returns birth one Brand R fit belief; the
	// single Brand T too-small return files its own and must never move
	// the Brand R recommendation. Nothing stays pending.
	if len(splitSeed.Beliefs) != 3 {
		t.Fatalf("cmp-brands-split: %d beliefs, want 3 (BRR fit, BRT fit, BRR seller)", len(splitSeed.Beliefs))
	}
	splitFit := fitBelief(t, "cmp-brands-split", splitSeed, "BRR")
	if got := splitFit.Value; got != "runs large, take size S" {
		t.Errorf("cmp-brands-split BRR fit value %q, want runs large, take size S", got)
	}
	if len(splitFit.Evidence) != 2 {
		t.Errorf("cmp-brands-split BRR fit evidence rows: %d, want 2", len(splitFit.Evidence))
	}
	splitT := fitBelief(t, "cmp-brands-split", splitSeed, "BRT")
	if len(splitT.Evidence) != 1 {
		t.Errorf("cmp-brands-split BRT fit evidence rows: %d, want 1", len(splitT.Evidence))
	}
	if len(splitSeed.Pending.Questions) != 0 {
		t.Errorf("cmp-brands-split pending: %d, want 0", len(splitSeed.Pending.Questions))
	}
	if brand, _, err := brandForRecall(split); err != nil || brand != "BRR" {
		t.Errorf("cmp-brands-split recall brand %q, %v; want BRR", brand, err)
	}
}
