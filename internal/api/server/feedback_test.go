package server

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/fitindex"
	"github.com/nrynss/memento-emptor/internal/identity"
)

// feedbackSends is one purchase's feedback delivered three times: the
// first recording, an exact retry, and a duplicated delivery in other
// words. The layer joins on the purchase, not the words, so all three are
// the same feedback.
var feedbackSends = []struct {
	name string
	text string
}{
	{name: "first", text: "The M fits well across the shoulders."},
	{name: "retry", text: "The M fits well across the shoulders."},
	{name: "other words", text: "fits well on me"},
}

// TestFeedbackIsIdempotentPerPurchase proves one purchase's feedback is
// recorded once: three deliveries leave one evidence row on the belief,
// one feedback id, and the status the belief held after the first.
func TestFeedbackIsIdempotentPerPurchase(t *testing.T) {
	ctx := context.Background()
	shopper := "shopper_01"
	svc, spy := storyHarness(t)
	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-06"); err != nil {
		t.Fatalf("ingest capture: %v", err)
	}
	spy.mu.Lock()
	before := len(spy.actions)
	spy.mu.Unlock()

	var first contract.RecordFeedbackResponse
	var status domain.BeliefStatus
	var evidenceID string
	for _, send := range feedbackSends {
		res, err := svc.RecordFeedback(ctx, contract.RecordFeedbackRequest{
			ShopperID:    shopper,
			CustomID:     "memento-shopper01-06",
			SKU:          "PSW8GeK",
			Size:         "M",
			FeedbackText: send.text,
		})
		if err != nil {
			t.Fatalf("%s send: %v", send.name, err)
		}
		if !res.Accepted || res.Verdict != contract.FeedbackFitsWell {
			t.Fatalf("%s send: accepted %v verdict %q, want true fits_well", send.name, res.Accepted, res.Verdict)
		}
		if res.CountsToward == nil || *res.CountsToward != domain.BeliefKindFit {
			t.Fatalf("%s send: counts toward %v, want fit", send.name, res.CountsToward)
		}
		if res.BeliefID == nil {
			t.Fatalf("%s send: no belief named", send.name)
		}
		if first.BeliefID == nil {
			first = res
			if res.FeedbackID == "" {
				t.Fatal("first send minted no feedback id")
			}
		}
		explained, err := svc.ExplainBelief(ctx, contract.ExplainBeliefRequest{ShopperID: shopper, BeliefID: *first.BeliefID})
		if err != nil {
			t.Fatalf("%s send: explain belief: %v", send.name, err)
		}
		if len(explained.Belief.Evidence) != 1 {
			t.Fatalf("%s send left %d evidence rows, want 1", send.name, len(explained.Belief.Evidence))
		}
		if status == "" {
			status = explained.Belief.Status
			evidenceID = explained.Belief.Evidence[0].EvidenceID
		}
		if explained.Belief.Evidence[0].EvidenceID != evidenceID {
			t.Fatalf("%s send: the one row is %q, want the purchase's row %q",
				send.name, explained.Belief.Evidence[0].EvidenceID, evidenceID)
		}
		if explained.Belief.Status != status {
			t.Fatalf("%s send moved the status %q → %q, want it to stay", send.name, status, explained.Belief.Status)
		}
		if res.FeedbackID != first.FeedbackID {
			t.Fatalf("%s send: feedback id %q, want the first id %q", send.name, res.FeedbackID, first.FeedbackID)
		}
		if *res.BeliefID != *first.BeliefID {
			t.Fatalf("%s send: belief %q, want %q", send.name, *res.BeliefID, *first.BeliefID)
		}
	}

	spy.mu.Lock()
	after := len(spy.actions)
	spy.mu.Unlock()
	if after-before != 1 {
		t.Fatalf("three sends wrote %d actions, want 1", after-before)
	}
	st, err := svc.state(ctx, shopper)
	if err != nil {
		t.Fatalf("partition: %v", err)
	}
	if st.seq["fb"] != 1 {
		t.Fatalf("three sends minted %d feedback ids, want 1", st.seq["fb"])
	}
}

