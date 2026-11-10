// Package seed builds deterministic seeded shopper histories for the demo
// and the comparison, and the hidden answers the comparison is graded
// against.
//
// Every row carries the seeded mark from the domain model, and event time
// is a relative span, never a calendar claim: Row.Day counts days since
// the shopper's session start, so promotion's time-extent gates can be
// checked honestly without dating anything.
//
// The demo shopper mirrors the proven seeding shape: a general statement,
// background purchases, three fit returns spread across the session
// extent, and a later non-arrival dispute that counts toward the seller,
// never toward sizing. The comparison shoppers are built to tempt
// wrong-cause errors: a broken-item refund, a non-arrival dispute, a
// changed mind, an unclear answer, and a shopper whose size genuinely
// changed.
//
// Histories and answers leave through separate paths: Write puts shopper
// histories under one directory and the hidden answers under another, so
// no comparison arm can read the answers.
package seed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// GoldenSeed is the fixed seed the committed fixtures under testdata/seed
// are generated from. Regenerating with it reproduces them byte for byte.
const GoldenSeed uint64 = 7

// ErrInvalidDir is returned when an output directory path is empty.
var ErrInvalidDir = errors.New("seed: invalid output directory")

// Row is one seeded history row. Day is days since the shopper's session
// start: a relative span, not a calendar date. SeedMark carries the
// domain's seeded mark on every row.
type Row struct {
	// Day is days since the shopper's session start.
	Day float64
	// Signal is which strength-table row this is: classified outcome,
	// fit feedback, kept past window, or general statement.
	Signal domain.Signal
	// Source names where the row arrived from.
	Source domain.Source
	// Outcome is what happened to the money, or nil for a bare statement.
	Outcome *domain.Outcome
	// Item is the purchased thing the row is about.
	Item domain.Item
	// Reason is why it happened, or nil when no reason is known yet.
	Reason *domain.Reason
	// Link ties the row to its transaction.
	Link domain.TransactionLink
	// Note is a short human description of the row.
	Note string
	// SeedMark carries the domain's seeded mark.
	SeedMark string
}

// SeededEvidence materializes the row as domain evidence, marked seeded,
// with event time measured from the given session-start anchor.
func (r Row) SeededEvidence(anchor time.Time) domain.Evidence {
	return domain.Evidence{
		Source:     r.Source,
		Outcome:    derefOutcome(r.Outcome),
		Item:       r.Item,
		Reason:     r.Reason,
		Link:       r.Link,
		Seeded:     true,
		OccurredAt: anchor.Add(time.Duration(r.Day * float64(24*time.Hour))),
	}
}

func derefOutcome(o *domain.Outcome) domain.Outcome {
	if o == nil {
		return domain.Outcome{}
	}
	return *o
}

// History is one shopper's seeded history. SessionDays is the session's
// time extent in days, measured from the same start Row.Day counts from.
type History struct {
	// ShopperID names the shopper; sessions isolate shoppers.
	ShopperID string
	// SessionDays is the session's time extent in days.
	SessionDays float64
	// Rows is the history in event-time order.
	Rows []Row
}

// Answer is the hidden right size for one comparison shopper. Answers
// travel only on the hidden path, never beside the histories.
type Answer struct {
	// ShopperID names the comparison shopper this answers.
	ShopperID string
	// RightSize is the size the comparison grades as right.
	RightSize string
	// Why states why that size is right: which evidence counts and which
	// tempting cause does not.
	Why string
}

// Bundle is everything the generator produces from one seed.
type Bundle struct {
	// Seed is the seed this bundle was generated from.
	Seed uint64
	// Histories holds the demo shopper first, then the comparison set.
	Histories []History
	// Answers holds one hidden answer per comparison shopper.
	Answers []Answer
}

