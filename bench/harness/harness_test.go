package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nrynss/memento-emptor/seed"
)

// fixtureHistoriesDir is the committed arm-facing histories. The test
// reads them the way the harness does: the histories directory only.
const fixtureHistoriesDir = "../../testdata/seed/histories"

// hiddenAnswersPath is read by this test alone to prove the prompts
// contain no answer material. Non-test harness code must never reference
// it; TestHarnessNeverLoadsHidden enforces that statically.
const hiddenAnswersPath = "../../testdata/seed/hidden/answers.json"

func loadFixtureHistories(t *testing.T) []seed.History {
	t.Helper()
	histories, err := LoadHistories(fixtureHistoriesDir)
	if err != nil {
		t.Fatalf("load fixture histories: %v", err)
	}
	if len(histories) == 0 {
		t.Fatal("no fixture histories to run")
	}
	return histories
}

// scriptedReplies records one fixture reply per shopper, standing in for
// the model so the dry run needs no key and no network.
func scriptedReplies(histories []seed.History) map[string]string {
	replies := make(map[string]string, len(histories))
	for _, h := range histories {
		replies[h.ShopperID] = "Recommend M for " + h.ShopperID + ": the kept size stands on the evidence given."
	}
	return replies
}

func TestDryRunProducesACompleteRecordForEachArm(t *testing.T) {
	histories := loadFixtureHistories(t)
	model := ScriptedByShopper(scriptedReplies(histories))
	caps := DefaultCaps()
	for _, arm := range []Arm{ArmNoMemory, ArmRawHistory, ArmMemento} {
		records, err := RunAll(context.Background(), histories, arm, model, caps)
		if err != nil {
			t.Fatalf("arm %s: run all: %v", arm, err)
		}
		if len(records) != len(histories) {
			t.Fatalf("arm %s: records = %d, want %d", arm, len(records), len(histories))
		}
		for _, record := range records {
			if record.Prompt == "" {
				t.Errorf("arm %s shopper %s: empty prompt", arm, record.ShopperID)
			}
			if record.Reply == "" {
				t.Errorf("arm %s shopper %s: empty reply", arm, record.ShopperID)
			}
			if record.PromptTokens <= 0 || record.ReplyTokens <= 0 {
				t.Errorf("arm %s shopper %s: token counts not recorded: prompt %d reply %d",
					arm, record.ShopperID, record.PromptTokens, record.ReplyTokens)
			}
			if record.TotalTokens != record.PromptTokens+record.ReplyTokens {
				t.Errorf("arm %s shopper %s: total %d != prompt %d + reply %d",
					arm, record.ShopperID, record.TotalTokens, record.PromptTokens, record.ReplyTokens)
			}
			wantSpend := caps.SpendFor(record.TotalTokens)
			if record.SpendUSD != wantSpend {
				t.Errorf("arm %s shopper %s: spend %f, want %f", arm, record.ShopperID, record.SpendUSD, wantSpend)
			}
			if record.SpendUSD > caps.MaxSpendUSD || record.TotalTokens > caps.MaxTokens {
				t.Errorf("arm %s shopper %s: dry run exceeded its caps", arm, record.ShopperID)
			}
			if record.Capped {
				t.Errorf("arm %s shopper %s: dry run unexpectedly capped", arm, record.ShopperID)
			}
			if !record.Complete {
				t.Errorf("arm %s shopper %s: dry run not complete", arm, record.ShopperID)
			}
			switch arm {
			case ArmNoMemory:
				if len(record.ToolCalls) != 0 {
					t.Errorf("arm %s shopper %s: %d tool calls, want none", arm, record.ShopperID, len(record.ToolCalls))
				}
			case ArmRawHistory:
				if len(record.ToolCalls) != 1 || record.ToolCalls[0].Name != "read_history" {
					t.Errorf("arm %s shopper %s: want one read_history call, got %+v", arm, record.ShopperID, record.ToolCalls)
				}
			case ArmMemento:
				if len(record.ToolCalls) != 1 || record.ToolCalls[0].Name != "recall_for_purchase" {
					t.Errorf("arm %s shopper %s: want one recall_for_purchase call, got %+v", arm, record.ShopperID, record.ToolCalls)
				}
			}
			for _, call := range record.ToolCalls {
				if call.Request == "" || call.Response == "" {
					t.Errorf("arm %s shopper %s: tool call %q missing request or response", arm, record.ShopperID, call.Name)
				}
			}
		}
	}
}