// TestSeededFeedbackIsNotDoubleCounted proves the seeded arc's own copy
// is the purchase's feedback: a delivery for a purchase the history
// already holds writes nothing, mints no id, and answers with the id the
// fixtures recorded for it. The evidence row listed once, its reason text
// untouched, and the belief's status unmoved.
func TestSeededFeedbackIsNotDoubleCounted(t *testing.T) {
	ctx := context.Background()
	shopper := "shopper_01"
	svc, spy := seededSpyService(t)
	spy.mu.Lock()
	before := len(spy.actions)
	spy.mu.Unlock()
	st, err := svc.state(ctx, shopper)
	if err != nil {
		t.Fatalf("partition: %v", err)
	}
	priorReason := st.evidence["ev_51c2d8"].row.Reason.Text

	for _, send := range feedbackSends {
		res, err := svc.RecordFeedback(ctx, contract.RecordFeedbackRequest{
			ShopperID:    shopper,
			CustomID:     "memento-shopper01-06",
			SKU:          "PSW8GeK",
			Size:         "M",
			FeedbackText: send.text,
		})
		if err != nil {
			t.Fatalf("%s send: %v", send.name, err)
		}
		explained, err := svc.ExplainBelief(ctx, contract.ExplainBeliefRequest{ShopperID: shopper, BeliefID: "blf_2b9d60"})
		if err != nil {
			t.Fatalf("%s send: explain seeded belief: %v", send.name, err)
		}
		if len(explained.Belief.Evidence) != 1 || explained.Belief.Evidence[0].EvidenceID != "ev_51c2d8" {
			t.Fatalf("%s send: the seeded belief lists %d evidence rows %v, want the one seeded copy ev_51c2d8",
				send.name, len(explained.Belief.Evidence), evidenceIDs(explained.Belief))
		}
		if explained.Belief.Status != domain.StatusClaimed {
			t.Fatalf("%s send: the seeded belief reads %q, want the Claimed it loaded with", send.name, explained.Belief.Status)
		}
		if res.FeedbackID != "fb_51c2d8" {
			t.Fatalf("%s send: feedback id %q, want the id the history recorded fb_51c2d8", send.name, res.FeedbackID)
		}
		if !res.Accepted || res.Verdict != contract.FeedbackFitsWell {
			t.Fatalf("%s send: accepted %v verdict %q, want true fits_well", send.name, res.Accepted, res.Verdict)
		}
		if res.BeliefID == nil || *res.BeliefID != "blf_2b9d60" {
			t.Fatalf("%s send: belief %v, want blf_2b9d60", send.name, res.BeliefID)
		}
	}

	explained, err := svc.ExplainBelief(ctx, contract.ExplainBeliefRequest{ShopperID: shopper, BeliefID: "blf_2b9d60"})
	if err != nil {
		t.Fatalf("explain seeded belief: %v", err)
	}
	if reason := explained.Belief.Evidence[0].Reason; reason == nil || reason.Text != priorReason {
		t.Fatalf("the seeded row's reason is %v, want its own words %q", reason, priorReason)
	}
	spy.mu.Lock()
	after := len(spy.actions)
	spy.mu.Unlock()
	if after != before {
		t.Fatalf("three sends wrote %d actions to the graph, want none", after-before)
	}
	if st.seq["fb"] != 0 {
		t.Fatalf("three sends minted %d feedback ids, want none", st.seq["fb"])
	}
}

// evidenceIDs lists the evidence ids one belief view carries.
func evidenceIDs(view contract.BeliefView) []string {
	out := make([]string, 0, len(view.Evidence))
	for _, ev := range view.Evidence {
		out = append(out, ev.EvidenceID)
	}
	return out
}

// TestRepeatFeedbackSettlesTheSeededPurchasesRowOnce proves the purchase
// still counts once in the pool: the history's own copy is a delivery for
// a purchase whose feedback is on record, and it settles the purchase's
// fit row from the verdict on record. Three sends leave one row, settled
// by fits-well feedback, so the pool's counts cannot move on a retry.
func TestRepeatFeedbackSettlesTheSeededPurchasesRowOnce(t *testing.T) {
	ctx := context.Background()
	shopper := "shopper_01"
	fitIdx := &fakeFitIndex{}
	svc, _ := seededFeedbackService(t, fitIdx, nil)

	for _, send := range feedbackSends {
		if _, err := svc.RecordFeedback(ctx, contract.RecordFeedbackRequest{
			ShopperID:    shopper,
			CustomID:     "memento-shopper01-06",
			SKU:          "PSW8GeK",
			Size:         "M",
			FeedbackText: send.text,
		}); err != nil {
			t.Fatalf("%s send: %v", send.name, err)
		}
		if got := fitIdx.rowCount(); got != 1 {
			t.Fatalf("%s send left %d fit rows, want the purchase's one", send.name, got)
		}
	}

	fitIdx.mu.Lock()
	defer fitIdx.mu.Unlock()
	for id, row := range fitIdx.rows {
		if !row.Settled || row.SettledKind != domain.FitSettledByFeedback || row.Verdict != domain.FitVerdictFitsWell {
			t.Fatalf("purchase %q reads settled %v by %q with verdict %q, want it settled by its fits-well feedback",
				id, row.Settled, row.SettledKind, row.Verdict)
		}
	}
}

