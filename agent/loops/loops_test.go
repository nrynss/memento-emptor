package loops

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nrynss/memento-emptor/agent/orders"
	"github.com/nrynss/memento-emptor/internal/domain"
)

// replayTurns parses the frozen P3 turn shapes beside this package.
func replayTurns(t *testing.T) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "replay", "p3-turns.json"))
	if err != nil {
		t.Fatalf("read p3-turns.json: %v", err)
	}
	var replay struct {
		Turns []map[string]any `json:"turns"`
	}
	if err := json.Unmarshal(raw, &replay); err != nil {
		t.Fatalf("decode p3-turns.json: %v", err)
	}
	if len(replay.Turns) == 0 {
		t.Fatal("p3-turns.json carries no turns")
	}
	return replay.Turns
}

func TestAskOneNamesItemAndSize(t *testing.T) {
	question, err := AskOne([]PendingQuestion{{
		QuestionID: "q_9f04ab",
		LineName:   "oxford shirt",
		Size:       "M",
		CustomID:   "memento-shopper01-05",
	}})
	if err != nil {
		t.Fatalf("AskOne: %v", err)
	}
	if !strings.Contains(question, "oxford shirt") || !strings.Contains(question, "M") {
		t.Fatalf("question names neither item nor size: %q", question)
	}
	if strings.Count(question, "?") != 1 {
		t.Fatalf("the loop asks one question, not several: %q", question)
	}
}

func TestAskOneEmptyIsNoPending(t *testing.T) {
	if _, err := AskOne(nil); !errors.Is(err, ErrNoPending) {
		t.Fatalf("empty pending = %v, want ErrNoPending", err)
	}
	if _, err := AskOne([]PendingQuestion{{CustomID: "memento-shopper01-05"}}); !errors.Is(err, ErrMalformed) {
		t.Fatalf("question without id = %v, want ErrMalformed", err)
	}
}

func TestApprovalYesIsStrict(t *testing.T) {
	for _, yes := range []string{"yes", "Yes", "yes!", "approve", "Confirmed"} {
		if !IsApprovalYes(yes) {
			t.Errorf("reply %q should count as approval", yes)
		}
	}
	for _, no := range []string{"", "maybe", "yes but in blue", "ok", "yess", "no"} {
		if IsApprovalYes(no) {
			t.Errorf("reply %q must not count as approval", no)
		}
	}
}

// mirrorFunc adapts a function to the ApprovalMirror seam.
type mirrorFunc func(orderID string)

// MarkApproved records the approval on the adapted function.
func (f mirrorFunc) MarkApproved(orderID string) { f(orderID) }

func TestApproveAfterYesMirrorsBeforeCapture(t *testing.T) {
	ctx := context.Background()
	fake := orders.NewFakeCheckout()
	svc, err := orders.NewService(fake, orders.Config{MerchantID: "sandbox-merchant", Sandbox: true})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	proposed, _, err := svc.Propose(ctx, "shopper_01", "oxford-M", domain.Item{BrandID: "brand-x-001", LineName: "oxford shirt", Size: "M"}, 19500, "USD", "req-t34-1")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}

	if _, err := svc.Capture(ctx, proposed.OrderID); !errors.Is(err, orders.ErrNoApproval) {
		t.Fatalf("capture without approval = %v, want ErrNoApproval", err)
	}
	if fake.CaptureCalls != 0 {
		t.Fatalf("fail-closed capture reached the transport %d times", fake.CaptureCalls)
	}

	var mirrored []string
	mirror := mirrorFunc(func(orderID string) {
		mirrored = append(mirrored, orderID)
		fake.ApproveOrder(orderID)
	})
	if _, err := ApproveAfterYes(ctx, svc, mirror, proposed.OrderID, "maybe"); !errors.Is(err, ErrNotApproved) {
		t.Fatalf("vague reply = %v, want ErrNotApproved", err)
	}
	if len(mirrored) != 0 {
		t.Fatalf("non-yes reply mirrored %d approvals", len(mirrored))
	}

	approved, err := ApproveAfterYes(ctx, svc, mirror, proposed.OrderID, "yes")
	if err != nil {
		t.Fatalf("ApproveAfterYes: %v", err)
	}
	if approved.State != orders.StateApproved {
		t.Fatalf("after yes the order is %q, want approved", approved.State)
	}
	if len(mirrored) != 1 || mirrored[0] != proposed.OrderID {
		t.Fatalf("approval mirrored to %v, want [%s]", mirrored, proposed.OrderID)
	}

	captured, err := svc.Capture(ctx, proposed.OrderID)
	if err != nil {
		t.Fatalf("Capture after mirrored approval: %v", err)
	}
	if captured.State != orders.StateCaptured {
		t.Fatalf("after capture the order is %q, want captured", captured.State)
	}
}

