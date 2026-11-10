package pool

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// testLine is the line every reader test reads.
var testLine = domain.ProductLine{Brand: "sunspel", Category: "shirt", FitHandle: "regular"}

// testClock is the clock the tests pin the recent window to.
func testClock() time.Time {
	return time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
}

// fakeIndex is the index in memory. It records the queries it was asked, so
// a test can assert the recent window and that an unusable query never
// reached it.
type fakeIndex struct {
	result  Result
	err     error
	warmErr error
	warmed  int
	queries []Query
}

// Warm records the warm call and returns the configured error.
func (f *fakeIndex) Warm(context.Context) error {
	f.warmed++
	return f.warmErr
}

// Aggregate records the query and returns the configured answer.
func (f *fakeIndex) Aggregate(_ context.Context, q Query) (Result, error) {
	f.queries = append(f.queries, q)
	return f.result, f.err
}

// newTestReader builds a reader over the fake index with a pinned clock.
func newTestReader(t *testing.T, index *fakeIndex) *Reader {
	t.Helper()
	reader, err := NewReader(index, WithClock(testClock))
	if err != nil {
		t.Fatalf("build reader: %v", err)
	}
	return reader
}

// TestFitDecidesThroughTheDomainRule walks the rule's boundaries through the
// reader: the Building floor, the Earned floor, the 60% share, the two
// together directions, and the recent window that flips the label. The
// reader must not hold a second copy of any threshold.
func TestFitDecidesThroughTheDomainRule(t *testing.T) {
	cases := []struct {
		name   string
		all    Counts
		recent Counts
		status domain.BeliefStatus
		way    domain.FitDirection
	}{
		{
			name:   "two shoppers say nothing",
			all:    Counts{Settled: 10, Returns: DirectionCounts{TooSmall: 2}, Shoppers: ShopperCounts{Fit: 2, TooSmall: 2}},
			status: domain.StatusClaimed,
		},
		{
			name:   "three shoppers build",
			all:    Counts{Settled: 10, Returns: DirectionCounts{TooSmall: 3}, Shoppers: ShopperCounts{Fit: 3, TooSmall: 3}},
			status: domain.StatusBuilding,
			way:    domain.FitDirectionSmall,
		},
		{
			name:   "eight shoppers in one direction earn",
			all:    Counts{Settled: 24, Returns: DirectionCounts{TooSmall: 16, TooLarge: 8}, Shoppers: ShopperCounts{Fit: 16, TooSmall: 8, TooLarge: 8}},
			status: domain.StatusEarned,
			way:    domain.FitDirectionSmall,
		},
		{
			name: "feedback and returns are one direction",
			all: Counts{
				Settled:  12,
				Returns:  DirectionCounts{TooSmall: 4},
				Feedback: VerdictCounts{TooSmall: 4, FitsWell: 4},
				Shoppers: ShopperCounts{Fit: 8, TooSmall: 8},
			},
			status: domain.StatusEarned,
			way:    domain.FitDirectionSmall,
		},
		{
			name: "fits-well verdicts dilute the share",
			all: Counts{
				Settled:  80,
				Returns:  DirectionCounts{TooSmall: 40},
				Feedback: VerdictCounts{FitsWell: 40},
				Shoppers: ShopperCounts{Fit: 8, TooSmall: 8},
			},
			status: domain.StatusBuilding,
			way:    domain.FitDirectionSmall,
		},
		{
			name: "feedback-only direction evidence still earns",
			all: Counts{
				Settled:  10,
				Feedback: VerdictCounts{TooSmall: 10},
				Shoppers: ShopperCounts{Fit: 10, TooSmall: 10},
			},
			status: domain.StatusEarned,
			way:    domain.FitDirectionSmall,
		},
		{
			name:   "a quiet recent window keeps the earned label",
			all:    Counts{Settled: 24, Returns: DirectionCounts{TooSmall: 16, TooLarge: 8}, Shoppers: ShopperCounts{Fit: 16, TooSmall: 8, TooLarge: 8}},
			recent: Counts{Settled: 2, Returns: DirectionCounts{TooSmall: 2}, Shoppers: ShopperCounts{Fit: 2, TooSmall: 2}},
			status: domain.StatusEarned,
			way:    domain.FitDirectionSmall,
		},
		{
			name:   "the recent window earning the opposite flips to out of true",
			all:    Counts{Settled: 24, Returns: DirectionCounts{TooSmall: 16, TooLarge: 8}, Shoppers: ShopperCounts{Fit: 16, TooSmall: 8, TooLarge: 8}},
			recent: Counts{Settled: 8, Returns: DirectionCounts{TooLarge: 8}, Shoppers: ShopperCounts{Fit: 8, TooLarge: 8}},
			status: domain.StatusOutOfTrue,
			way:    domain.FitDirectionSmall,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			index := &fakeIndex{result: Result{All: tc.all, Recent: tc.recent}}
			fit, err := newTestReader(t, index).Fit(context.Background(), testLine, "M")
			if err != nil {
				t.Fatalf("fit: %v", err)
			}
			if fit.Verdict.Status != tc.status || fit.Verdict.Direction != tc.way {
				t.Errorf("verdict = %s/%q, want %s/%q",
					fit.Verdict.Status, fit.Verdict.Direction, tc.status, tc.way)
			}
			if fit.Line != testLine || fit.Size != "M" {
				t.Errorf("fit names %+v size %q, want %+v size M", fit.Line, fit.Size, testLine)
			}
			if !reflect.DeepEqual(fit.Counts, tc.all) {
				t.Errorf("counts = %+v, want the bucket the index answered %+v", fit.Counts, tc.all)
			}
		})
	}
}