// TestRepeatFeedbackAnswersAnIdTheHistoryDidNotName proves a seed that
// names no feedback id still answers with one. The frozen response schema
// forbids an empty feedback_id, so the layer mints the id it holds the
// row under, once, and every later repeat answers that same id.
func TestRepeatFeedbackAnswersAnIdTheHistoryDidNotName(t *testing.T) {
	ctx := context.Background()
	shopper := "shopper_01"
	svc, _ := seededFeedbackService(t, nil, func(seed *Seed) { seed.FeedbackIDs = map[string]string{} })
	req := contract.RecordFeedbackRequest{
		ShopperID:    shopper,
		CustomID:     "memento-shopper01-06",
		SKU:          "PSW8GeK",
		Size:         "M",
		FeedbackText: "The M fits well across the shoulders.",
	}

	first, err := svc.RecordFeedback(ctx, req)
	if err != nil {
		t.Fatalf("first send: %v", err)
	}
	if first.FeedbackID == "" {
		t.Fatal("the repeat answered with no feedback id, which the frozen response schema forbids")
	}
	second, err := svc.RecordFeedback(ctx, req)
	if err != nil {
		t.Fatalf("retry send: %v", err)
	}
	if second.FeedbackID != first.FeedbackID {
		t.Fatalf("retry answered %q, want the id it answered first %q", second.FeedbackID, first.FeedbackID)
	}
	st, err := svc.state(ctx, shopper)
	if err != nil {
		t.Fatalf("partition: %v", err)
	}
	if st.seq["fb"] != 1 {
		t.Fatalf("two sends minted %d feedback ids, want the one the row is held under", st.seq["fb"])
	}
}

// seededSpyService builds the seeded service with a spy over the graph,
// so a repeat delivery can be proved to write nothing.
func seededSpyService(t *testing.T) (*Service, *spyGraph) {
	t.Helper()
	return seededFeedbackService(t, nil, nil)
}

// seededFeedbackService builds the service the layer boots from, with a
// spy over the graph and, when fitIdx is not nil, the fit writer wired so a
// test can read the pool's rows. change edits the seed before it loads, for
// the seeds a hand-built history carries.
func seededFeedbackService(t *testing.T, fitIdx fitindex.Index, change func(*Seed)) (*Service, *spyGraph) {
	t.Helper()
	dir := apiDir(t)
	seed, err := SeedFromExamples(dir)
	if err != nil {
		t.Fatalf("build seed: %v", err)
	}
	if change != nil {
		change(&seed)
	}
	store, err := evidence.NewStore(testContract{kind: "server-feedback"})
	if err != nil {
		t.Fatalf("open evidence store: %v", err)
	}
	classifier, err := fakes.NewRecordingReasonClassifier(jevDir(t))
	if err != nil {
		t.Fatalf("open classifier fake: %v", err)
	}
	spy := &spyGraph{inner: fakes.NewRecordingEvidenceGraph(SeedGraphContents(seed), fakes.StoryRecalls())}
	fixed := time.Date(2026, 11, 8, 18, 40, 0, 0, time.UTC)
	svc, err := NewService(Deps{
		Payments:     fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()),
		Classifier:   classifier,
		Store:        store,
		Graph:        spy,
		SchemaDir:    dir,
		Clock:        func() time.Time { return fixed },
		FitRows:      fitIdx,
		Lines:        fitindex.LinesFunc(staticLines),
		IdentitySalt: identity.SeedSaltHex,
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	if err := svc.LoadSeed(context.Background(), seed); err != nil {
		t.Fatalf("load seed: %v", err)
	}
	return svc, spy
}

// freshFeedbackService builds the pivot service with the fit writer wired
// and one captured purchase, so a test can deliver that purchase's first
// feedback and read the pool's rows.
func freshFeedbackService(t *testing.T) (*Service, *spyGraph, *fakeFitIndex, string) {
	t.Helper()
	fitIdx := &fakeFitIndex{}
	svc, spy := pivotHarness(t, nil, fitIdx, "")
	shopper := "shopper_01"
	if _, err := svc.IngestWebhook(context.Background(), shopper, "WH-2EV-06"); err != nil {
		t.Fatalf("ingest capture: %v", err)
	}
	return svc, spy, fitIdx, shopper
}

// freshFeedbackRequest names the captured purchase freshFeedbackService
// ingested.
func freshFeedbackRequest(shopper, text string) contract.RecordFeedbackRequest {
	return contract.RecordFeedbackRequest{
		ShopperID:    shopper,
		CustomID:     "memento-shopper01-06",
		SKU:          "PSW8GeK",
		Size:         "M",
		FeedbackText: text,
	}
}

// fitRowsOf copies the fit index's rows under its lock, so a test can
// compare the pool's store before and after a delivery.
func fitRowsOf(f *fakeFitIndex) map[string]domain.FitRow {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[string]domain.FitRow, len(f.rows))
	for id, row := range f.rows {
		out[id] = row
	}
	return out
}

