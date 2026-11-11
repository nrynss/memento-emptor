package evidence

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
)

// testSnapshot builds one evidence snapshot with two documents, a seeded
// row and a live row with its feedback marker, an open question, a stored
// event, a belief's citations and timeline, the mint counters and the
// last brand context.
func testSnapshot() Snapshot {
	at := time.Date(2026, 11, 7, 9, 15, 0, 0, time.UTC)
	line := Line{BrandID: "brand-x-001", Category: "shirts", FitHandle: "regular"}
	return Snapshot{
		Documents: []Document{
			{DocID: "doc-0001", Kind: KindReason, Text: "REFUND COMPLETED: Brand X oxford shirt, size M: too tight", Line: line, Seeded: true},
			{DocID: "doc-0002", Kind: KindMessage, Text: "The M fits well across the shoulders."},
		},
		Rows: []Row{
			{
				ID: "ev_6d1a09",
				Evidence: domain.Evidence{
					Source:     domain.SourcePayment,
					Outcome:    domain.Outcome{Kind: domain.OutcomeRefund, Status: "COMPLETED", AmountMinor: 19500, Currency: "USD"},
					Item:       domain.Item{BrandID: "brand-x-001", LineName: "oxford shirt", SKU: "tbnrl46", Size: "M", Category: "shirts"},
					Reason:     &domain.Reason{Category: domain.CategoryFitTooSmall, Text: "too tight", Confidence: 1},
					Link:       domain.TransactionLink{OrderID: "ORD-1", EventID: "WH-2EV-05"},
					Seeded:     true,
					OccurredAt: at,
				},
				CountsThatToward: domain.BeliefKindFit,
				WriteOp:          domain.WriteRecordAction,
				DocID:            "doc-0001",
				BeliefIDs:        []string{"blf_8f2c41"},
			},
			{
				ID: "ev_000001",
				Evidence: domain.Evidence{
					Outcome:    domain.Outcome{Kind: domain.OutcomeCapture, Status: "COMPLETED"},
					Item:       domain.Item{BrandID: "brand-x-001", SKU: "PSW8GeK", Size: "M"},
					Link:       domain.TransactionLink{CustomID: "memento-shopper01-06", EventID: "WH-2EV-06"},
					OccurredAt: at.Add(time.Hour),
				},
				CountsThatToward: domain.BeliefKindFit,
				WriteOp:          domain.WriteRecordAction,
				DocID:            "doc-0002",
				BeliefIDs:        []string{"blf_2b9d60"},
				Feedback:         &Feedback{ID: "fb_000001", Verdict: contract.FeedbackFitsWell, BeliefID: "blf_2b9d60"},
			},
		},
		Pending: []Pending{{
			ID:            "q_000001",
			EventID:       "WH-2EV-05",
			Outcome:       domain.Outcome{Kind: domain.OutcomeRefund, Status: "COMPLETED"},
			Items:         []domain.Item{{BrandID: "brand-x-001", SKU: "tbnrl46", Size: "M"}},
			Link:          domain.TransactionLink{OrderID: "ORD-1", EventID: "WH-2EV-05"},
			AskedCount:    1,
			LastAskedAt:   &at,
			AwaitingSince: at,
		}},
		Links: []BeliefEvidence{{
			BeliefID:    "blf_8f2c41",
			EvidenceIDs: []string{"ev_6d1a09", "ev_000001"},
			UpdatedAt:   at.Add(time.Hour),
		}},
		Timelines: []BeliefTimeline{{
			BeliefID: "blf_8f2c41",
			Events: []contract.TimelineEvent{{
				At: at.Format(time.RFC3339), Kind: contract.TimelineEvidence,
				Summary: "Fit return 1.", RefID: "ev_6d1a09",
			}},
		}},
		Seq:       map[string]int{"ev": 1, "rsn": 1, "q": 1},
		LastBrand: domain.Subject{Kind: domain.SubjectBrand, ID: "brand-x-001", Name: "Brand X"},
	}
}

// TestJournalRoundTrip proves a snapshot survives the file byte for byte
// in meaning: the documents, rows with their reasons, links, write ops
// and feedback markers, the pending question, the citations, timelines,
// counters and brand context.
func TestJournalRoundTrip(t *testing.T) {
	ctx := context.Background()
	journal, err := NewJournal(t.TempDir())
	if err != nil {
		t.Fatalf("new journal: %v", err)
	}
	want := testSnapshot()
	if err := journal.Save(ctx, "shopper_01", want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, found, err := journal.Load(ctx, "shopper_01")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !found {
		t.Fatal("load reports no snapshot after a save")
	}
	if !reflect.DeepEqual(got.Documents, want.Documents) {
		t.Errorf("documents differ:\nactual:   %+v\nwant: %+v", got.Documents, want.Documents)
	}
	if !reflect.DeepEqual(got.Rows, want.Rows) {
		t.Errorf("rows differ:\nactual:   %+v\nwant: %+v", got.Rows, want.Rows)
	}
	if !reflect.DeepEqual(got.Pending, want.Pending) {
		t.Errorf("pending differ:\nactual:   %+v\nwant: %+v", got.Pending, want.Pending)
	}
	if !reflect.DeepEqual(got.Links, want.Links) {
		t.Errorf("links differ:\nactual:   %+v\nwant: %+v", got.Links, want.Links)
	}
	if !reflect.DeepEqual(got.Timelines, want.Timelines) {
		t.Errorf("timelines differ:\nactual:   %+v\nwant: %+v", got.Timelines, want.Timelines)
	}
	if !reflect.DeepEqual(got.Seq, want.Seq) {
		t.Errorf("seq differs: %v, want %v", got.Seq, want.Seq)
	}
	if got.LastBrand != want.LastBrand {
		t.Errorf("last brand = %+v, want %+v", got.LastBrand, want.LastBrand)
	}
}

// TestJournalWritesBesideTheShopper proves the file is named per shopper
// under the directory, so it sits beside the corrections journal.
func TestJournalWritesBesideTheShopper(t *testing.T) {
	dir := t.TempDir()
	journal, err := NewJournal(dir)
	if err != nil {
		t.Fatalf("new journal: %v", err)
	}
	if err := journal.Save(context.Background(), "shopper_01", testSnapshot()); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "shopper_01.evidence.json")); err != nil {
		t.Fatalf("journal file is not beside the corrections journal: %v", err)
	}
}

