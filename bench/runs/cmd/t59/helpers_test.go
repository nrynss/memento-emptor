// Offline test for the T5.9 correction mapping and prompt construction.
// No key, no network, no spend: it proves the uniform correction rule
// fires the documented way for every cell, the committed first turns
// load and agree with the pinned effective sizes, the seed builds for
// all nine shoppers, and the second-turn prompts reuse the first turns
// byte for byte.
package main

import (
	"strings"
	"testing"

	"github.com/nrynss/memento-emptor/bench/harness"
	"github.com/nrynss/memento-emptor/bench/pilot"
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

func TestCorrectionRule(t *testing.T) {
	cases := map[string][2]string{
		// Explicit L flips to M; everything else takes L.
		"L": {"Actually I take M now", "M"},
		"l": {"Actually I take M now", "M"},
		// M, S and refusals all take the default L.
		"M": {"Actually I take L now", "L"},
		"S": {"Actually I take L now", "L"},
		"":  {"Actually I take L now", "L"},
	}
	for first, want := range cases {
		text, wantSize := correctionFor(first)
		if text != want[0] || wantSize != want[1] {
			t.Errorf("correctionFor(%q) = (%q, %q), want (%q, %q)",
				first, text, wantSize, want[0], want[1])
		}
	}
}

// pinnedFirstEffective is the effective first size of the reused r1 run
// in each cell, measured from the committed files before any T5.9 turn.
var pinnedFirstEffective = map[string]string{
	"1:cmp-broken": "", "1:cmp-changed-mind": "", "1:cmp-never-arrived": "M",
	"1:cmp-size-changed": "", "1:cmp-unclear": "",
	"1:cmp-noisy-small": "", "1:cmp-noisy-large": "",
	"1:cmp-brands-steady": "", "1:cmp-brands-split": "",
	"2:cmp-broken": "M", "2:cmp-changed-mind": "M", "2:cmp-never-arrived": "L",
	"2:cmp-size-changed": "L", "2:cmp-unclear": "M",
	"2:cmp-noisy-small": "L", "2:cmp-noisy-large": "S",
	"2:cmp-brands-steady": "M", "2:cmp-brands-split": "S",
	"3:cmp-broken": "", "3:cmp-changed-mind": "", "3:cmp-never-arrived": "",
	"3:cmp-size-changed": "", "3:cmp-unclear": "M",
	"3:cmp-noisy-small": "L", "3:cmp-noisy-large": "S",
	"3:cmp-brands-steady": "", "3:cmp-brands-split": "",
}

// pinnedWant is the correction want size per cell under the uniform rule:
// only effective-L cells take M.
var pinnedWant = map[string]string{
	"1:cmp-broken": "L", "1:cmp-changed-mind": "L", "1:cmp-never-arrived": "L",
	"1:cmp-size-changed": "L", "1:cmp-unclear": "L",
	"1:cmp-noisy-small": "L", "1:cmp-noisy-large": "L",
	"1:cmp-brands-steady": "L", "1:cmp-brands-split": "L",
	"2:cmp-broken": "L", "2:cmp-changed-mind": "L", "2:cmp-never-arrived": "M",
	"2:cmp-size-changed": "M", "2:cmp-unclear": "L",
	"2:cmp-noisy-small": "M", "2:cmp-noisy-large": "L",
	"2:cmp-brands-steady": "L", "2:cmp-brands-split": "L",
	"3:cmp-broken": "L", "3:cmp-changed-mind": "L", "3:cmp-never-arrived": "L",
	"3:cmp-size-changed": "L", "3:cmp-unclear": "L",
	"3:cmp-noisy-small": "M", "3:cmp-noisy-large": "L",
	"3:cmp-brands-steady": "L", "3:cmp-brands-split": "L",
}

func TestSourceRunsLoadAndMap(t *testing.T) {
	for _, shopper := range allShoppers {
		for _, arm := range []int{1, 2, 3} {
			key := strings.Join([]string{string(rune('0' + arm)), shopper}, ":")
			source, err := loadSourceRun("../../../../bench/runs", shopper, arm)
			if err != nil {
				t.Fatalf("cell %s: %v", key, err)
			}
			if source.Prompt == "" || source.Reply == "" {
				t.Fatalf("cell %s: source run carries empty prompt or reply", key)
			}
			effective := pilot.EffectiveSize(source.Extraction.Size, source.Extraction.HumanSize)
			if effective != pinnedFirstEffective[key] {
				t.Errorf("cell %s: effective first size %q, want pinned %q (reused run changed?)",
					key, effective, pinnedFirstEffective[key])
			}
			_, want := correctionFor(effective)
			if want != pinnedWant[key] {
				t.Errorf("cell %s: want %q, want pinned %q", key, want, pinnedWant[key])
			}
		}
	}
}

func TestSeedBuildsForAllNine(t *testing.T) {
	for _, shopper := range allShoppers {
		history := mustHistory(t, shopper)
		s, err := buildSeed(history)
		if err != nil {
			t.Fatalf("seed %s: %v", shopper, err)
		}
		if _, _, err := brandForRecall(history); err != nil {
			t.Errorf("seed %s: %v", shopper, err)
		}
		_ = s
	}
}

func TestSecondPromptConstruction(t *testing.T) {
	firstPrompt := "Shopper: cmp-broken\nMemory: none."
	firstReply := "I recommend M."
	_, want := correctionFor("M")
	plain := secondPromptPlain(firstPrompt, firstReply, "Actually I take L now")
	if !strings.HasPrefix(plain, firstPrompt) {
		t.Error("arms-1-2 prompt does not reuse the first prompt verbatim")
	}
	for _, needle := range []string{firstReply, "Actually I take L now", reAsk} {
		if !strings.Contains(plain, needle) {
			t.Errorf("arms-1-2 prompt misses %q", needle)
		}
	}
	if want != "L" {
		t.Errorf("want for M first = %q, want L", want)
	}
	history := mustHistory(t, "cmp-noisy-small")
	var post arm3Memory
	arm3prompt := secondPromptArm3(history, post, firstReply, "Actually I take M now")
	for _, needle := range []string{"Memory: Memento layer", firstReply, "Actually I take M now", reAsk} {
		if !strings.Contains(arm3prompt, needle) {
			t.Errorf("arm-3 prompt misses %q", needle)
		}
	}
	if !strings.HasPrefix(arm3prompt, harness.BaseSection(history)) {
		t.Error("arm-3 prompt does not open with the frozen base section")
	}
}