// fitBeliefsFor lists the fit beliefs stored for one brand.
func fitBeliefsFor(st *shopperState, brandID string) []*beliefRecord {
	out := []*beliefRecord{}
	for _, rec := range st.beliefs {
		if rec.kind == domain.BeliefKindFit && rec.subject.ID == brandID {
			out = append(out, rec)
		}
	}
	return out
}

// purchaseFeedback returns the feedback marker on the purchase's capture
// row, or nil while the purchase carries none.
func purchaseFeedback(t *testing.T, st *shopperState, customID string) *recordedFeedback {
	t.Helper()
	ids := st.captures[customID]
	if len(ids) == 0 {
		t.Fatalf("purchase %q has no capture row", customID)
	}
	rec := st.evidence[ids[0]]
	if rec == nil {
		t.Fatalf("purchase %q names an unknown capture row %q", customID, ids[0])
	}
	return rec.feedback
}

// TestSellerFeedbackWritesSellerEvidence proves a seller verdict is
// recorded honestly: it counts toward seller reliability, names the seller
// belief, writes one evidence action citing that belief, and leaves every
// fit belief and the purchase's fit row untouched. A repeat of the same
// verdict changes nothing.
func TestSellerFeedbackWritesSellerEvidence(t *testing.T) {
	ctx := context.Background()
	svc, spy, fitIdx, shopper := freshFeedbackService(t)
	st, err := svc.state(ctx, shopper)
	if err != nil {
		t.Fatalf("partition: %v", err)
	}
	rowsBefore := fitRowsOf(fitIdx)
	putsBefore := fitIdx.puts
	spy.mu.Lock()
	actionsBefore := len(spy.actions)
	spy.mu.Unlock()

	req := freshFeedbackRequest(shopper, "It arrived damaged: the seam is split.")
	first, err := svc.RecordFeedback(ctx, req)
	if err != nil {
		t.Fatalf("first seller feedback: %v", err)
	}
	if !first.Accepted {
		t.Fatalf("seller feedback answered accepted false, want the evidence recorded")
	}
	if first.Verdict != contract.FeedbackDamagedOrDefective {
		t.Fatalf("verdict %q, want damaged_or_defective", first.Verdict)
	}
	if first.CountsToward == nil || *first.CountsToward != domain.BeliefKindSeller {
		t.Fatalf("counts toward %v, want seller_reliability", first.CountsToward)
	}
	if first.BeliefID == nil {
		t.Fatal("seller feedback named no belief")
	}
	belief := st.beliefs[*first.BeliefID]
	if belief == nil || belief.kind != domain.BeliefKindSeller {
		t.Fatalf("belief %q is %+v, want the seller belief", *first.BeliefID, belief)
	}
	if fits := fitBeliefsFor(st, "brand-x-001"); len(fits) != 0 {
		t.Fatalf("seller feedback left %d fit beliefs, want none", len(fits))
	}
	row := st.evidence[st.captures["memento-shopper01-06"][0]]
	if row == nil || row.counts != domain.BeliefKindSeller {
		t.Fatalf("the purchase row counts toward %v, want seller_reliability", row)
	}
	if row.row.Reason == nil || row.row.Reason.Category != domain.CategoryDamagedOrDefective {
		t.Fatalf("the purchase row carries reason %+v, want damaged_or_defective", row.row.Reason)
	}
	if len(row.beliefIDs) != 1 || row.beliefIDs[0] != belief.id {
		t.Fatalf("the purchase row links %v, want the seller belief %q", row.beliefIDs, belief.id)
	}
	if len(belief.evIDs) != 1 || belief.evIDs[0] != row.id {
		t.Fatalf("the seller belief holds %v, want the purchase row %q", belief.evIDs, row.id)
	}

	spy.mu.Lock()
	actions := append([]recordedAction(nil), spy.actions...)
	spy.mu.Unlock()
	if len(actions) != actionsBefore+1 {
		t.Fatalf("seller feedback wrote %d actions, want 1", len(actions)-actionsBefore)
	}
	last := actions[len(actions)-1]
	if len(last.dependsOn) != 1 || last.dependsOn[0] != belief.content {
		t.Fatalf("seller feedback cites %v, want the seller belief %q", last.dependsOn, belief.content)
	}
	if after := fitRowsOf(fitIdx); !reflect.DeepEqual(after, rowsBefore) {
		t.Fatalf("seller feedback moved the fit rows:\nbefore: %+v\nafter:  %+v", rowsBefore, after)
	}
	if fitIdx.puts != putsBefore {
		t.Fatalf("seller feedback wrote %d fit rows, want none", fitIdx.puts-putsBefore)
	}

	second := req
	second.FeedbackText = "The shirt was broken when I opened the parcel."
	again, err := svc.RecordFeedback(ctx, second)
	if err != nil {
		t.Fatalf("repeat seller feedback: %v", err)
	}
	if again.FeedbackID != first.FeedbackID {
		t.Fatalf("repeat answered %q, want the first id %q", again.FeedbackID, first.FeedbackID)
	}
	if !again.Accepted || again.CountsToward == nil || *again.CountsToward != domain.BeliefKindSeller {
		t.Fatalf("repeat answered accepted %v counts %v, want accepted seller evidence", again.Accepted, again.CountsToward)
	}
	if again.BeliefID == nil || *again.BeliefID != *first.BeliefID {
		t.Fatalf("repeat answered belief %v, want %q", again.BeliefID, *first.BeliefID)
	}
	spy.mu.Lock()
	afterRepeat := len(spy.actions)
	spy.mu.Unlock()
	if afterRepeat != actionsBefore+1 {
		t.Fatalf("repeat wrote %d actions, want none", afterRepeat-(actionsBefore+1))
	}
	if after := fitRowsOf(fitIdx); !reflect.DeepEqual(after, rowsBefore) {
		t.Fatalf("repeat moved the fit rows:\nbefore: %+v\nafter:  %+v", rowsBefore, after)
	}
	if fits := fitBeliefsFor(st, "brand-x-001"); len(fits) != 0 {
		t.Fatalf("repeat left %d fit beliefs, want none", len(fits))
	}
	if st.seq["fb"] != 1 {
		t.Fatalf("two seller deliveries minted %d feedback ids, want 1", st.seq["fb"])
	}
}