// TestFitPlacesTheRecentWindow pins the window's start to one
// domain.PoolRecentWindow before the reader's clock, in event time.
func TestFitPlacesTheRecentWindow(t *testing.T) {
	index := &fakeIndex{}
	if _, err := newTestReader(t, index).Fit(context.Background(), testLine, "M"); err != nil {
		t.Fatalf("fit: %v", err)
	}
	if len(index.queries) != 1 {
		t.Fatalf("index asked %d times, want once", len(index.queries))
	}
	query := index.queries[0]
	if query.Line != testLine || query.Size != "M" {
		t.Errorf("query names %+v size %q, want %+v size M", query.Line, query.Size, testLine)
	}
	want := testClock().Add(-domain.PoolRecentWindow)
	if !query.Since.Equal(want) {
		t.Errorf("recent window starts %s, want %s", query.Since, want)
	}
}

// TestEmptyPoolIsAnAnswerNotAnError separates a valid empty pool from an
// index that could not be reached: the first is Claimed, the second is an
// error, and the reader never turns one into the other.
func TestEmptyPoolIsAnAnswerNotAnError(t *testing.T) {
	index := &fakeIndex{result: Result{}}
	fit, err := newTestReader(t, index).Fit(context.Background(), testLine, "M")
	if err != nil {
		t.Fatalf("an empty pool is an answer, got error: %v", err)
	}
	if fit.Verdict.Status != domain.StatusClaimed || fit.Verdict.Direction != domain.FitDirectionNone {
		t.Errorf("empty pool reads %s/%q, want Claimed/no direction",
			fit.Verdict.Status, fit.Verdict.Direction)
	}
	if fit.Counts.Settled != 0 {
		t.Errorf("empty pool settled = %d, want 0", fit.Counts.Settled)
	}
}

// TestUnreachableIndexIsADistinctOutcome proves the reader reports an index
// it cannot reach with ErrUnavailable and no counts, both when the
// connection is refused and when the request times out.
func TestUnreachableIndexIsADistinctOutcome(t *testing.T) {
	cases := []struct {
		name   string
		server func(t *testing.T) *HTTPIndex
	}{
		{
			name: "connection refused",
			server: func(t *testing.T) *HTTPIndex {
				srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
				url := srv.URL
				srv.Close()
				index, err := NewHTTPIndex(url, "test-key", "fit-rows")
				if err != nil {
					t.Fatalf("build index: %v", err)
				}
				return index
			},
		},
		{
			name: "request timed out",
			server: func(t *testing.T) *HTTPIndex {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					time.Sleep(200 * time.Millisecond)
					w.WriteHeader(http.StatusOK)
				}))
				t.Cleanup(srv.Close)
				index, err := NewHTTPIndex(srv.URL, "test-key", "fit-rows", WithTimeout(20*time.Millisecond))
				if err != nil {
					t.Fatalf("build index: %v", err)
				}
				return index
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader, err := NewReader(tc.server(t))
			if err != nil {
				t.Fatalf("build reader: %v", err)
			}
			fit, err := reader.Fit(context.Background(), testLine, "M")
			if !errors.Is(err, ErrUnavailable) {
				t.Fatalf("fit error = %v, want ErrUnavailable", err)
			}
			if !reflect.DeepEqual(fit, Fit{}) {
				t.Errorf("unreachable index returned a fit %+v, want none", fit)
			}
			if err := reader.Warm(context.Background()); !errors.Is(err, ErrUnavailable) {
				t.Errorf("warm error = %v, want ErrUnavailable", err)
			}
		})
	}
}

// TestFitRefusesAnIncompleteQuery keeps a query that would match more than
// one line or size from reaching the index.
func TestFitRefusesAnIncompleteQuery(t *testing.T) {
	cases := []struct {
		name string
		line domain.ProductLine
		size string
	}{
		{"no brand", domain.ProductLine{Category: "shirt", FitHandle: "regular"}, "M"},
		{"no category", domain.ProductLine{Brand: "sunspel", FitHandle: "regular"}, "M"},
		{"no fit handle", domain.ProductLine{Brand: "sunspel", Category: "shirt"}, "M"},
		{"no size", testLine, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			index := &fakeIndex{}
			_, err := newTestReader(t, index).Fit(context.Background(), tc.line, tc.size)
			if !errors.Is(err, ErrMissingQuery) {
				t.Fatalf("fit error = %v, want ErrMissingQuery", err)
			}
			if len(index.queries) != 0 {
				t.Errorf("index asked %d times for an incomplete query, want none", len(index.queries))
			}
		})
	}
}

// TestNewReaderRequiresAnIndex pins the constructor's refusal.
func TestNewReaderRequiresAnIndex(t *testing.T) {
	if _, err := NewReader(nil); !errors.Is(err, ErrMissingIndex) {
		t.Fatalf("NewReader(nil) = %v, want ErrMissingIndex", err)
	}
}

// TestCountsForTheRule pins the conversion from the index's counts to the
// rule's shape: returns and feedback are one direction, a fits-well verdict
// points nowhere, and the shopper counts carry the fit-only floor.
func TestCountsForTheRule(t *testing.T) {
	counts := Counts{
		Settled:  42,
		Kept:     5,
		Returns:  DirectionCounts{TooSmall: 9, TooLarge: 2},
		Feedback: VerdictCounts{FitsWell: 6, TooSmall: 3, TooLarge: 1},
		Shoppers: ShopperCounts{Fit: 17, TooSmall: 11, TooLarge: 3},
	}
	got := counts.poolCounts()
	want := domain.PoolCounts{
		TooSmall:         12,
		TooLarge:         3,
		FitsWell:         6,
		ShoppersTooSmall: 11,
		ShoppersTooLarge: 3,
		Shoppers:         17,
	}
	if got != want {
		t.Errorf("poolCounts = %+v, want %+v", got, want)
	}
}
