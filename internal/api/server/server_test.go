package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// testContract is a test-only embedder contract with no model constants.
type testContract struct {
	kind string
}

// EmbeddingKind names the contract family.
func (c testContract) EmbeddingKind() string { return c.kind }

// Validate reports whether the contract is usable.
func (c testContract) Validate() error {
	if c.kind == "" {
		return errors.New("server test: contract names no kind")
	}
	return nil
}

// apiDir returns the frozen fixtures directory.
func apiDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "testdata", "api")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("testdata/api is missing at %s", dir)
	}
	return dir
}

// jevDir returns the classifier recordings directory.
func jevDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "testdata", "jev")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("testdata/jev is missing at %s", dir)
	}
	return dir
}

// seedAllowlist is gone: the test builds the graph fake over the same
// exported closed world the demo binary uses.
func TestSeedGraphContentsCoversSeed(t *testing.T) {
	seed, err := SeedFromExamples(apiDir(t))
	if err != nil {
		t.Fatalf("build seed: %v", err)
	}
	allowed := map[string]bool{}
	for _, content := range SeedGraphContents(seed) {
		allowed[content] = true
	}
	for _, belief := range seed.Beliefs {
		content := beliefContent(belief.Kind, belief.Subject, belief.Value)
		if !allowed[content] {
			t.Errorf("seed graph contents miss belief %q", content)
		}
	}
}

// toMap decodes one JSON document into a generic map for value comparison.
func toMap(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("decode JSON object: %v", err)
	}
	return value
}

// pretty renders one compared value for failure output.
func pretty(value any) string {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(raw)
}

