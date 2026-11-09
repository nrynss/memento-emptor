package evals

import (
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