// Generate builds the seeded bundle deterministically from seed: the same
// seed always yields byte-identical output, and different seeds differ in
// their transaction ids. Event-time spans are fixed constants, so the
// promotion gates hold for every seed.
func Generate(seed uint64) Bundle {
	rng := rand.New(rand.NewSource(int64(seed)))
	b := &builder{rng: rng, seed: seed}
	histories := []History{
		b.demoShopper(),
		b.brokenShopper(),
		b.neverArrivedShopper(),
		b.changedMindShopper(),
		b.sizeChangedShopper(),
		b.unclearShopper(),
		b.noisySmallShopper(),
		b.noisyLargeShopper(),
		b.brandsSteadyShopper(),
		b.brandsSplitShopper(),
	}
	return Bundle{
		Seed:      seed,
		Histories: histories,
		Answers: []Answer{
			{
				ShopperID: "cmp-broken",
				RightSize: "M",
				Why:       "The torn-seam refund is damage, not fit: it counts toward the seller and never toward sizing, so the kept size M stands.",
			},
			{
				ShopperID: "cmp-never-arrived",
				RightSize: "L",
				Why:       "The open dispute is a parcel that never arrived, not a fit signal: it counts toward the seller and never toward sizing, so the kept size L stands.",
			},
			{
				ShopperID: "cmp-changed-mind",
				RightSize: "M",
				Why:       "The changed-mind refund names no product fault and counts toward nothing, so the kept size M stands.",
			},
			{
				ShopperID: "cmp-size-changed",
				RightSize: "L",
				Why:       "The early fits-well note for M is stale: two newer classified too-small returns in M are structural evidence the size moved, so the right size is L.",
			},
			{
				ShopperID: "cmp-unclear",
				RightSize: "M",
				Why:       "The vague refund sits below the confidence floor and stays a pending question with no reason, so it counts toward nothing and the kept size M stands.",
			},
			{
				ShopperID: "cmp-noisy-small",
				RightSize: "L",
				Why:       "Three classified too-small returns in M accumulate to size up to L; the changed-mind, vague and unrecognised-charge rows count toward nothing, the torn-seam refund and the lost parcel count toward the seller, the kept rows are weak inference only, and the Brand J fits-well is another brand's cut.",
			},
			{
				ShopperID: "cmp-noisy-large",
				RightSize: "S",
				Why:       "Three classified too-large returns in M accumulate to size down to S; the kept colour, vague answer and cracked-button and loose-hem refunds count toward nothing or the seller, and the Brand L fits-well is another brand's cut.",
			},
			{
				ShopperID: "cmp-brands-steady",
				RightSize: "M",
				Why:       "Brand Q is true to size for this shopper: kept M plus fits-well M and no fit evidence against it. Brand P's two too-small returns are that brand's cut and never leak into Brand Q, so the right size stays M.",
			},
			{
				ShopperID: "cmp-brands-split",
				RightSize: "S",
				Why:       "Brand R runs large twice in M, so the right size for Brand R is S; Brand T's kept M and too-small M are that brand's opposite cut and never leak into Brand R.",
			},
		},
	}
}

// Write generates the bundle for seed and writes the shopper histories to
// historiesDir (one file per shopper) and the hidden answers to hiddenDir.
// The two directories must be distinct paths.
func Write(ctx context.Context, historiesDir, hiddenDir string, seed uint64) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("seed: write histories: %w", err)
	}
	if historiesDir == "" || hiddenDir == "" {
		return fmt.Errorf("seed: write histories: %w", ErrInvalidDir)
	}
	bundle := Generate(seed)
	encoded, answers, err := marshalBundle(bundle)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(historiesDir, 0o755); err != nil {
		return fmt.Errorf("seed: write histories: %w", err)
	}
	if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
		return fmt.Errorf("seed: write hidden answers: %w", err)
	}
	for _, h := range bundle.Histories {
		path := filepath.Join(historiesDir, h.ShopperID+".json")
		if err := os.WriteFile(path, encoded[h.ShopperID], 0o644); err != nil {
			return fmt.Errorf("seed: write histories: %w", err)
		}
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "answers.json"), answers, 0o644); err != nil {
		return fmt.Errorf("seed: write hidden answers: %w", err)
	}
	return nil
}