// TestJournalAbsentIsNotAnError proves a shopper with no journal loads as
// absent, which is how a new shopper starts empty.
func TestJournalAbsentIsNotAnError(t *testing.T) {
	journal, err := NewJournal(t.TempDir())
	if err != nil {
		t.Fatalf("new journal: %v", err)
	}
	_, found, err := journal.Load(context.Background(), "new_shopper")
	if err != nil {
		t.Fatalf("load absent: %v", err)
	}
	if found {
		t.Fatal("load reports a snapshot that was never saved")
	}
}

// TestJournalBadShopperIsRefused proves a shopper id that cannot name a
// file is refused on both paths, so no caller escapes the directory.
func TestJournalBadShopperIsRefused(t *testing.T) {
	ctx := context.Background()
	journal, err := NewJournal(t.TempDir())
	if err != nil {
		t.Fatalf("new journal: %v", err)
	}
	for _, shopper := range []string{"", ".", "..", "../x", "a/b", "a\\b"} {
		if err := journal.Save(ctx, shopper, testSnapshot()); err == nil {
			t.Errorf("save accepts shopper %q", shopper)
		}
		if _, _, err := journal.Load(ctx, shopper); err == nil {
			t.Errorf("load accepts shopper %q", shopper)
		}
	}
}

// TestJournalCorruptFileIsRefused proves a file that does not decode, or
// that names a newer layout version, is refused rather than guessed at.
func TestJournalCorruptFileIsRefused(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	journal, err := NewJournal(dir)
	if err != nil {
		t.Fatalf("new journal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "shopper_01.evidence.json"), []byte("{nope"), 0o600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	if _, _, err := journal.Load(ctx, "shopper_01"); !errors.Is(err, ErrCorruptJournal) {
		t.Fatalf("load corrupt: %v, want %v", err, ErrCorruptJournal)
	}
	if err := os.WriteFile(filepath.Join(dir, "shopper_02.evidence.json"), []byte("{\"version\":99}\n"), 0o600); err != nil {
		t.Fatalf("write future file: %v", err)
	}
	if _, _, err := journal.Load(ctx, "shopper_02"); !errors.Is(err, ErrCorruptJournal) {
		t.Fatalf("load future version: %v, want %v", err, ErrCorruptJournal)
	}
}

// TestStoreRestoreDocuments proves a store takes its documents back under
// their own ids, keeps the index order, and mints the next id after them
// instead of reusing one.
func TestStoreRestoreDocuments(t *testing.T) {
	store, err := NewStore(stubContract{kind: "test-kind"})
	if err != nil {
		t.Fatal(err)
	}
	docs := []Document{
		{DocID: "doc-0001", Kind: KindReason, Text: "first", Seeded: true},
		{DocID: "doc-0002", Kind: KindReason, Text: "second", Seeded: true},
	}
	if err := store.RestoreDocuments(docs); err != nil {
		t.Fatalf("restore: %v", err)
	}
	kept := store.Documents()
	if len(kept) != 2 || kept[0].DocID != "doc-0001" || kept[1].Text != "second" {
		t.Fatalf("restored documents = %+v, want the two in index order", kept)
	}
	if err := store.RestoreDocuments(docs); err != nil {
		t.Fatalf("repeat restore: %v", err)
	}
	if len(store.Documents()) != 2 {
		t.Errorf("repeat restore doubled the documents")
	}
	id, err := store.IndexDoc(context.Background(), "third")
	if err != nil {
		t.Fatal(err)
	}
	if id != "doc-0003" {
		t.Errorf("next id = %q, want doc-0003 past the restored two", id)
	}
	if err := store.RestoreDocuments([]Document{{Text: "no id"}}); !errors.Is(err, ErrNoDocID) {
		t.Errorf("restore without an id = %v, want %v", err, ErrNoDocID)
	}
	if err := store.RestoreDocuments([]Document{{DocID: "doc-0004"}}); !errors.Is(err, ErrEmptyText) {
		t.Errorf("restore of empty text = %v, want %v", err, ErrEmptyText)
	}
}
