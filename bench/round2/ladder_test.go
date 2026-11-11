package round2

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/api/server"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// ladderAnswer names the answer the ladder drive hands the classifier for
// one direction. The text is irrelevant to the size: the classifier's
// category is what the layer reads, and the answer only has to be non-empty
// so the pending question settles.
const (
	ladderAnswerSmall = "the ladder drive's too-small answer"
	ladderAnswerLarge = "the ladder drive's too-large answer"
)

// ladderContract is the test-only embedder contract the evidence store
// checks at construction. The drive stores documents, so a usable contract
// is all it needs.
type ladderContract struct{}

// EmbeddingKind names the contract family.
func (ladderContract) EmbeddingKind() string { return "ladder-test" }

// Validate reports whether the contract is usable.
func (ladderContract) Validate() error { return nil }

// ladderClassifier answers each drive answer with one fit category. It is
// the seam's whole job here: the layer, not the classifier, composes the
// size.
type ladderClassifier struct{}

// Classify returns the category the answer names.
func (ladderClassifier) Classify(ctx context.Context, answer string) (fakes.Classification, error) {
	if err := ctx.Err(); err != nil {
		return fakes.Classification{}, err
	}
	switch answer {
	case ladderAnswerSmall:
		return fakes.Classification{Category: domain.CategoryFitTooSmall, Confidence: 1}, nil
	case ladderAnswerLarge:
		return fakes.Classification{Category: domain.CategoryFitTooLarge, Confidence: 1}, nil
	default:
		return fakes.Classification{}, fmt.Errorf("ladder test: unknown answer %q", answer)
	}
}

// ladderGraph accepts every content the drive derives. It is deliberately
// permissive: the drive pins the size the layer composes, and a graph fake
// with an allowlist of expected contents would hide a drifted value behind
// its own error instead of letting the assertion name it.
type ladderGraph struct{}

// Derive records one belief or statement.
func (*ladderGraph) Derive(context.Context, string, string, time.Time) error { return nil }

// RecordAction records one structural write.
func (*ladderGraph) RecordAction(context.Context, string, []string, []string, time.Time) error {
	return nil
}

// Recall answers no hits: the drive reads the ledger, never recall.
func (*ladderGraph) Recall(context.Context, string) ([]string, error) { return nil, nil }

// Inspect reports the content at no standing: the drive needs the belief
// served, not promoted.
func (*ladderGraph) Inspect(_ context.Context, content string) (fakes.BeliefReport, error) {
	return fakes.BeliefReport{Content: content, State: domain.GraphNone}, nil
}

// ladderCell is one drive: a shopper whose single return was worn at one
// rung and blamed one way.
type ladderCell struct {
	shopper string
	event   string
	size    string
	answer  string
	up      bool
}