// TestNotArrivedFeedbackWritesSellerEvidence proves the second seller
// verdict takes the same path with its own category, so the row reads
// non-arrival and never damage.
func TestNotArrivedFeedbackWritesSellerEvidence(t *testing.T) {
	ctx := context.Background()
	svc, _, fitIdx, shopper := freshFeedbackService(t)
	st, err := svc.state(ctx, shopper)
	if err != nil {
		t.Fatalf("partition: %v", err)
	}
	rowsBefore := fitRowsOf(fitIdx)

	res, err := svc.RecordFeedback(ctx, freshFeedbackRequest(shopper, "It never arrived."))
	if err != nil {
		t.Fatalf("seller feedback: %v", err)
	}
	if !res.Accepted || res.Verdict != contract.FeedbackNotArrived {
		t.Fatalf("answered accepted %v verdict %q, want true not_arrived", res.Accepted, res.Verdict)
	}
	if res.CountsToward == nil || *res.CountsToward != domain.BeliefKindSeller {
		t.Fatalf("counts toward %v, want seller_reliability", res.CountsToward)
	}
	if res.BeliefID == nil {
		t.Fatal("seller feedback named no belief")
	}
	belief := st.beliefs[*res.BeliefID]
	if belief == nil || belief.kind != domain.BeliefKindSeller {
		t.Fatalf("belief %q is %+v, want the seller belief", *res.BeliefID, belief)
	}
	row := st.evidence[st.captures["memento-shopper01-06"][0]]
	if row == nil || row.row.Reason == nil || row.row.Reason.Category != domain.CategoryNotArrived {
		t.Fatalf("the purchase row carries reason %+v, want not_arrived", row)
	}
	if fits := fitBeliefsFor(st, "brand-x-001"); len(fits) != 0 {
		t.Fatalf("seller feedback left %d fit beliefs, want none", len(fits))
	}
	if after := fitRowsOf(fitIdx); !reflect.DeepEqual(after, rowsBefore) {
		t.Fatalf("seller feedback moved the fit rows:\nbefore: %+v\nafter:  %+v", rowsBefore, after)
	}
}

