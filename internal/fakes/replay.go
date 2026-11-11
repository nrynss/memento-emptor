package fakes

import (
	"context"
	"fmt"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
)

// The story's graph contents. Short, keyed, exact: the graph holds stubs,
// never prose, and every content is covered by the committed story.
const (
	// BeliefRunsSmall is the sizing belief the three returns earn.
	BeliefRunsSmall = "runs small, take size L"
	// BeliefUsuallyM is the shopper's general statement. It is an
	// assertion and never promotes.
	BeliefUsuallyM = "shopper says they are usually size M"
	// ReturnOne, ReturnTwo and ReturnThree are the three documented fit
	// returns, one structural write each.
	ReturnOne   = "Fit return 1: oxford shirt size M refunded; too tight in the shoulders"
	ReturnTwo   = "Fit return 2: oxford shirt size M refunded; too tight across the chest"
	ReturnThree = "Fit return 3: oxford shirt size M refunded; sleeves too short in M"
	// CorrectionLostWeight is the shopper's explicit correction, declared
	// against the sizing belief it contradicts.
	CorrectionLostWeight = "I've lost weight, use M"
	// DisputeNeverArrived is the non-arrival dispute. It counts toward
	// the seller, never toward sizing.
	DisputeNeverArrived = "Dispute opened: chino parcel never arrived; carrier lost it"
	// BeliefShipsReliably is the seller belief the dispute counts toward.
	// One dispute never promotes it.
	BeliefShipsReliably = "ships reliably for this shopper"
	// BeliefSizeMNow is the emerging rival belief the fits-well feedback
	// supports. One data point never promotes it.
	BeliefSizeMNow = "size M fits this shopper now"
	// FeedbackFitsWell is the fit feedback tied to the approved purchase.
	FeedbackFitsWell = "Fit feedback: oxford shirt size M fits well"
)

// StoryRecallQuery is the recorded recall query the agent asks before
// recommending. Recall serves only recorded queries.
const StoryRecallQuery = "oxford shirt sizing for shopper_01"

// GraphContents lists every content the story covers in the graph fake.
func GraphContents() []string {
	return []string{
		BeliefRunsSmall,
		BeliefUsuallyM,
		ReturnOne,
		ReturnTwo,
		ReturnThree,
		CorrectionLostWeight,
		DisputeNeverArrived,
		BeliefShipsReliably,
		BeliefSizeMNow,
		FeedbackFitsWell,
	}
}

// StoryRecalls is the recorded recall table: one query, the hits the
// agent reads before recommending.
func StoryRecalls() map[string][]string {
	return map[string][]string{
		StoryRecallQuery: {BeliefRunsSmall, BeliefUsuallyM, ReturnOne, ReturnTwo, ReturnThree},
	}
}

// StoryStep is one committed line of the replay log, from the first
// webhook to the final ledger state.
type StoryStep struct {
	// N is the step number, in order.
	N int `json:"n"`
	// At is the event time of the step, RFC 3339.
	At string `json:"at"`
	// Summary states the step in one line.
	Summary string `json:"summary"`
}

// Waypoint is a machine-checked point in the replay: the graph's verdict
// on one belief, plus how many refunds still miss a reason.
type Waypoint struct {
	// Step is the story step the verdict follows.
	Step int `json:"step"`
	// Content is the inspected belief.
	Content string `json:"content"`
	// State is the promotion state.
	State domain.GraphState `json:"state"`
	// Dependents counts the structural evidence citing the belief.
	Dependents int `json:"dependents"`
	// Contradicted is true while a correction applies.
	Contradicted bool `json:"contradicted"`
	// PendingCount counts the refunds still missing a reason.
	PendingCount int `json:"pending_count"`
}

// Result is the replay's outcome: its committed log, its waypoints, and
// its final ledger state.
type Result struct {
	// Steps is the committed log, in order.
	Steps []StoryStep `json:"steps"`
	// Waypoints are the machine-checked verdicts along the way.
	Waypoints []Waypoint `json:"waypoints"`
	// Final is the final ledger state.
	Final FinalState `json:"final"`
}

// FinalState is the end of the story: the ledger rows P4 renders, and the
// one refund still missing its reason that P3 must ask about.
type FinalState struct {
	// Beliefs are the final ledger belief rows, strongest first.
	Beliefs []contract.LedgerBeliefRow `json:"beliefs"`
	// Evidence are the final ledger evidence rows, newest first.
	Evidence []contract.LedgerEvidenceRow `json:"evidence"`
	// Pending is the pending question left open at the end.
	Pending contract.PendingQuestionsResponse `json:"pending"`
}

