package evals

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wantKeys is the frozen evaluator set: one check per loop-policy rule
// under test. Adding a check is an orchestrator call; the server caps a
// set at ten evaluators.
var wantKeys = []string{"recall_first", "correction_honoured", "approval_before_purchase"}

// decisiveTerms ties each evaluator key to the behaviour its prompt must
// judge. The prompt has to name the tool, the correction, or the approval
// gate explicitly, or the judge grades vibes.
var decisiveTerms = map[string][]string{
	"recall_first":             {"belief", "evidence", "seeded"},
	"correction_honoured":      {"correction"},
	"approval_before_purchase": {"explicit yes", "purchase"},
}

// readSet loads EVALUATION.yaml beside this test. The file is the
// artifact `ast eval push` activates; the test pins its shape without a
// YAML dependency and without contacting the server.
func readSet(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("EVALUATION.yaml"))
	if err != nil {
		t.Fatalf("read EVALUATION.yaml: %v", err)
	}
	return string(raw)
}

// blockOf returns the evaluator block that declares key, from "- key:"
// to the next "- key:" or the end of the document.
func blockOf(t *testing.T, doc, key string) string {
	t.Helper()
	start := strings.Index(doc, "- key: "+key+"\n")
	if start < 0 {
		t.Fatalf("EVALUATION.yaml carries no evaluator %q", key)
	}
	rest := doc[start+len("- key: "+key+"\n"):]
	if next := strings.Index(rest, "- key: "); next >= 0 {
		rest = rest[:next]
	}
	return rest
}

func TestSetSchemaAndKeys(t *testing.T) {
	doc := readSet(t)
	if !strings.HasPrefix(doc, "schema: evaluation/v1\n") {
		t.Fatal("set must open with `schema: evaluation/v1`")
	}
	if got := strings.Count(doc, "- key: "); got != len(wantKeys) {
		t.Fatalf("set carries %d evaluators, want %d", got, len(wantKeys))
	}
	for _, key := range wantKeys {
		if strings.Count(doc, "- key: "+key+"\n") != 1 {
			t.Fatalf("evaluator %q must appear exactly once", key)
		}
	}
}

func TestSetEvaluatorShape(t *testing.T) {
	doc := readSet(t)
	for _, key := range wantKeys {
		block := blockOf(t, doc, key)
		if !strings.Contains(block, "label: ") {
			t.Fatalf("%s: evaluator needs a label", key)
		}
		if !strings.Contains(block, "type: llm") {
			t.Fatalf("%s: evaluator type must be llm; the platform defines no code metric", key)
		}
		if !strings.Contains(block, "prompt:") {
			t.Fatalf("%s: evaluator needs a prompt", key)
		}
		if !strings.Contains(block, "type: boolean") {
			t.Fatalf("%s: evaluator output must be boolean", key)
		}
		promptIdx := strings.Index(block, "prompt:")
		if len(strings.TrimSpace(block[promptIdx:])) < 200 {
			t.Fatalf("%s: prompt is too short to discriminate pass from fail", key)
		}
		for _, term := range decisiveTerms[key] {
			if !strings.Contains(strings.ToLower(block), strings.ToLower(term)) {
				t.Fatalf("%s: prompt must name %q", key, term)
			}
		}
	}
}

func TestSetCarriesNoSecrets(t *testing.T) {
	doc := readSet(t)
	for _, leak := range []string{"sk-", "AKIA", "api_key", "secret", "token", "Bearer"} {
		if strings.Contains(doc, leak) {
			t.Fatalf("EVALUATION.yaml must reference the vault, never a value: found %q", leak)
		}
	}
}

// counterfactualPair is the committed record of one off-platform
// pass/fail judgment: the exact prompt text sent, both transcripts,
// and both verdicts. Re-verify by sending pair.prompt plus each side's
// input/output to any instruction-following judge.
type counterfactualPair struct {
	Evaluator string `json:"evaluator"`
	Prompt    string `json:"prompt"`
	Pass      struct {
		Source  string  `json:"source"`
		Input   string  `json:"input"`
		Output  string  `json:"output"`
		Verdict bool    `json:"verdict"`
		CostUSD float64 `json:"cost_usd"`
	} `json:"pass"`
	Fail struct {
		Source       string  `json:"source"`
		Constructed  bool    `json:"constructed"`
		Construction string  `json:"construction"`
		Input        string  `json:"input"`
		Output       string  `json:"output"`
		Verdict      bool    `json:"verdict"`
		CostUSD      float64 `json:"cost_usd"`
	} `json:"fail"`
}

// foldedPrompt reconstructs the YAML `>-` folded prompt for key from the
// set: continuation lines lose their indent and fold to spaces. The
// counterfactual fixture must carry exactly this text, or the recorded
// verdicts no longer belong to the frozen prompt.
func foldedPrompt(t *testing.T, doc, key string) string {
	t.Helper()
	block := blockOf(t, doc, key)
	idx := strings.Index(block, "prompt: >-")
	if idx < 0 {
		t.Fatalf("%s: no folded prompt block", key)
	}
	var lines []string
	for _, line := range strings.Split(block[idx:], "\n")[1:] {
		if !strings.HasPrefix(line, "      ") || strings.TrimSpace(line) == "" {
			break
		}
		lines = append(lines, strings.TrimSpace(line))
	}
	if len(lines) == 0 {
		t.Fatalf("%s: folded prompt is empty", key)
	}
	return strings.Join(lines, " ")
}

func TestCounterfactualPairs(t *testing.T) {
	doc := readSet(t)
	for _, file := range []string{"correction-counterfactual.json", "approval-counterfactual.json"} {
		raw, err := os.ReadFile(filepath.Join("testdata", file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		var pair counterfactualPair
		if err := json.Unmarshal(raw, &pair); err != nil {
			t.Fatalf("decode %s: %v", file, err)
		}
		var wantKey string
		for _, key := range wantKeys {
			if pair.Evaluator == key {
				wantKey = key
			}
		}
		if wantKey == "" {
			t.Fatalf("%s: evaluator %q is not in the frozen set", file, pair.Evaluator)
		}
		if pair.Prompt != foldedPrompt(t, doc, wantKey) {
			t.Fatalf("%s: fixture prompt drifted from EVALUATION.yaml; re-record the verdicts", file)
		}
		if !pair.Pass.Verdict {
			t.Fatalf("%s: pass side must record true", file)
		}
		if pair.Pass.Input == "" || pair.Pass.Output == "" || pair.Pass.Source == "" {
			t.Fatalf("%s: pass side needs its source trace ref and both texts", file)
		}
		if pair.Fail.Verdict {
			t.Fatalf("%s: fail side must record false", file)
		}
		if !pair.Fail.Constructed || pair.Fail.Construction == "" {
			t.Fatalf("%s: fail side must say how it was constructed", file)
		}
		if pair.Fail.Input == "" || pair.Fail.Output == "" {
			t.Fatalf("%s: fail side needs both texts", file)
		}
		for _, leak := range []string{"sk-", "AKIA", "api_key", "secret", "token", "Bearer", "user_01"} {
			if strings.Contains(string(raw), leak) {
				t.Fatalf("%s: fixtures carry transcripts, never values or ids: found %q", file, leak)
			}
		}
	}
}