// TestSameVerdictRepeatIsIdempotent proves a repeat of the recorded
// verdict, in the same or other words, writes nothing and answers the
// first answer: one feedback id, one evidence row, one belief, and
// accepted echoing whether the verdict counts toward anything.
func TestSameVerdictRepeatIsIdempotent(t *testing.T) {
	cases := []struct {
		name    string
		first   string
		second  string
		verdict contract.FeedbackVerdict
		counts  domain.BeliefKind
	}{
		{name: "fit", first: "The M fits well across the shoulders.", second: "fits great on me", verdict: contract.FeedbackFitsWell, counts: domain.BeliefKindFit},
		{name: "seller", first: "It arrived damaged.", second: "The item was broken in the box.", verdict: contract.FeedbackDamagedOrDefective, counts: domain.BeliefKindSeller},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc, spy, fitIdx, shopper := freshFeedbackService(t)
			st, err := svc.state(ctx, shopper)
			if err != nil {
				t.Fatalf("partition: %v", err)
			}
			first, err := svc.RecordFeedback(ctx, freshFeedbackRequest(shopper, tc.first))
			if err != nil {
				t.Fatalf("first send: %v", err)
			}
			if !first.Accepted || first.Verdict != tc.verdict {
				t.Fatalf("first send: accepted %v verdict %q, want true %q", first.Accepted, first.Verdict, tc.verdict)
			}
			spy.mu.Lock()
			actions := len(spy.actions)
			spy.mu.Unlock()
			rows := fitRowsOf(fitIdx)

			second, err := svc.RecordFeedback(ctx, freshFeedbackRequest(shopper, tc.second))
			if err != nil {
				t.Fatalf("repeat send: %v", err)
			}
			if second.FeedbackID != first.FeedbackID {
				t.Fatalf("repeat answered %q, want the first id %q", second.FeedbackID, first.FeedbackID)
			}
			if second.Verdict != tc.verdict || second.Accepted != true {
				t.Fatalf("repeat answered %q accepted %v, want %q accepted true", second.Verdict, second.Accepted, tc.verdict)
			}
			if second.CountsToward == nil || *second.CountsToward != tc.counts {
				t.Fatalf("repeat counts toward %v, want %q", second.CountsToward, tc.counts)
			}
			if second.BeliefID == nil || first.BeliefID == nil || *second.BeliefID != *first.BeliefID {
				t.Fatalf("repeat answered belief %v, want the first belief %v", second.BeliefID, first.BeliefID)
			}
			spy.mu.Lock()
			after := len(spy.actions)
			spy.mu.Unlock()
			if after != actions {
				t.Fatalf("repeat wrote %d actions, want none", after-actions)
			}
			if !reflect.DeepEqual(fitRowsOf(fitIdx), rows) {
				t.Fatal("repeat moved the purchase's fit row")
			}
			purchase := purchaseFeedback(t, st, "memento-shopper01-06")
			if purchase == nil || purchase.id != first.FeedbackID {
				t.Fatalf("purchase holds feedback %+v, want %q", purchase, first.FeedbackID)
			}
			if st.seq["fb"] != 1 {
				t.Fatalf("two sends minted %d feedback ids, want 1", st.seq["fb"])
			}
		})
	}
}

