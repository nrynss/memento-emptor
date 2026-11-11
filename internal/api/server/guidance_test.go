package server

import (
	"context"
	"strings"
	"testing"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/pool"
)

// earnedLargePool mirrors earnedSmallPool with the two directions swapped,
// so the pool earns large and recommends one rung down from the size it was
// consulted at. The reasons swap with the counts they explain.
func earnedLargePool() pool.Result {
	result := earnedSmallPool()
	swap := func(counts pool.Counts) pool.Counts {
		counts.Returns.TooSmall, counts.Returns.TooLarge = counts.Returns.TooLarge, counts.Returns.TooSmall
		counts.Feedback.TooSmall, counts.Feedback.TooLarge = counts.Feedback.TooLarge, counts.Feedback.TooSmall
		counts.Shoppers.TooSmall, counts.Shoppers.TooLarge = counts.Shoppers.TooLarge, counts.Shoppers.TooSmall
		small := counts.Reasons[domain.CategoryFitTooSmall]
		counts.Reasons[domain.CategoryFitTooSmall] = counts.Reasons[domain.CategoryFitTooLarge]
		counts.Reasons[domain.CategoryFitTooLarge] = small
		return counts
	}
	result.All = swap(result.All)
	result.Seeded = swap(result.Seeded)
	result.Live = swap(result.Live)
	result.Recent = swap(result.Recent)
	return result
}

// answerOneSmall records one too-small return for the shopper's first
// refund, earning a Claimed runs-small belief.
func answerOneSmall(t *testing.T, svc *Service, shopper string) {
	t.Helper()
	if _, err := svc.IngestWebhook(context.Background(), shopper, "WH-2EV-01"); err != nil {
		t.Fatalf("ingest WH-2EV-01: %v", err)
	}
	if res := answerReason(t, svc, shopper, "WH-2EV-01", "too tight in the shoulders"); !res.Accepted {
		t.Fatal("the too-small answer was not accepted")
	}
}

// answerOneLarge records one too-large return for the shopper's first
// refund, earning a Claimed runs-large belief.
func answerOneLarge(t *testing.T, svc *Service, shopper string) {
	t.Helper()
	if _, err := svc.IngestWebhook(context.Background(), shopper, "WH-2EV-01"); err != nil {
		t.Fatalf("ingest WH-2EV-01: %v", err)
	}
	if res := answerReason(t, svc, shopper, "WH-2EV-01", "it swims on me, far too big"); !res.Accepted {
		t.Fatal("the too-large answer was not accepted")
	}
}

// answerFitsWell records one fits-well feedback on a fresh purchase,
// earning the emerging fits-well belief with its one row.
func answerFitsWell(t *testing.T, svc *Service, shopper string) {
	t.Helper()
	ctx := context.Background()
	if _, err := svc.IngestWebhook(ctx, shopper, "WH-2EV-06"); err != nil {
		t.Fatalf("ingest WH-2EV-06: %v", err)
	}
	res, err := svc.RecordFeedback(ctx, contract.RecordFeedbackRequest{
		ShopperID:    shopper,
		CustomID:     "memento-shopper01-06",
		SKU:          "PSW8GeK",
		Size:         "M",
		FeedbackText: "The M fits well across the shoulders.",
	})
	if err != nil {
		t.Fatalf("record fits-well feedback: %v", err)
	}
	if !res.Accepted || res.Verdict != contract.FeedbackFitsWell {
		t.Fatalf("fits-well feedback: accepted %v verdict %q, want true fits_well", res.Accepted, res.Verdict)
	}
}