// marshalBundle renders every history and the answers file. Struct field
// order makes the encoding deterministic, so the same bundle always
// yields the same bytes.
func marshalBundle(bundle Bundle) (map[string][]byte, []byte, error) {
	encoded := make(map[string][]byte, len(bundle.Histories))
	for _, h := range bundle.Histories {
		raw, err := json.MarshalIndent(h, "", "  ")
		if err != nil {
			return nil, nil, fmt.Errorf("seed: encode history %q: %w", h.ShopperID, err)
		}
		encoded[h.ShopperID] = append(raw, '\n')
	}
	raw, err := json.MarshalIndent(bundle.Answers, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("seed: encode hidden answers: %w", err)
	}
	return encoded, append(raw, '\n'), nil
}

// builder carries the deterministic transaction-id stream for one bundle.
type builder struct {
	rng  *rand.Rand
	seed uint64
	seq  int
}

// link mints the transaction ids for one row of the given outcome kind.
// The custom id propagates across the order, capture, refund and dispute
// records, so it is the join key; the event id keys webhook idempotency.
// Only the ids the outcome kind actually has are filled: a kept purchase
// carries no refund or dispute id.
func (b *builder) link(shopper string, kind domain.OutcomeKind) domain.TransactionLink {
	b.seq++
	token := fmt.Sprintf("%04x", b.rng.Uint32()&0xffff)
	tag := fmt.Sprintf("seed-%d-%s-%03d-%s", b.seed, shopper, b.seq, token)
	link := domain.TransactionLink{
		OrderID:   "ORD-" + tag,
		CaptureID: "CAP-" + tag,
		CustomID:  fmt.Sprintf("seed-%d-%s-%03d", b.seed, shopper, b.seq),
		EventID:   "EVT-" + tag,
	}
	switch kind {
	case domain.OutcomeRefund:
		link.RefundID = "RFD-" + tag
	case domain.OutcomeDispute:
		link.DisputeID = "DSP-" + tag
	}
	return link
}

// statement records a general shopper statement: an assertion that never
// promotes, whatever the ledger later shows beside it.
func (b *builder) statement(shopper string, day float64, item domain.Item, note string) Row {
	return Row{
		Day:      day,
		Signal:   domain.SignalGeneralStatement,
		Source:   domain.SourceShopper,
		Outcome:  nil,
		Item:     item,
		Reason:   nil,
		Link:     domain.TransactionLink{},
		Note:     note,
		SeedMark: domain.SeedMark,
	}
}

// kept records a purchase kept past the return window with no feedback: a
// weak inferred observation, never satisfaction and never promotion.
func (b *builder) kept(shopper string, day float64, item domain.Item, amountMinor int64, note string) Row {
	return Row{
		Day:    day,
		Signal: domain.SignalKeptPastWindow,
		Source: domain.SourcePayment,
		Outcome: &domain.Outcome{
			Kind:        domain.OutcomeKept,
			Status:      "PAST_WINDOW",
			AmountMinor: amountMinor,
			Currency:    "USD",
		},
		Item:     item,
		Reason:   nil,
		Link:     b.link(shopper, domain.OutcomeKept),
		Note:     note,
		SeedMark: domain.SeedMark,
	}
}

// refund records a refunded capture with the shopper's classified answer.
// The reason decides what the row counts toward: a refund without one
// would count toward nothing.
func (b *builder) refund(shopper string, day float64, item domain.Item, amountMinor int64, text string, category domain.Category, confidence float64, note string) Row {
	return Row{
		Day:    day,
		Signal: domain.SignalClassifiedOutcome,
		Source: domain.SourcePayment,
		Outcome: &domain.Outcome{
			Kind:        domain.OutcomeRefund,
			Status:      "COMPLETED",
			AmountMinor: amountMinor,
			Currency:    "USD",
		},
		Item: item,
		Reason: &domain.Reason{
			Category:   category,
			Text:       text,
			Confidence: confidence,
		},
		Link:     b.link(shopper, domain.OutcomeRefund),
		Note:     note,
		SeedMark: domain.SeedMark,
	}
}