// TestConflictingFeedbackIsRefused proves a repeat whose words read as a
// different verdict is refused honestly: the sentinel matches, the error
// names the recorded verdict, and the store is exactly as the first
// delivery left it. Nothing is written and no accepted answer is given
// over words the purchase does not hold.
func TestConflictingFeedbackIsRefused(t *testing.T) {
	ctx := context.Background()

	t.Run("fresh purchase", func(t *testing.T) {
		svc, spy, fitIdx, shopper := freshFeedbackService(t)
		st, err := svc.state(ctx, shopper)
		if err != nil {
			t.Fatalf("partition: %v", err)
		}
		first, err := svc.RecordFeedback(ctx, freshFeedbackRequest(shopper, "The M fits well across the shoulders."))
		if err != nil {
			t.Fatalf("first send: %v", err)
		}
		rowsBefore := fitRowsOf(fitIdx)
		putsBefore := fitIdx.puts
		spy.mu.Lock()
		actionsBefore := len(spy.actions)
		spy.mu.Unlock()
		evidenceBefore := st.evidence[st.captures["memento-shopper01-06"][0]]
		reasonBefore := evidenceBefore.row.Reason

		_, err = svc.RecordFeedback(ctx, freshFeedbackRequest(shopper, "It arrived damaged."))
		if !errors.Is(err, ErrFeedbackConflict) {
			t.Fatalf("conflicting repeat error = %v, want ErrFeedbackConflict", err)
		}
		if !strings.Contains(err.Error(), "already holds a recorded") {
			t.Fatalf("conflicting repeat error %q does not name the recorded verdict", err)
		}
		spy.mu.Lock()
		actionsAfter := len(spy.actions)
		spy.mu.Unlock()
		if actionsAfter != actionsBefore {
			t.Fatalf("conflicting repeat wrote %d actions, want none", actionsAfter-actionsBefore)
		}
		if after := fitRowsOf(fitIdx); !reflect.DeepEqual(after, rowsBefore) {
			t.Fatalf("conflicting repeat moved the fit rows:\nbefore: %+v\nafter:  %+v", rowsBefore, after)
		}
		if fitIdx.puts != putsBefore {
			t.Fatalf("conflicting repeat wrote %d fit rows, want none", fitIdx.puts-putsBefore)
		}
		if st.evidence[evidenceBefore.id].row.Reason != reasonBefore {
			t.Fatal("conflicting repeat replaced the purchase's recorded reason")
		}
		if len(fitBeliefsFor(st, "brand-x-001")) != 1 {
			t.Fatal("conflicting repeat changed the brand's fit beliefs")
		}
		if len(st.beliefs) != 1 {
			t.Fatalf("conflicting repeat left %d beliefs, want the one fit belief", len(st.beliefs))
		}
		if st.seq["fb"] != 1 {
			t.Fatalf("conflicting repeat minted %d feedback ids, want the first only", st.seq["fb"])
		}
		if purchase := purchaseFeedback(t, st, "memento-shopper01-06"); purchase == nil || purchase.id != first.FeedbackID || purchase.verdict != contract.FeedbackFitsWell {
			t.Fatalf("purchase holds %+v, want the recorded fits-well feedback %q", purchase, first.FeedbackID)
		}
	})

	t.Run("seeded purchase", func(t *testing.T) {
		svc, spy := seededSpyService(t)
		st, err := svc.state(ctx, "shopper_01")
		if err != nil {
			t.Fatalf("partition: %v", err)
		}
		spy.mu.Lock()
		actionsBefore, derivesBefore := len(spy.actions), len(spy.derives)
		spy.mu.Unlock()
		evidenceBefore := st.evidence["ev_51c2d8"]
		reasonBefore := evidenceBefore.row.Reason.Text

		_, err = svc.RecordFeedback(ctx, freshFeedbackRequest("shopper_01", "It arrived damaged."))
		if !errors.Is(err, ErrFeedbackConflict) {
			t.Fatalf("conflicting repeat error = %v, want ErrFeedbackConflict", err)
		}
		if !strings.Contains(err.Error(), "already holds a recorded") {
			t.Fatalf("conflicting repeat error %q does not name the recorded verdict", err)
		}
		spy.mu.Lock()
		actionsAfter, derivesAfter := len(spy.actions), len(spy.derives)
		spy.mu.Unlock()
		if actionsAfter != actionsBefore || derivesAfter != derivesBefore {
			t.Fatalf("conflicting repeat wrote %d actions and %d derives, want none",
				actionsAfter-actionsBefore, derivesAfter-derivesBefore)
		}
		if len(st.beliefs) != 4 {
			t.Fatalf("conflicting repeat left %d beliefs, want the seeded four", len(st.beliefs))
		}
		if len(st.evidence["ev_51c2d8"].beliefIDs) != 1 || st.evidence["ev_51c2d8"].beliefIDs[0] != "blf_2b9d60" {
			t.Fatalf("conflicting repeat changed the seeded row's belief links: %v", st.evidence["ev_51c2d8"].beliefIDs)
		}
		if st.evidence["ev_51c2d8"].row.Reason.Text != reasonBefore {
			t.Fatal("conflicting repeat replaced the seeded row's reason")
		}
		if purchase := purchaseFeedback(t, st, "memento-shopper01-06"); purchase == nil || purchase.id != "fb_51c2d8" || purchase.verdict != contract.FeedbackFitsWell {
			t.Fatalf("purchase holds %+v, want the seeded fits-well feedback fb_51c2d8", purchase)
		}
		if st.seq["fb"] != 0 {
			t.Fatalf("conflicting repeat minted %d feedback ids, want none", st.seq["fb"])
		}
	})

	t.Run("no-count first delivery", func(t *testing.T) {
		svc, spy, fitIdx, shopper := freshFeedbackService(t)
		st, err := svc.state(ctx, shopper)
		if err != nil {
			t.Fatalf("partition: %v", err)
		}
		first, err := svc.RecordFeedback(ctx, freshFeedbackRequest(shopper, "I'm not sure yet."))
		if err != nil {
			t.Fatalf("first send: %v", err)
		}
		rowsBefore := fitRowsOf(fitIdx)
		spy.mu.Lock()
		actionsBefore, derivesBefore := len(spy.actions), len(spy.derives)
		spy.mu.Unlock()

		_, err = svc.RecordFeedback(ctx, freshFeedbackRequest(shopper, "The M fits well."))
		if !errors.Is(err, ErrFeedbackConflict) {
			t.Fatalf("conflicting repeat error = %v, want ErrFeedbackConflict", err)
		}
		spy.mu.Lock()
		actionsAfter, derivesAfter := len(spy.actions), len(spy.derives)
		spy.mu.Unlock()
		if actionsAfter != actionsBefore || derivesAfter != derivesBefore {
			t.Fatalf("conflicting repeat wrote %d actions and %d derives, want none",
				actionsAfter-actionsBefore, derivesAfter-derivesBefore)
		}
		if len(st.beliefs) != 0 {
			t.Fatalf("conflicting repeat created %d beliefs, want none", len(st.beliefs))
		}
		if !reflect.DeepEqual(fitRowsOf(fitIdx), rowsBefore) {
			t.Fatal("conflicting repeat moved the purchase's fit row")
		}
		if purchase := purchaseFeedback(t, st, "memento-shopper01-06"); purchase == nil || purchase.id != first.FeedbackID || purchase.verdict != contract.FeedbackUnclear {
			t.Fatalf("purchase holds %+v, want the recorded unclear feedback %q", purchase, first.FeedbackID)
		}
		if st.seq["fb"] != 1 {
			t.Fatalf("conflicting repeat minted %d feedback ids, want the first only", st.seq["fb"])
		}
	})
}