// TestOwnEvidenceSentenceTellsTheTruth pins every shape of the own-evidence
// sentence: the direction clause states what the belief's own value found
// (fits well, runs small, runs large) with the row count, and the pool
// clause says whether the pool agrees or recommends another size. A pool
// that agrees is never called overridden in prose, a recall with no pool
// claims nothing about one, and precedence does not move: a present pool is
// still the weaker step, marked overridden, however its size compares.
func TestOwnEvidenceSentenceTellsTheTruth(t *testing.T) {
	cases := []struct {
		name string
		// pool builds the canned index result; nil leaves the pool unwired.
		pool  func() pool.Result
		setup func(t *testing.T, svc *Service, shopper string)
		want  string
		// forbid lists phrases the served sentence must not carry.
		forbid []string
	}{
		{
			name:   "lone fits-well row beside a pool that runs small",
			pool:   earnedSmallPool,
			setup:  answerFitsWell,
			want:   "Take size M for Brand X: your own one purchase says it fits well, ahead of the pool.",
			forbid: []string{"runs small", "return"},
		},
		{
			name:   "lone too-small row beside a pool that agrees",
			pool:   earnedSmallPool,
			setup:  answerOneSmall,
			want:   "Take size L for Brand X: your own one return says it runs small, and the pool agrees.",
			forbid: []string{"ahead of the pool"},
		},
		{
			name:   "too-large row beside a pool that agrees",
			pool:   earnedLargePool,
			setup:  answerOneLarge,
			want:   "Take size S for Brand X: your own one return says it runs large, and the pool agrees.",
			forbid: []string{"ahead of the pool"},
		},
		{
			name:   "two too-large returns beside a pool that agrees",
			pool:   earnedLargePool,
			setup:  answerTwoLarge,
			want:   "Take size S for Brand X: your own two returns say it runs large, and the pool agrees.",
			forbid: []string{"ahead of the pool"},
		},
		{
			name:   "two too-large returns beside a pool that disagrees",
			pool:   earnedSmallPool,
			setup:  answerTwoLarge,
			want:   "Take size S for Brand X: your own two returns say it runs large, ahead of the pool.",
			forbid: []string{"the pool agrees"},
		},
		{
			name:   "two too-large returns with no pool",
			setup:  answerTwoLarge,
			want:   "Take size S for Brand X",
			forbid: []string{"ahead of the pool", "the pool agrees"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var idx pool.Index
			if tc.pool != nil {
				idx = &fakePoolIndex{result: tc.pool()}
			}
			svc, _ := pivotHarness(t, idx, &fakeFitIndex{}, "")
			shopper := "own_guidance_shopper"
			tc.setup(t, svc, shopper)
			recall, err := svc.Recall(context.Background(), contract.RecallRequest{
				ShopperID: shopper, BrandID: "brand-x-001",
				Category: "shirts", FitHandle: "regular", SizeConsidered: "M",
			})
			if err != nil {
				t.Fatalf("recall: %v", err)
			}
			if recall.Guidance.Basis != contract.GuidanceOwnEvidence {
				t.Fatalf("guidance %+v, want own_evidence", recall.Guidance)
			}
			if recall.Guidance.Text != tc.want {
				t.Errorf("guidance text %q, want %q", recall.Guidance.Text, tc.want)
			}
			for _, phrase := range tc.forbid {
				if strings.Contains(recall.Guidance.Text, phrase) {
					t.Errorf("guidance text %q carries %q, want it never there", recall.Guidance.Text, phrase)
				}
			}
			if tc.pool == nil {
				if recall.Pool != nil {
					t.Fatalf("no pool wired, recall carries %+v", recall.Pool)
				}
				return
			}
			// The prose distinguishes an agreeing pool from one that was
			// overridden on the size; the step order does not. The pool is
			// still the weaker step, so its precedence stays overridden by
			// the own evidence that decided.
			if recall.Pool == nil || recall.Pool.Precedence == nil ||
				!recall.Pool.Precedence.Overridden || recall.Pool.Precedence.OverriddenBy == nil ||
				*recall.Pool.Precedence.OverriddenBy != contract.GuidanceOwnEvidence {
				t.Fatalf("pool precedence %+v, want the pool still overridden by own_evidence", recall.Pool)
			}
		})
	}
}

// TestOwnEvidenceWordsFollowTheValue pins the clause's source: the fit words
// come from the belief's own value, a fits-well value names the purchases its
// feedback came from rather than returns, and a value naming no direction
// leaves the clause stating the rows without inventing one.
func TestOwnEvidenceWordsFollowTheValue(t *testing.T) {
	cases := []struct {
		value  string
		words  string
		clause string
	}{
		{"runs small, take size L", "runs small", "your own two returns say it runs small"},
		{"runs large, take size S", "runs large", "your own two returns say it runs large"},
		{"size M fits this shopper now", "fits well", "your own two purchases say it fits well"},
		{"shopper says they usually take M", "", "your own two returns say so"},
	}
	for _, tc := range cases {
		if got := fitWords(tc.value); got != tc.words {
			t.Errorf("fitWords(%q) = %q, want %q", tc.value, got, tc.words)
		}
		if got := ownEvidenceClause(2, tc.words); got != tc.clause {
			t.Errorf("ownEvidenceClause(2, %q) = %q, want %q", tc.words, got, tc.clause)
		}
	}
}