// dispute records a dispute filed against a capture. The dispute carries
// the evidence source's reason code as its text.
func (b *builder) dispute(shopper string, day float64, item domain.Item, amountMinor int64, code domain.DisputeCode, category domain.Category, note string) Row {
	return Row{
		Day:    day,
		Signal: domain.SignalClassifiedOutcome,
		Source: domain.SourcePayment,
		Outcome: &domain.Outcome{
			Kind:        domain.OutcomeDispute,
			Status:      "OPEN",
			AmountMinor: amountMinor,
			Currency:    "USD",
		},
		Item: item,
		Reason: &domain.Reason{
			Category:   category,
			Text:       string(code),
			Confidence: 1,
		},
		Link:     b.link(shopper, domain.OutcomeDispute),
		Note:     note,
		SeedMark: domain.SeedMark,
	}
}

// feedback records explicit fit feedback tied to a specific purchase: a
// structural write linked to the transaction.
func (b *builder) feedback(shopper string, day float64, item domain.Item, amountMinor int64, note string) Row {
	return Row{
		Day:    day,
		Signal: domain.SignalFitFeedback,
		Source: domain.SourceShopper,
		Outcome: &domain.Outcome{
			Kind:        domain.OutcomeCapture,
			Status:      "COMPLETED",
			AmountMinor: amountMinor,
			Currency:    "USD",
		},
		Item:     item,
		Reason:   nil,
		Link:     b.link(shopper, domain.OutcomeCapture),
		Note:     note,
		SeedMark: domain.SeedMark,
	}
}

// demoShopper is the demo story: three fit returns spread across the
// session extent so the promotion gates are met honestly, realistic
// background purchases, and a later never-arrived dispute that counts
// toward the seller, never toward sizing.
func (b *builder) demoShopper() History {
	const shopper = "demo-shopper"
	brandX := func(sku, size string) domain.Item {
		return domain.Item{
			BrandID:   "BRX",
			BrandName: "Brand X",
			LineName:  "Button Down Oxford Shirt",
			SKU:       sku,
			Size:      size,
			Category:  "dress-shirts",
		}
	}
	return History{
		ShopperID:   shopper,
		SessionDays: 11,
		Rows: []Row{
			b.statement(shopper, 0, brandX("BRX-OXF-WHT", "M"), "Shopper says they are usually size M"),
			b.kept(shopper, 1, domain.Item{BrandID: "BRY", BrandName: "Brand Y", LineName: "Everyday Tee", SKU: "BRY-TEE-GRY", Size: "M", Category: "t-shirts"}, 2500, "Kept past the window: Brand Y tee, no feedback"),
			b.kept(shopper, 2, domain.Item{BrandID: "BRZ", BrandName: "Brand Z", LineName: "Canvas Cap", SKU: "BRZ-CAP-NVY", Size: "OS", Category: "hats"}, 3000, "Kept past the window: Brand Z cap, no feedback"),
			b.refund(shopper, 3, brandX("BRX-OXF-WHT", "M"), 19500, "too tight in the shoulders", domain.CategoryFitTooSmall, 1, "Fit return 1: Brand X oxford size M refunded; too tight in the shoulders"),
			b.refund(shopper, 6, brandX("BRX-OXF-BLU", "M"), 19500, "too tight across the chest", domain.CategoryFitTooSmall, 1, "Fit return 2: Brand X oxford size M refunded; too tight across the chest"),
			b.refund(shopper, 9, brandX("BRX-OXF-STR", "M"), 19500, "sleeves too short in M", domain.CategoryFitTooSmall, 1, "Fit return 3: Brand X oxford size M refunded; sleeves too short in M"),
			b.dispute(shopper, 11, brandX("BRX-OXF-WHT", "M"), 19500, domain.DisputeMerchandiseNotReceived, domain.CategoryNotArrived, "Dispute opened: Brand X parcel never arrived; carrier lost it"),
		},
	}
}

// brokenShopper tempts the wrong cause with a refund for a broken item:
// damage counts toward the seller, never toward sizing.
func (b *builder) brokenShopper() History {
	const shopper = "cmp-broken"
	item := func(size string) domain.Item {
		return domain.Item{BrandID: "BRC", BrandName: "Brand C", LineName: "Everyday Oxford Shirt", SKU: "BRC-OXF-" + size, Size: size, Category: "dress-shirts"}
	}
	return History{
		ShopperID:   shopper,
		SessionDays: 6,
		Rows: []Row{
			b.statement(shopper, 0, item("M"), "Shopper says they are usually size M"),
			b.kept(shopper, 1, item("M"), 4800, "Kept past the window: Brand C oxford size M, no feedback"),
			b.refund(shopper, 4, item("M"), 4800, "arrived with a torn seam", domain.CategoryDamagedOrDefective, 1, "Refund: Brand C oxford size M; arrived with a torn seam"),
		},
	}
}