// TestLadderMatchesTheLayer proves the reference reasoner's ladder is the
// served layer's. The reference reasoner restates the ladder (reason.go)
// because it runs offline; this test drives the layer's own path for every
// rung in both directions and reads the size the layer composes into the
// fit belief. A layer ladder that was reordered, extended, shortened or
// recased, without reason.go following, fails here rather than leaving the
// freeze holding an old order.
//
// The path is the served one end to end: a refund webhook through
// IngestWebhook, the shopper's answer through RecordReason, and the belief
// through LedgerBeliefs. Nothing here reads an unexported layer identifier
// or a hashed file.
func TestLadderMatchesTheLayer(t *testing.T) {
	if len(sizeLadder) == 0 {
		t.Fatal("the reference reasoner's ladder is empty")
	}
	ctx := context.Background()
	inputs, cells := ladderInputs(t)
	svc, err := server.NewService(server.Deps{
		Payments:   fakes.NewRecordingPaymentSource(inputs),
		Classifier: ladderClassifier{},
		Store:      ladderStore(t),
		Graph:      &ladderGraph{},
		SchemaDir:  filepath.Join("..", "..", "testdata", "api"),
		Clock:      func() time.Time { return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	for _, cell := range cells {
		t.Run(cell.shopper, func(t *testing.T) {
			if _, err := svc.IngestWebhook(ctx, cell.shopper, cell.event); err != nil {
				t.Fatalf("ingest %s: %v", cell.event, err)
			}
			pending, err := svc.PendingQuestions(ctx, contract.PendingQuestionsRequest{ShopperID: cell.shopper})
			if err != nil {
				t.Fatalf("pending questions: %v", err)
			}
			if len(pending.Questions) != 1 {
				t.Fatalf("pending questions = %d, want 1", len(pending.Questions))
			}
			answer, err := svc.RecordReason(ctx, contract.RecordReasonRequest{
				ShopperID:  cell.shopper,
				QuestionID: pending.Questions[0].QuestionID,
				AnswerText: cell.answer,
			})
			if err != nil {
				t.Fatalf("record reason: %v", err)
			}
			if !answer.Accepted || answer.BeliefID == nil {
				t.Fatalf("answer accepted %v with belief %v, want true and attached", answer.Accepted, answer.BeliefID)
			}
			ledger, err := svc.LedgerBeliefs(ctx, contract.LedgerBeliefsRequest{ShopperID: cell.shopper})
			if err != nil {
				t.Fatalf("ledger beliefs: %v", err)
			}
			got := ladderBeliefSize(t, ledger)
			direction := domain.FitDirectionLarge
			if cell.up {
				direction = domain.FitDirectionSmall
			}
			want := SizeForDirection(cell.size, direction)
			if got != want {
				t.Errorf("worn %s, blamed %q: the layer recommends %q, the reference reasoner's ladder says %q",
					cell.size, direction, got, want)
			}
		})
	}
}

// ladderInputs builds one refund event per rung and direction, and the cell
// that drives it. Every order carries exactly one line item at the rung, so
// the fit belief's size is the worn size and nothing else.
func ladderInputs(t *testing.T) (fakes.StoryInputs, []ladderCell) {
	t.Helper()
	var inputs fakes.StoryInputs
	var cells []ladderCell
	for _, size := range sizeLadder {
		for _, up := range []bool{true, false} {
			n := len(cells) + 1
			cell := ladderCell{
				shopper: fmt.Sprintf("ladder-%02d", n),
				event:   fmt.Sprintf("WH-LADDER-%02d", n),
				size:    size,
				answer:  ladderAnswerLarge,
				up:      up,
			}
			if up {
				cell.answer = ladderAnswerSmall
			}
			if strings.TrimSpace(size) == "" {
				t.Fatalf("the reference reasoner's ladder carries an empty rung")
			}
			link := domain.TransactionLink{
				OrderID:   fmt.Sprintf("ORD-LADDER-%02d", n),
				CaptureID: fmt.Sprintf("CAP-LADDER-%02d", n),
				RefundID:  fmt.Sprintf("RFD-LADDER-%02d", n),
				CustomID:  "memento-" + cell.shopper,
				EventID:   cell.event,
			}
			outcome := domain.Outcome{Kind: domain.OutcomeRefund, Status: "COMPLETED", AmountMinor: 5000, Currency: "USD"}
			inputs.Events = append(inputs.Events, fakes.WebhookEvent{
				EventID:    cell.event,
				EventType:  "PAYMENT.CAPTURE.REFUNDED",
				Verified:   true,
				Outcome:    outcome,
				Link:       link,
				Provenance: "ladder test: one refund per rung and direction",
			})
			inputs.Orders = append(inputs.Orders, fakes.OrderRecord{
				OrderID: fmt.Sprintf("ORD-LADDER-%02d", n),
				Items: []domain.Item{{
					BrandID:   "brand-ladder",
					BrandName: "Ladder Brand",
					LineName:  "ladder tee",
					SKU:       fmt.Sprintf("ladder-%02d", n),
					Size:      size,
					Category:  "shirts",
				}},
				Link:       link,
				Provenance: "ladder test: one line item per rung",
			})
			inputs.Refunds = append(inputs.Refunds, fakes.RefundRecord{
				RefundID:   fmt.Sprintf("RFD-LADDER-%02d", n),
				Outcome:    outcome,
				Link:       link,
				HasReason:  false,
				Provenance: "ladder test: a refund carries no reason",
			})
			cells = append(cells, cell)
		}
	}
	return inputs, cells
}

// ladderStore builds the evidence store the drive indexes its documents
// into.
func ladderStore(t *testing.T) fakes.EvidenceStore {
	t.Helper()
	store, err := evidence.NewStore(ladderContract{})
	if err != nil {
		t.Fatalf("open evidence store: %v", err)
	}
	return store
}

// ladderBeliefSize reads the size the ledger's one fit belief names. The
// value is prose ("runs small, take size L"), so the size is the last
// token; a value naming no size fails the drive rather than passing it.
func ladderBeliefSize(t *testing.T, ledger contract.LedgerBeliefsResponse) string {
	t.Helper()
	var fit []string
	for _, row := range ledger.Beliefs {
		if row.Belief.Kind == domain.BeliefKindFit {
			fit = append(fit, row.Belief.Value)
		}
	}
	if len(fit) != 1 {
		t.Fatalf("ledger holds %d fit beliefs, want 1: %v", len(fit), fit)
	}
	fields := strings.Fields(fit[0])
	if len(fields) == 0 {
		t.Fatalf("fit belief value %q names no size", fit[0])
	}
	return fields[len(fields)-1]
}