// TestNoCountFeedbackIsIdempotentOnResend proves a first delivery that
// counts toward nothing mints one id, records it on the purchase with its
// verdict, and answers every resend with that id and accepted false.
func TestNoCountFeedbackIsIdempotentOnResend(t *testing.T) {
	cases := []struct {
		name    string
		first   string
		second  string
		verdict contract.FeedbackVerdict
	}{
		{name: "unclear", first: "I'm not sure yet.", second: "Hard to say.", verdict: contract.FeedbackUnclear},
		{name: "changed mind", first: "I changed my mind.", second: "Actually I don't want it.", verdict: contract.FeedbackChangedMind},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc, spy, fitIdx, shopper := freshFeedbackService(t)
			st, err := svc.state(ctx, shopper)
			if err != nil {
				t.Fatalf("partition: %v", err)
			}
			rowsBefore := fitRowsOf(fitIdx)
			spy.mu.Lock()
			actionsBefore := len(spy.actions)
			spy.mu.Unlock()

			first, err := svc.RecordFeedback(ctx, freshFeedbackRequest(shopper, tc.first))
			if err != nil {
				t.Fatalf("first send: %v", err)
			}
			if first.Accepted || first.Verdict != tc.verdict || first.CountsToward != nil || first.BeliefID != nil {
				t.Fatalf("first send: accepted %v verdict %q counts %v belief %v, want accepted false %q toward nothing",
					first.Accepted, first.Verdict, first.CountsToward, first.BeliefID, tc.verdict)
			}
			if first.FeedbackID == "" {
				t.Fatal("first send minted no feedback id")
			}
			purchase := purchaseFeedback(t, st, "memento-shopper01-06")
			if purchase == nil || purchase.id != first.FeedbackID || purchase.verdict != tc.verdict {
				t.Fatalf("purchase holds %+v, want %q with verdict %q", purchase, first.FeedbackID, tc.verdict)
			}

			for _, text := range []string{tc.first, tc.second} {
				again, err := svc.RecordFeedback(ctx, freshFeedbackRequest(shopper, text))
				if err != nil {
					t.Fatalf("resend %q: %v", text, err)
				}
				if again.FeedbackID != first.FeedbackID {
					t.Fatalf("resend %q answered %q, want %q", text, again.FeedbackID, first.FeedbackID)
				}
				if again.Accepted || again.CountsToward != nil || again.BeliefID != nil {
					t.Fatalf("resend %q answered accepted %v counts %v belief %v, want accepted false toward nothing",
						text, again.Accepted, again.CountsToward, again.BeliefID)
				}
			}
			spy.mu.Lock()
			actionsAfter := len(spy.actions)
			spy.mu.Unlock()
			if actionsAfter != actionsBefore {
				t.Fatalf("three no-count deliveries wrote %d actions, want none", actionsAfter-actionsBefore)
			}
			if len(st.beliefs) != 0 {
				t.Fatalf("three no-count deliveries created %d beliefs, want none", len(st.beliefs))
			}
			if !reflect.DeepEqual(fitRowsOf(fitIdx), rowsBefore) {
				t.Fatal("no-count deliveries moved the purchase's fit row")
			}
			if st.seq["fb"] != 1 {
				t.Fatalf("three no-count deliveries minted %d feedback ids, want 1", st.seq["fb"])
			}
		})
	}
}