// neverArrivedShopper tempts the wrong cause with a non-arrival dispute:
// it counts toward the seller, never toward sizing.
func (b *builder) neverArrivedShopper() History {
	const shopper = "cmp-never-arrived"
	item := func(size string) domain.Item {
		return domain.Item{BrandID: "BRD", BrandName: "Brand D", LineName: "Washed Oxford Shirt", SKU: "BRD-OXF-" + size, Size: size, Category: "dress-shirts"}
	}
	return History{
		ShopperID:   shopper,
		SessionDays: 8,
		Rows: []Row{
			b.statement(shopper, 0, item("L"), "Shopper says they are usually size L"),
			b.kept(shopper, 2, item("L"), 5200, "Kept past the window: Brand D oxford size L, no feedback"),
			b.dispute(shopper, 6, item("L"), 5200, domain.DisputeMerchandiseNotReceived, domain.CategoryNotArrived, "Dispute opened: Brand D parcel never arrived; tracking stuck for weeks"),
		},
	}
}

// changedMindShopper tempts the wrong cause with a no-fault return: a
// changed mind counts toward nothing.
func (b *builder) changedMindShopper() History {
	const shopper = "cmp-changed-mind"
	item := func(size string) domain.Item {
		return domain.Item{BrandID: "BRE", BrandName: "Brand E", LineName: "Classic Oxford Shirt", SKU: "BRE-OXF-" + size, Size: size, Category: "dress-shirts"}
	}
	return History{
		ShopperID:   shopper,
		SessionDays: 7,
		Rows: []Row{
			b.statement(shopper, 0, item("M"), "Shopper says they are usually size M"),
			b.kept(shopper, 1, item("M"), 3900, "Kept past the window: Brand E oxford size M, no feedback"),
			b.refund(shopper, 5, item("M"), 3900, "changed my mind", domain.CategoryChangedMind, 1, "Refund: Brand E oxford size M; shopper changed their mind"),
		},
	}
}

// sizeChangedShopper is the shopper whose size genuinely changed: early
// feedback says M fits, then two newer classified too-small returns in M
// move the answer to L.
func (b *builder) sizeChangedShopper() History {
	const shopper = "cmp-size-changed"
	item := func(size string) domain.Item {
		return domain.Item{BrandID: "BRF", BrandName: "Brand F", LineName: "Heritage Oxford Shirt", SKU: "BRF-OXF-" + size, Size: size, Category: "dress-shirts"}
	}
	return History{
		ShopperID:   shopper,
		SessionDays: 9,
		Rows: []Row{
			b.statement(shopper, 0, item("M"), "Shopper says they are usually size M"),
			b.feedback(shopper, 1, item("M"), 4400, "Fit feedback: Brand F oxford size M fits well, tied to this purchase"),
			b.refund(shopper, 5, item("M"), 4400, "too tight in the shoulders", domain.CategoryFitTooSmall, 1, "Fit return 1: Brand F oxford size M refunded; too tight in the shoulders"),
			b.refund(shopper, 7, item("M"), 4400, "too tight across the chest", domain.CategoryFitTooSmall, 1, "Fit return 2: Brand F oxford size M refunded; too tight across the chest"),
		},
	}
}

// unclearShopper tempts the wrong cause with a vague answer: below the
// confidence floor it stays a pending question with no reason and counts
// toward nothing.
func (b *builder) unclearShopper() History {
	const shopper = "cmp-unclear"
	item := func(size string) domain.Item {
		return domain.Item{BrandID: "BRG", BrandName: "Brand G", LineName: "Slim Oxford Shirt", SKU: "BRG-OXF-" + size, Size: size, Category: "dress-shirts"}
	}
	return History{
		ShopperID:   shopper,
		SessionDays: 7,
		Rows: []Row{
			b.statement(shopper, 0, item("M"), "Shopper says they are usually size M"),
			b.kept(shopper, 1, item("M"), 3600, "Kept past the window: Brand G oxford size M, no feedback"),
			b.refund(shopper, 5, item("M"), 3600, "not great", domain.CategoryUnclear, 0.94, "Refund: Brand G oxford size M; shopper answer too vague to classify, ask again"),
		},
	}
}

