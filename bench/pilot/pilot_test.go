package pilot

import (
	"strings"
	"testing"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/seed"
)

func linkWith(customID string) domain.TransactionLink {
	return domain.TransactionLink{
		OrderID:   "ORD-" + customID,
		CaptureID: "CAP-" + customID,
		CustomID:  customID,
		EventID:   "EVT-" + customID,
	}
}

func itemSized(size string) domain.Item {
	return domain.Item{BrandID: "BRP", BrandName: "Brand P", LineName: "Probe Oxford", SKU: "BRP-OXF", Size: size, Category: "dress-shirts"}
}

func TestExtractSizeExplicit(t *testing.T) {
	size, rule := ExtractSize("Recommend size L: the kept size stands.")
	if size != "L" || rule != SizeExplicit {
		t.Errorf("explicit: got %q/%s, want L/explicit-size", size, rule)
	}
}

func TestExtractSizeStandalone(t *testing.T) {
	size, rule := ExtractSize("Take M. The torn-seam refund is damage, not fit.")
	if size != "M" || rule != SizeStandalone {
		t.Errorf("standalone: got %q/%s, want M/standalone-token", size, rule)
	}
}

func TestExtractSizeSkipsNonVocabulary(t *testing.T) {
	size, rule := ExtractSize("What size do you usually take?")
	if size != "" || rule != SizeNone {
		t.Errorf("question: got %q/%s, want empty/none", size, rule)
	}
}

func TestExtractSizeSkipsSizeUnchosenEcho(t *testing.T) {
	// "size unchosen" echoes the prompt but names no vocabulary size, so
	// the explicit rule skips it and the standalone rule finds the L.
	size, rule := ExtractSize("The candidate has its size unchosen; I recommend L for it.")
	if size != "L" || rule != SizeStandalone {
		t.Errorf("echo: got %q/%s, want L/standalone-token", size, rule)
	}
}

func TestExtractSizePunctuation(t *testing.T) {
	size, _ := ExtractSize("My recommendation: (M).")
	if size != "M" {
		t.Errorf("punctuation: got %q, want M", size)
	}
}

func TestExtractIDRefsVerbatimOnly(t *testing.T) {
	history := seed.History{
		ShopperID: "cmp-probe",
		Rows: []seed.Row{
			{Note: "first", Link: linkWith("seed-7-cmp-probe-001")},
			{Note: "second", Link: linkWith("seed-7-cmp-probe-002")},
		},
	}
	refs := ExtractIDRefs(history, "See seed-7-cmp-probe-002 and BRC-OXF-M for details.")
	if len(refs) != 1 || refs[0] != "seed-7-cmp-probe-002" {
		t.Errorf("id refs: got %v, want only the verbatim custom id", refs)
	}
}

func TestExtractIDRefsDeduplicates(t *testing.T) {
	history := seed.History{
		ShopperID: "cmp-probe",
		Rows: []seed.Row{
			{Note: "first", Link: linkWith("seed-7-cmp-probe-001")},
		},
	}
	refs := ExtractIDRefs(history, "seed-7-cmp-probe-001 again seed-7-cmp-probe-001")
	if len(refs) != 1 {
		t.Errorf("dedupe: got %v, want one ref", refs)
	}
}

func TestCheckContamination(t *testing.T) {
	hit, tokens := CheckContamination("Per belief blf_8f2c41, take L.")
	if !hit || len(tokens) == 0 {
		t.Error("demo belief id must flag contamination")
	}
	if hit, _ := CheckContamination("Recommend M: the kept size stands on the evidence given."); hit {
		t.Error("clean arm reply must not flag contamination")
	}
}

func TestEffectiveSizePrefersHuman(t *testing.T) {
	if got := EffectiveSize("L", "M"); got != "M" {
		t.Errorf("human override: got %q, want M", got)
	}
	if got := EffectiveSize("L", ""); got != "L" {
		t.Errorf("no override: got %q, want L", got)
	}
}

func TestGradingGlueScores(t *testing.T) {
	history := seed.History{
		ShopperID: "cmp-probe",
		Rows: []seed.Row{
			{Note: "kept", Item: itemSized("M"), Signal: domain.SignalKeptPastWindow},
		},
	}
	answer := seed.Answer{ShopperID: "cmp-probe", RightSize: "M"}
	score, err := GradeFirst(history, answer, "prompt", "Take M.", "M", nil)
	if err != nil {
		t.Fatalf("grade: %v", err)
	}
	if !strings.Contains(score.TokenBasis, "words") {
		t.Errorf("token basis not recorded: %q", score.TokenBasis)
	}
}