func TestCorrectionWinsNextRecommendation(t *testing.T) {
	turns := replayTurns(t)
	var recall map[string]any
	for _, turn := range turns {
		if turn["tool"] == "recall_for_purchase" {
			recall = turn
			break
		}
	}
	if recall == nil {
		t.Fatal("replay carries no recall turn")
	}
	response := recall["response"].(map[string]any)
	guidance := response["guidance"].(map[string]any)
	correction := response["overriding_correction"].(map[string]any)
	next := RecommendationFor(
		correction["text"].(string),
		response["beliefs"].([]any)[0].(map[string]any)["value"].(string),
	)
	if next != "I've lost weight, use M" {
		t.Fatalf("next recommendation = %q, want the correction", next)
	}
	if guidance["basis"] != "correction" {
		t.Fatalf("recall guidance basis = %v, want correction", guidance["basis"])
	}
}

func TestReplayPinsAllFourLoops(t *testing.T) {
	byTool := map[string]map[string]any{}
	for _, turn := range replayTurns(t) {
		if tool, ok := turn["tool"].(string); ok && tool != "" {
			byTool[tool] = turn
		}
	}

	pending := byTool["pending_questions"]["response"].(map[string]any)["questions"].([]any)
	first := pending[0].(map[string]any)
	q, err := AskOne([]PendingQuestion{{
		QuestionID: first["question_id"].(string),
		LineName:   first["item"].(map[string]any)["line_name"].(string),
		Size:       first["item"].(map[string]any)["size"].(string),
		CustomID:   first["link"].(map[string]any)["custom_id"].(string),
	}})
	if err != nil {
		t.Fatalf("replay pending question: %v", err)
	}
	if !strings.Contains(q, "oxford shirt") {
		t.Fatalf("replay question asks about %q", q)
	}

	reason := byTool["record_reason"]["response"].(map[string]any)
	if reason["ask_again"] != true || reason["accepted"] != false {
		t.Fatalf("vague replay answer settles the question: %v", reason)
	}
	if reason["question_id"] != first["question_id"] {
		t.Fatalf("reason answers %v, pending asked %v", reason["question_id"], first["question_id"])
	}

	correction := byTool["record_correction"]["response"].(map[string]any)
	if correction["correction"].(map[string]any)["takes_precedence"] != true {
		t.Fatalf("replay correction takes no precedence: %v", correction)
	}

	feedback := byTool["record_feedback"]
	request := feedback["request"].(map[string]any)
	fb := Feedback{
		CustomID: request["custom_id"].(string),
		SKU:      request["sku"].(string),
		Size:     request["size"].(string),
		Text:     request["feedback_text"].(string),
	}
	if err := fb.Validate(); err != nil {
		t.Fatalf("replay feedback untied: %v", err)
	}
	if byTool["record_feedback"]["response"].(map[string]any)["accepted"] != true {
		t.Fatal("replay fits-well feedback not accepted")
	}
}

func TestFeedbackWithoutTransactionRefused(t *testing.T) {
	fb := Feedback{SKU: "PSW8GeK", Size: "M", Text: "The M fits well."}
	if err := fb.Validate(); !errors.Is(err, ErrMalformed) {
		t.Fatalf("untied feedback = %v, want ErrMalformed", err)
	}
}