// noisySmallShopper is a long noisy wardrobe: thirteen rows where three
// classified too-small returns in M accumulate to L while damage, a lost
// parcel, a changed mind, a vague answer and an unrecognised charge tempt
// the wrong cause and kept rows plus another brand's fits-well add clutter.
// A perfect reasoner sorts every row by its reason category and sizes up;
// raw context alone drowns the three fit rows in the other ten.
func (b *builder) noisySmallShopper() History {
	const shopper = "cmp-noisy-small"
	oxford := func(size string) domain.Item {
		return domain.Item{BrandID: "BRH", BrandName: "Brand H", LineName: "Harbor Oxford Shirt", SKU: "BRH-OXF-" + size, Size: size, Category: "dress-shirts"}
	}
	tee := domain.Item{BrandID: "BRH", BrandName: "Brand H", LineName: "Harbor Tee", SKU: "BRH-TEE-M", Size: "M", Category: "t-shirts"}
	otherTee := domain.Item{BrandID: "BRJ", BrandName: "Brand J", LineName: "Jetty Tee", SKU: "BRJ-TEE-M", Size: "M", Category: "t-shirts"}
	beanie := domain.Item{BrandID: "BRK", BrandName: "Brand K", LineName: "Kelpie Beanie", SKU: "BRK-BEA-OS", Size: "OS", Category: "hats"}
	return History{
		ShopperID:   shopper,
		SessionDays: 30,
		Rows: []Row{
			b.statement(shopper, 0, oxford("M"), "Shopper says they are usually size M"),
			b.kept(shopper, 1, otherTee, 2600, "Kept past the window: Brand J jetty tee size M, no feedback"),
			b.kept(shopper, 2, beanie, 2800, "Kept past the window: Brand K kelpie beanie, no feedback"),
			b.refund(shopper, 4, oxford("M"), 4900, "changed my mind about the colour", domain.CategoryChangedMind, 1, "Refund: Brand H oxford size M; shopper changed their mind about the colour"),
			b.refund(shopper, 6, oxford("M"), 4900, "too tight in the shoulders", domain.CategoryFitTooSmall, 1, "Fit return 1: Brand H oxford size M refunded; too tight in the shoulders"),
			b.dispute(shopper, 8, oxford("M"), 4900, domain.DisputeMerchandiseNotReceived, domain.CategoryNotArrived, "Dispute opened: Brand H parcel never arrived; tracking stuck for weeks"),
			b.refund(shopper, 10, oxford("M"), 4900, "arrived with a torn seam", domain.CategoryDamagedOrDefective, 1, "Refund: Brand H oxford size M; arrived with a torn seam"),
			b.refund(shopper, 12, oxford("M"), 4900, "not great", domain.CategoryUnclear, 0.94, "Refund: Brand H oxford size M; shopper answer too vague to classify, ask again"),
			b.kept(shopper, 14, tee, 2500, "Kept past the window: Brand H harbor tee size M, no feedback"),
			b.refund(shopper, 16, oxford("M"), 4900, "too tight across the chest", domain.CategoryFitTooSmall, 1, "Fit return 2: Brand H oxford size M refunded; too tight across the chest"),
			b.feedback(shopper, 18, otherTee, 2600, "Fit feedback: Brand J jetty tee size M fits well, tied to this purchase"),
			b.dispute(shopper, 20, oxford("M"), 4900, domain.DisputeUnauthorizedTransaction, domain.CategoryNone, "Dispute opened: Brand H charge the shopper does not recognise; account-takeover review"),
			b.refund(shopper, 24, oxford("M"), 4900, "sleeves too short in M", domain.CategoryFitTooSmall, 1, "Fit return 3: Brand H oxford size M refunded; sleeves too short in M"),
		},
	}
}