func TestArmPromptsDifferOnlyInMemory(t *testing.T) {
	histories := loadFixtureHistories(t)
	for _, history := range histories {
		bases := map[Arm]string{}
		memories := map[Arm]string{}
		fulls := map[Arm]string{}
		for _, arm := range []Arm{ArmNoMemory, ArmRawHistory, ArmMemento} {
			bases[arm] = BaseSection(history)
			memory, _ := MemorySection(arm, history)
			memories[arm] = memory
			prompt, _ := PromptFor(history, arm)
			fulls[arm] = prompt
			if prompt != bases[arm]+"\n"+memory {
				t.Errorf("shopper %s arm %s: prompt is not base plus memory", history.ShopperID, arm)
			}
		}
		if bases[ArmNoMemory] != bases[ArmRawHistory] || bases[ArmRawHistory] != bases[ArmMemento] {
			t.Errorf("shopper %s: base prompt differs between arms", history.ShopperID)
		}
		if memories[ArmNoMemory] == memories[ArmRawHistory] ||
			memories[ArmRawHistory] == memories[ArmMemento] ||
			memories[ArmNoMemory] == memories[ArmMemento] {
			t.Errorf("shopper %s: two arms carry identical memory", history.ShopperID)
		}
		if fulls[ArmNoMemory] == fulls[ArmRawHistory] || fulls[ArmRawHistory] == fulls[ArmMemento] {
			t.Errorf("shopper %s: two arm prompts are identical", history.ShopperID)
		}
	}
}

func TestHarnessNeverLoadsHidden(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list harness sources: %v", err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		for _, banned := range []string{"hidden", "answers.json", "RightSize", "right_size"} {
			if strings.Contains(string(raw), banned) {
				t.Errorf("%s references %q: harness code must never touch the hidden answers", file, banned)
			}
		}
	}
	raw, err := os.ReadFile(hiddenAnswersPath)
	if err != nil {
		t.Fatalf("read hidden answers: %v", err)
	}
	var answers []seed.Answer
	if err := json.Unmarshal(raw, &answers); err != nil {
		t.Fatalf("decode hidden answers: %v", err)
	}
	if len(answers) == 0 {
		t.Fatal("no hidden answers to check against")
	}
	histories := loadFixtureHistories(t)
	for _, history := range histories {
		for _, arm := range []Arm{ArmNoMemory, ArmRawHistory, ArmMemento} {
			prompt, _ := PromptFor(history, arm)
			if strings.Contains(prompt, "RightSize") || strings.Contains(prompt, "right_size") {
				t.Errorf("shopper %s arm %s: prompt carries an answer key", history.ShopperID, arm)
			}
			for _, a := range answers {
				if strings.Contains(prompt, a.Why) {
					t.Errorf("shopper %s arm %s: prompt contains the hidden rationale for %s",
						history.ShopperID, arm, a.ShopperID)
				}
			}
		}
	}
}

func TestCapsStopTheRunBeforeAnyModelCall(t *testing.T) {
	histories := loadFixtureHistories(t)
	history := histories[0]
	calls := 0
	model := func(ctx context.Context, prompt string) (string, error) {
		calls++
		return "this reply must never be generated", nil
	}
	tiny := Caps{MaxTokens: 5, MaxSpendUSD: 0.000001, PricePerTokenUSD: 0.000005}
	for _, arm := range []Arm{ArmNoMemory, ArmRawHistory, ArmMemento} {
		record, err := RunOne(context.Background(), history, arm, ModelFunc(model), tiny)
		if err != nil {
			t.Fatalf("arm %s: capped run: %v", arm, err)
		}
		if !record.Capped {
			t.Errorf("arm %s: run over cap is not marked capped", arm)
		}
		if record.Complete {
			t.Errorf("arm %s: run over cap reports complete", arm)
		}
		if record.Reply != "" {
			t.Errorf("arm %s: run over cap carries a reply", arm)
		}
	}
	if calls != 0 {
		t.Errorf("capped runs issued %d model calls, want none", calls)
	}
}

func TestReplyOverrunIsMarkedCapped(t *testing.T) {
	histories := loadFixtureHistories(t)
	history := histories[0]
	base, _ := PromptFor(history, ArmNoMemory)
	promptTokens := CountTokens(base)
	caps := DefaultCaps()
	caps.MaxTokens = promptTokens
	model := ScriptedByShopper(map[string]string{history.ShopperID: "a recommendation with several extra words"})
	record, err := RunOne(context.Background(), history, ArmNoMemory, model, caps)
	if err != nil {
		t.Fatalf("overrun run: %v", err)
	}
	if !record.Capped {
		t.Error("reply pushing the total over the token cap is not marked capped")
	}
	if record.Complete {
		t.Error("capped run reports complete")
	}
	if record.Reply == "" {
		t.Error("overrun record dropped the reply the fixture generated")
	}
}