// Deps wires the replay to its fakes. Each field is the interface the
// real P2 client implements, so the story runs unchanged on live clients.
type Deps struct {
	// Payments replays the webhook deliveries and transaction reads.
	Payments PaymentSource
	// Classifier replays the reason classifications.
	Classifier ReasonClassifier
	// Store replays the evidence indexing and search.
	Store EvidenceStore
	// Graph applies the researched promotion bars.
	Graph EvidenceGraph
}

// step appends one committed log line.
func step(log *[]StoryStep, n int, at, summary string) {
	*log = append(*log, StoryStep{N: n, At: at, Summary: summary})
}

// Run walks the demo shopper's whole story, from the first webhook to the
// final ledger state. It uses fixed event times, so promotion is
// deterministic without waiting, and it touches no key and no network:
// every answer comes from a recording or the committed story.
func Run(ctx context.Context, deps Deps) (Result, error) {
	var out Result
	log := &out.Steps
	pending := 0
	waypoint := func(n int, content string) error {
		report, err := deps.Graph.Inspect(ctx, content)
		if err != nil {
			return err
		}
		out.Waypoints = append(out.Waypoints, Waypoint{
			Step: n, Content: content, State: report.State,
			Dependents: report.Dependents, Contradicted: report.Contradicted,
			PendingCount: pending,
		})
		return nil
	}

	// Steps 1-2: the seeded past. A general statement that never
	// promotes, and the sizing belief the returns will earn.
	if err := deps.Graph.Derive(ctx, BeliefRunsSmall, "logic", OccurredAt("WH-2EV-01")); err != nil {
		return Result{}, err
	}
	if err := deps.Graph.Derive(ctx, BeliefUsuallyM, "observation", OccurredAt("WH-2EV-01")); err != nil {
		return Result{}, err
	}
	step(log, 1, "2026-10-14T10:00:00Z", "Seeded past: general statement recorded as an assertion; sizing belief derived.")

	// Steps 2-4: three refund webhooks. Each refund record carries no
	// reason, so the reason comes from the documented answer, classified
	// from its own recording; each classified return is one structural
	// write citing the belief.
	returns := []struct {
		event, order, refund, action, reason string
	}{
		{"WH-2EV-01", "ORD-7XK-01", "RFD-7XK-01", ReturnOne, "too tight in the shoulders"},
		{"WH-2EV-02", "ORD-7XK-02", "RFD-7XK-02", ReturnTwo, "too tight across the chest"},
		{"WH-2EV-03", "ORD-7XK-03", "RFD-7XK-03", ReturnThree, "sleeves too short in M"},
	}
	for i, ret := range returns {
		event, err := deps.Payments.VerifyWebhook(ctx, ret.event)
		if err != nil {
			return Result{}, err
		}
		if !event.Verified {
			return Result{}, fmt.Errorf("fakes: replay: webhook %s did not verify", ret.event)
		}
		if _, err := deps.Payments.GetOrder(ctx, ret.order); err != nil {
			return Result{}, err
		}
		record, err := deps.Payments.GetRefund(ctx, ret.refund)
		if err != nil {
			return Result{}, err
		}
		if record.HasReason {
			return Result{}, fmt.Errorf("fakes: replay: refund %s carries a reason; recordings carry none", ret.refund)
		}
		class, err := deps.Classifier.Classify(ctx, ret.reason)
		if err != nil {
			return Result{}, err
		}
		if AskAgain(class) || class.Category.CountsToward() != domain.BeliefKindFit {
			return Result{}, fmt.Errorf("fakes: replay: return %s answer %q classifies as %q at %v, want a clear fit reason",
				ret.refund, ret.reason, class.Category, class.Confidence)
		}
		if _, err := deps.Store.IndexDoc(ctx, ret.action); err != nil {
			return Result{}, err
		}
		if err := deps.Graph.RecordAction(ctx, ret.action, []string{BeliefRunsSmall}, nil, OccurredAt(ret.event)); err != nil {
			return Result{}, err
		}
		step(log, 2+i, OccurredAt(ret.event).Format(time.RFC3339), "Fit return "+ret.refund+" arrives by webhook; documented answer becomes structural evidence.")
		if err := waypoint(2+i, BeliefRunsSmall); err != nil {
			return Result{}, err
		}
	}

	// Step 5: the shopper's correction wins at once, and the belief stays.
	correctionAt, _ := time.Parse(time.RFC3339, "2026-11-05T14:02:11Z")
	if err := deps.Graph.RecordAction(ctx, CorrectionLostWeight, nil, []string{BeliefRunsSmall}, correctionAt); err != nil {
		return Result{}, err
	}
	step(log, 5, "2026-11-05T14:02:11Z", "Shopper correction: lost weight, take M. The very next recommendation is M; the L belief stands beside the warning.")
	if err := waypoint(5, BeliefRunsSmall); err != nil {
		return Result{}, err
	}

	// Step 6: the candidate search matches the returned line, and the
	// non-arrival dispute counts toward the seller, never toward sizing.
	sellerAt, _ := time.Parse(time.RFC3339, "2026-11-06T11:30:00Z")
	hits, err := deps.Store.Search(ctx, "Brand X oxford shirt")
	if err != nil {
		return Result{}, err
	}
	if len(hits) == 0 || hits[0].DocID != "p6" {
		return Result{}, fmt.Errorf("fakes: replay: candidate search does not rank the same-line shirt first")
	}
	disputeEvent, err := deps.Payments.VerifyWebhook(ctx, "WH-2EV-04")
	if err != nil {
		return Result{}, err
	}
	dispute, err := deps.Payments.GetDispute(ctx, "PP-D-51713")
	if err != nil {
		return Result{}, err
	}
	if dispute.Category != domain.CategoryNotArrived || disputeEvent.DisputeCode != domain.DisputeMerchandiseNotReceived {
		return Result{}, fmt.Errorf("fakes: replay: dispute reason does not route to the seller")
	}
	if err := deps.Graph.Derive(ctx, BeliefShipsReliably, "logic", sellerAt); err != nil {
		return Result{}, err
	}
	if err := deps.Graph.RecordAction(ctx, DisputeNeverArrived, []string{BeliefShipsReliably}, nil, sellerAt); err != nil {
		return Result{}, err
	}
	step(log, 6, "2026-11-06T11:30:00Z", "Candidate matches the returned line; dispute for a parcel that never arrived counts toward the seller only.")
	if err := waypoint(6, BeliefRunsSmall); err != nil {
		return Result{}, err
	}
	if err := waypoint(6, BeliefShipsReliably); err != nil {
		return Result{}, err
	}

	// Step 7: a new refund arrives with no reason and stays a pending
	// question. The shopper's vague answer falls below the confidence
	// floor, so the question stays open: the replay leaves it pending.
	if _, err := deps.Payments.VerifyWebhook(ctx, "WH-2EV-05"); err != nil {
		return Result{}, err
	}
	if _, err := deps.Payments.GetRefund(ctx, "RFD-7XK-05"); err != nil {
		return Result{}, err
	}
	pending = 1
	step(log, 7, "2026-11-07T09:15:00Z", "New refund arrives with no reason; it becomes a pending question and counts toward nothing.")
	class, err := deps.Classifier.Classify(ctx, "not great")
	if err != nil {
		return Result{}, err
	}
	if class.Category != domain.CategoryUnclear || !AskAgain(class) {
		return Result{}, fmt.Errorf("fakes: replay: vague answer did not ask again: %v", class)
	}
	step(log, 8, "2026-11-08T18:40:00Z", "Shopper answers vaguely; classification stays below the floor, so the question stays open.")

	// Steps 9-10: approval, the sandbox purchase of the M, and the
	// fits-well feedback tied to that purchase. One data point supports
	// the emerging belief without promoting it.
	captureAt := OccurredAt("WH-2EV-06")
	if _, err := deps.Payments.VerifyWebhook(ctx, "WH-2EV-06"); err != nil {
		return Result{}, err
	}
	if _, err := deps.Payments.GetOrder(ctx, "ORD-7XK-06"); err != nil {
		return Result{}, err
	}
	if err := deps.Graph.Derive(ctx, BeliefSizeMNow, "logic", captureAt); err != nil {
		return Result{}, err
	}
	if _, err := deps.Store.IndexDoc(ctx, FeedbackFitsWell); err != nil {
		return Result{}, err
	}
	if err := deps.Graph.RecordAction(ctx, FeedbackFitsWell, []string{BeliefSizeMNow}, nil, captureAt); err != nil {
		return Result{}, err
	}
	step(log, 9, "2026-11-08T20:10:00Z", "Approved purchase of the M; fit feedback arrives tied to that purchase.")
	if err := waypoint(9, BeliefSizeMNow); err != nil {
		return Result{}, err
	}
	if err := waypoint(9, BeliefRunsSmall); err != nil {
		return Result{}, err
	}

	out.Final = assembleFinal()
	return out, nil
}