// noisyLargeShopper mirrors the small noisy wardrobe in the other
// direction: three classified too-large returns in M accumulate to S amid
// seller-fault refunds, a lost parcel, a kept colour, a vague answer and
// another brand's fits-well. Same phenomenon, opposite size move, so a
// hardened set that only ever sizes up cannot pass it.
func (b *builder) noisyLargeShopper() History {
	const shopper = "cmp-noisy-large"
	oxford := func(size string) domain.Item {
		return domain.Item{BrandID: "BRI", BrandName: "Brand I", LineName: "Inlet Oxford Shirt", SKU: "BRI-OXF-" + size, Size: size, Category: "dress-shirts"}
	}
	tee := domain.Item{BrandID: "BRI", BrandName: "Brand I", LineName: "Inlet Tee", SKU: "BRI-TEE-M", Size: "M", Category: "t-shirts"}
	otherTee := domain.Item{BrandID: "BRL", BrandName: "Brand L", LineName: "Lagoon Tee", SKU: "BRL-TEE-M", Size: "M", Category: "t-shirts"}
	cap := domain.Item{BrandID: "BRM", BrandName: "Brand M", LineName: "Mooring Cap", SKU: "BRM-CAP-OS", Size: "OS", Category: "hats"}
	return History{
		ShopperID:   shopper,
		SessionDays: 28,
		Rows: []Row{
			b.statement(shopper, 0, oxford("M"), "Shopper says they are usually size M"),
			b.kept(shopper, 1, otherTee, 2700, "Kept past the window: Brand L lagoon tee size M, no feedback"),
			b.kept(shopper, 3, cap, 2900, "Kept past the window: Brand M mooring cap, no feedback"),
			b.refund(shopper, 5, oxford("M"), 5100, "swims on the shopper", domain.CategoryFitTooLarge, 1, "Fit return 1: Brand I oxford size M refunded; swims on the shopper"),
			b.refund(shopper, 7, oxford("M"), 5100, "buttons arrived cracked", domain.CategoryDamagedOrDefective, 1, "Refund: Brand I oxford size M; buttons arrived cracked"),
			b.dispute(shopper, 9, oxford("M"), 5100, domain.DisputeMerchandiseNotReceived, domain.CategoryNotArrived, "Dispute opened: Brand I parcel never arrived; carrier lost it"),
			b.refund(shopper, 11, oxford("M"), 5100, "too loose across the chest", domain.CategoryFitTooLarge, 1, "Fit return 2: Brand I oxford size M refunded; too loose across the chest"),
			b.kept(shopper, 13, tee, 2400, "Kept past the window: Brand I inlet tee size M, no feedback"),
			b.refund(shopper, 15, oxford("M"), 5100, "ordered two colours, kept the other", domain.CategoryChangedMind, 1, "Refund: Brand I oxford size M; ordered two colours, kept the other"),
			b.feedback(shopper, 17, otherTee, 2700, "Fit feedback: Brand L lagoon tee size M fits well, tied to this purchase"),
			b.refund(shopper, 19, oxford("M"), 5100, "did not love it", domain.CategoryUnclear, 0.94, "Refund: Brand I oxford size M; shopper answer too vague to classify, ask again"),
			b.refund(shopper, 21, oxford("M"), 5100, "arrived with a loose hem", domain.CategoryDamagedOrDefective, 1, "Refund: Brand I oxford size M; arrived with a loose hem"),
			b.refund(shopper, 23, oxford("M"), 5100, "sleeves too long in M", domain.CategoryFitTooLarge, 1, "Fit return 3: Brand I oxford size M refunded; sleeves too long in M"),
		},
	}
}