// seededService builds a service replayed to the frozen story's end state
// on the fakes, with the clock pinned so stamped times are exact.
func seededService(t *testing.T) *Service {
	t.Helper()
	dir := apiDir(t)
	seed, err := SeedFromExamples(dir)
	if err != nil {
		t.Fatalf("build seed: %v", err)
	}
	store, err := evidence.NewStore(testContract{kind: "server-test"})
	if err != nil {
		t.Fatalf("open evidence store: %v", err)
	}
	classifier, err := fakes.NewRecordingReasonClassifier(jevDir(t))
	if err != nil {
		t.Fatalf("open classifier fake: %v", err)
	}
	fixed := time.Date(2026, 11, 8, 18, 40, 0, 0, time.UTC)
	svc, err := NewService(Deps{
		Payments:   fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()),
		Classifier: classifier,
		Store:      store,
		Graph:      fakes.NewRecordingEvidenceGraph(SeedGraphContents(seed), fakes.StoryRecalls()),
		SchemaDir:  dir,
		Clock:      func() time.Time { return fixed },
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	if err := svc.LoadSeed(context.Background(), seed); err != nil {
		t.Fatalf("load seed: %v", err)
	}
	return svc
}

// responseFor maps a tool to its response wire type for the strict shape
// check: unknown fields fail the comparison.
func responseFor(tool string) any {
	switch contract.ToolName(tool) {
	case contract.ToolRecallForPurchase:
		return &contract.RecallResponse{}
	case contract.ToolRecordCorrection:
		return &contract.RecordCorrectionResponse{}
	case contract.ToolPendingQuestions:
		return &contract.PendingQuestionsResponse{}
	case contract.ToolRecordReason:
		return &contract.RecordReasonResponse{}
	case contract.ToolRecordFeedback:
		return &contract.RecordFeedbackResponse{}
	case contract.ToolExplainBelief:
		return &contract.ExplainBeliefResponse{}
	case contract.ToolLedgerBeliefs:
		return &contract.LedgerBeliefsResponse{}
	case contract.ToolLedgerBelief:
		return &contract.LedgerBeliefResponse{}
	case contract.ToolLedgerEvidence:
		return &contract.LedgerEvidenceResponse{}
	default:
		return nil
	}
}

// compareExample calls one tool with the example request and compares the
// response to the example response. Fields named in redact are
// service-minted on write and asserted non-empty instead of byte-exact;
// everything else must match exactly.
func compareExample(t *testing.T, svc *Service, tool, reqFile, respFile string, redact ...string) {
	t.Helper()
	ctx := context.Background()
	raw, err := os.ReadFile(filepath.Join(apiDir(t), reqFile))
	if err != nil {
		t.Fatalf("read %s: %v", reqFile, err)
	}
	out, err := svc.dispatch(ctx, tool, raw)
	if err != nil {
		t.Fatalf("%s with %s: %v", tool, reqFile, err)
	}
	actual := responseFor(tool)
	decoder := json.NewDecoder(bytes.NewReader(out))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(actual); err != nil {
		t.Fatalf("%s response does not match its Go type: %v", tool, err)
	}
	wantRaw, err := os.ReadFile(filepath.Join(apiDir(t), respFile))
	if err != nil {
		t.Fatalf("read %s: %v", respFile, err)
	}
	want := responseFor(tool)
	decoder = json.NewDecoder(bytes.NewReader(wantRaw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(want); err != nil {
		t.Fatalf("%s fixture does not match its Go type: %v", tool, err)
	}
	actualMap := toMap(t, out)
	wantMap := toMap(t, wantRaw)
	for _, field := range redact {
		actualValue, ok := actualMap[field]
		if !ok || actualValue == "" {
			t.Fatalf("%s: redacted field %q is missing or empty in the response", tool, field)
		}
		delete(actualMap, field)
		delete(wantMap, field)
	}
	if !reflect.DeepEqual(actualMap, wantMap) {
		t.Fatalf("%s with %s differs from %s:\nactual:   %s\nwant: %s", tool, reqFile, respFile, pretty(actualMap), pretty(wantMap))
	}
}

// TestContractExamplesAgainstService drives every frozen example through
// the service on the fakes. Reads and the idempotent correction match
// byte-exact; the two writes that mint ids match modulo the minted id,
// which is asserted present. The accepted record_reason replays the
// recorded clear answer behind the fixture's shape: the fixture's answer
// text has no P0 recording and the fake never invents one.
func TestContractExamplesAgainstService(t *testing.T) {
	t.Run("recall", func(t *testing.T) {
		compareExample(t, seededService(t), "recall_for_purchase",
			"recall_for_purchase.request.example.json", "recall_for_purchase.response.example.json")
	})
	t.Run("record_correction", func(t *testing.T) {
		compareExample(t, seededService(t), "record_correction",
			"record_correction.request.example.json", "record_correction.response.example.json")
	})
	t.Run("pending_questions", func(t *testing.T) {
		compareExample(t, seededService(t), "pending_questions",
			"pending_questions.request.example.json", "pending_questions.response.example.json")
	})
	t.Run("record_reason_accepted", func(t *testing.T) {
		svc := seededService(t)
		req := map[string]any{
			"shopper_id":  "shopper_01",
			"question_id": "q_9f04ab",
			"answer_text": "too tight in the shoulders",
			"trace_id":    "trace_3f8a03",
		}
		raw, _ := json.Marshal(req)
		out, err := svc.dispatch(context.Background(), "record_reason", raw)
		if err != nil {
			t.Fatalf("record_reason: %v", err)
		}
		actual := responseFor("record_reason")
		decoder := json.NewDecoder(bytes.NewReader(out))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(actual); err != nil {
			t.Fatalf("record_reason response does not match its Go type: %v", err)
		}
		want := responseFor("record_reason")
		wantRaw, _ := os.ReadFile(filepath.Join(apiDir(t), "record_reason.response.example.json"))
		decoder = json.NewDecoder(bytes.NewReader(wantRaw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(want); err != nil {
			t.Fatalf("record_reason fixture does not match its Go type: %v", err)
		}
		actualMap := toMap(t, out)
		wantMap := toMap(t, wantRaw)
		if actualMap["reason_id"] == "" {
			t.Fatal("record_reason: minted reason id is empty")
		}
		delete(actualMap, "reason_id")
		delete(wantMap, "reason_id")
		if !reflect.DeepEqual(actualMap, wantMap) {
			t.Fatalf("record_reason differs:\nactual:   %s\nwant: %s", pretty(actualMap), pretty(wantMap))
		}
	})
	t.Run("record_reason_unclear", func(t *testing.T) {
		svc := seededService(t)
		req := map[string]any{
			"shopper_id":  "shopper_01",
			"question_id": "q_9f04ab",
			"answer_text": "not great",
			"trace_id":    "trace_3f8a03",
		}
		raw, _ := json.Marshal(req)
		out, err := svc.dispatch(context.Background(), "record_reason", raw)
		if err != nil {
			t.Fatalf("record_reason unclear: %v", err)
		}
		actual := responseFor("record_reason")
		decoder := json.NewDecoder(bytes.NewReader(out))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(actual); err != nil {
			t.Fatalf("record_reason unclear response does not match its Go type: %v", err)
		}
		wantRaw, _ := os.ReadFile(filepath.Join(apiDir(t), "record_reason.response.example-unclear.json"))
		actualMap := toMap(t, out)
		wantMap := toMap(t, wantRaw)
		if actualMap["reason_id"] == "" {
			t.Fatal("record_reason unclear: minted reason id is empty")
		}
		delete(actualMap, "reason_id")
		delete(wantMap, "reason_id")
		if !reflect.DeepEqual(actualMap, wantMap) {
			t.Fatalf("record_reason unclear differs:\nactual:   %s\nwant: %s", pretty(actualMap), pretty(wantMap))
		}
	})
	t.Run("record_feedback", func(t *testing.T) {
		compareExample(t, seededService(t), "record_feedback",
			"record_feedback.request.example.json", "record_feedback.response.example.json", "feedback_id")
	})
	t.Run("explain_belief", func(t *testing.T) {
		compareExample(t, seededService(t), "explain_belief",
			"explain_belief.request.example.json", "explain_belief.response.example.json")
	})
	t.Run("ledger_beliefs", func(t *testing.T) {
		compareExample(t, seededService(t), "ledger_beliefs",
			"ledger_beliefs.request.example.json", "ledger_beliefs.response.example.json")
	})
	t.Run("ledger_belief", func(t *testing.T) {
		compareExample(t, seededService(t), "ledger_belief",
			"ledger_belief.request.example.json", "ledger_belief.response.example.json")
	})
	t.Run("ledger_evidence", func(t *testing.T) {
		compareExample(t, seededService(t), "ledger_evidence",
			"ledger_evidence.request.example.json", "ledger_evidence.response.example.json")
	})
}

// TestLedgerReadsNarrow proves the ledger filters: unseeded rows only,
// and one status only.
func TestLedgerReadsNarrow(t *testing.T) {
	ctx := context.Background()
	svc := seededService(t)
	unseeded, err := svc.LedgerBeliefs(ctx, contract.LedgerBeliefsRequest{ShopperID: "shopper_01"})
	if err != nil {
		t.Fatalf("ledger beliefs: %v", err)
	}
	for _, row := range unseeded.Beliefs {
		if row.Belief.Seeded {
			t.Errorf("unseeded read carries seeded belief %q", row.Belief.BeliefID)
		}
	}
	if len(unseeded.Beliefs) != 2 {
		t.Errorf("unseeded read carries %d beliefs, want 2", len(unseeded.Beliefs))
	}
	outOfTrue, err := svc.LedgerBeliefs(ctx, contract.LedgerBeliefsRequest{ShopperID: "shopper_01", StatusFilter: domain.StatusOutOfTrue, IncludeSeeded: true})
	if err != nil {
		t.Fatalf("ledger beliefs filter: %v", err)
	}
	if len(outOfTrue.Beliefs) != 1 || outOfTrue.Beliefs[0].Belief.BeliefID != "blf_8f2c41" {
		t.Errorf("status filter carries %v, want [blf_8f2c41]", beliefIDs(outOfTrue))
	}
	scoped, err := svc.LedgerEvidence(ctx, contract.LedgerEvidenceRequest{ShopperID: "shopper_01", BeliefID: "blf_8f2c41"})
	if err != nil {
		t.Fatalf("ledger evidence filter: %v", err)
	}
	if len(scoped.Evidence) != 3 {
		t.Errorf("belief-scoped evidence carries %d rows, want 3", len(scoped.Evidence))
	}
}

// beliefIDs lists the belief ids in a ledger response.
func beliefIDs(res contract.LedgerBeliefsResponse) []string {
	out := []string{}
	for _, row := range res.Beliefs {
		out = append(out, row.Belief.BeliefID)
	}
	return out
}