// brandsSteadyShopper is a two-brand wardrobe with distinct cuts: Brand P
// runs small twice in M while the candidate Brand Q holds a kept M and a
// fits-well M with nothing against it. The tempting error is sizing up in
// Brand Q on Brand P's evidence; a perfect reasoner keeps the beliefs
// per brand and stays at M for Brand Q.
func (b *builder) brandsSteadyShopper() History {
	const shopper = "cmp-brands-steady"
	narrow := func(size string) domain.Item {
		return domain.Item{BrandID: "BRP", BrandName: "Brand P", LineName: "Pier Oxford Shirt", SKU: "BRP-OXF-" + size, Size: size, Category: "dress-shirts"}
	}
	regular := func(size string) domain.Item {
		return domain.Item{BrandID: "BRQ", BrandName: "Brand Q", LineName: "Quay Oxford Shirt", SKU: "BRQ-OXF-" + size, Size: size, Category: "dress-shirts"}
	}
	tee := domain.Item{BrandID: "BRQ", BrandName: "Brand Q", LineName: "Quay Tee", SKU: "BRQ-TEE-M", Size: "M", Category: "t-shirts"}
	return History{
		ShopperID:   shopper,
		SessionDays: 9,
		Rows: []Row{
			b.statement(shopper, 0, regular("M"), "Shopper says they are usually size M"),
			b.kept(shopper, 1, regular("M"), 4600, "Kept past the window: Brand Q quay oxford size M, no feedback"),
			b.refund(shopper, 3, narrow("M"), 4500, "too tight in the shoulders", domain.CategoryFitTooSmall, 1, "Fit return 1: Brand P pier oxford size M refunded; too tight in the shoulders"),
			b.feedback(shopper, 5, regular("M"), 4600, "Fit feedback: Brand Q quay oxford size M fits well, tied to this purchase"),
			b.refund(shopper, 6, narrow("M"), 4500, "too tight across the chest", domain.CategoryFitTooSmall, 1, "Fit return 2: Brand P pier oxford size M refunded; too tight across the chest"),
			b.kept(shopper, 8, tee, 2300, "Kept past the window: Brand Q quay tee size M, no feedback"),
			b.dispute(shopper, 9, regular("M"), 4600, domain.DisputeMerchandiseNotReceived, domain.CategoryNotArrived, "Dispute opened: Brand Q parcel never arrived; tracking stuck for weeks"),
		},
	}
}

// brandsSplitShopper is a two-brand wardrobe with opposite cuts: the
// candidate Brand R runs large twice in M while Brand T runs small once in
// M beside a kept M. The tempting errors are holding M on Brand T's
// evidence or sizing up on it; a perfect reasoner follows Brand R's own
// evidence down to S.
func (b *builder) brandsSplitShopper() History {
	const shopper = "cmp-brands-split"
	relaxed := func(size string) domain.Item {
		return domain.Item{BrandID: "BRR", BrandName: "Brand R", LineName: "Reef Oxford Shirt", SKU: "BRR-OXF-" + size, Size: size, Category: "dress-shirts"}
	}
	slim := func(size string) domain.Item {
		return domain.Item{BrandID: "BRT", BrandName: "Brand T", LineName: "Tide Oxford Shirt", SKU: "BRT-OXF-" + size, Size: size, Category: "dress-shirts"}
	}
	return History{
		ShopperID:   shopper,
		SessionDays: 9,
		Rows: []Row{
			b.statement(shopper, 0, relaxed("M"), "Shopper says they are usually size M"),
			b.refund(shopper, 2, relaxed("M"), 4700, "too loose across the chest", domain.CategoryFitTooLarge, 1, "Fit return 1: Brand R reef oxford size M refunded; too loose across the chest"),
			b.kept(shopper, 4, slim("M"), 4200, "Kept past the window: Brand T tide oxford size M, no feedback"),
			b.refund(shopper, 5, slim("M"), 4200, "too tight in the shoulders", domain.CategoryFitTooSmall, 1, "Fit return 1: Brand T tide oxford size M refunded; too tight in the shoulders"),
			b.refund(shopper, 7, relaxed("M"), 4700, "sleeves too long in M", domain.CategoryFitTooLarge, 1, "Fit return 2: Brand R reef oxford size M refunded; sleeves too long in M"),
			b.dispute(shopper, 9, relaxed("M"), 4700, domain.DisputeMerchandiseNotReceived, domain.CategoryNotArrived, "Dispute opened: Brand R parcel never arrived; carrier lost it"),
		},
	}
}
