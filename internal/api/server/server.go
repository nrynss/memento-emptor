package server

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/fitindex"
	"github.com/nrynss/memento-emptor/internal/identity"
	"github.com/nrynss/memento-emptor/internal/ingest"
	"github.com/nrynss/memento-emptor/internal/memory"
	"github.com/nrynss/memento-emptor/internal/pool"
	"github.com/nrynss/memento-emptor/internal/standing"
	"github.com/nrynss/memento-emptor/seed"
)

// The service's sentinel conditions. Callers branch on these with errors.Is
// and never on message text.
var (
	// ErrUnknownBelief answers a belief id the shopper has no record of.
	ErrUnknownBelief = errors.New("server: unknown belief")
	// ErrUnknownQuestion answers a question id with no open pending question.
	ErrUnknownQuestion = errors.New("server: unknown question")
	// ErrQuestionClosed is reserved for a record_reason that re-answers a
	// question already answered. No served path returns it today:
	// record_reason answers only a question in the shopper's pending set,
	// and a clear answer removes the question from that set, so a second
	// answer is ErrUnknownQuestion.
	ErrQuestionClosed = errors.New("server: question is already answered")
	// ErrUnknownPurchase answers feedback for a transaction the layer
	// never saw. Feedback joins on the capture it concerns, never floats.
	ErrUnknownPurchase = errors.New("server: unknown purchase")
	// ErrFeedbackMismatch answers feedback whose variant does not match
	// the purchase it names. Feedback is tied to a specific purchase.
	ErrFeedbackMismatch = errors.New("server: feedback does not match the purchase")
	// ErrFeedbackConflict answers a repeat delivery whose words read as a
	// different verdict than the purchase already holds. The purchase
	// keeps its recorded verdict and nothing is written.
	ErrFeedbackConflict = errors.New("server: feedback conflicts with the recorded verdict")
	// ErrEmptyCorrection answers a correction with no shopper words.
	ErrEmptyCorrection = errors.New("server: correction has no text")
	// ErrMissingShopper answers a call with no shopper id. Every call is
	// scoped to one shopper, so an unscoped call answers nothing.
	ErrMissingShopper = errors.New("server: missing shopper id")
	// ErrWebhookShopper answers a capture or refund delivery whose
	// resource names no shopper. The text is the wire message the route
	// already answered, so the capture and refund behaviour is unchanged.
	ErrWebhookShopper = errors.New("delivery names no shopper")
	// ErrDisputeWithoutShopper answers a dispute delivery whose resource
	// and disputed transactions name no shopper. It wraps
	// ErrWebhookShopper, so a caller can treat every missing shopper
	// alike, and its text says which shape was missing one.
	ErrDisputeWithoutShopper = fmt.Errorf("%w: dispute carries no custom", ErrWebhookShopper)
	// ErrPoolUnwired answers a pooled read or write while the pool is not
	// wired: no index, no identity salt or no line resolver. The layered
	// half answers nothing rather than guess a line or a hash.
	ErrPoolUnwired = errors.New("server: pool is not wired")
)

// graphAdapter implements the memory seam over the evidence-graph seam,
// so the writer and reader code against one small interface while the
// service holds the seam the real graph client implements. The live swap
// replaces the fake behind fakes.EvidenceGraph and this adapter stands.
type graphAdapter struct {
	inner fakes.EvidenceGraph
}

// Derive records one belief or statement.
func (a graphAdapter) Derive(ctx context.Context, content, kind string, eventTime time.Time) error {
	return a.inner.Derive(ctx, content, kind, eventTime)
}

// RecordAction records one structural write.
func (a graphAdapter) RecordAction(ctx context.Context, action string, dependsOn, against []string, eventTime time.Time) error {
	return a.inner.RecordAction(ctx, action, dependsOn, against, eventTime)
}

// Inspect reports the promotion state of one belief.
func (a graphAdapter) Inspect(ctx context.Context, content string) (memory.Insight, error) {
	report, err := a.inner.Inspect(ctx, content)
	if err != nil {
		return memory.Insight{}, err
	}
	return memory.Insight{
		State:                report.State,
		Contradicted:         report.Contradicted,
		Dependents:           report.Dependents,
		DistinctInteractions: report.DistinctInteractions,
		Coverage:             report.Coverage,
		Warning:              report.Warning,
	}, nil
}

// Deps wires the service to its pieces. The payment source, classifier,
// store and graph arrive behind the fakes' seams: tests pass the recording
// fakes and the live swap passes the real clients with no caller change.
// The SchemaDir holds the frozen request schemas served on tools/list; the
// service fails closed at construction when one is missing. Clock stamps
// live writes; tests pin it for deterministic times.
//
// The pivot's second half is optional: Pool, FitRows, IdentitySalt and
// Lines wire the pooled reads and writes, and StandingDir persists the
// per-shopper journals. With them absent the service keeps the pre-pool
// shape — no pool fields, no fit rows, journals in memory — so the frozen
// base fixtures still describe it exactly. A verifier replaces the
// delivery gate on the webhook route; tests inject a fake so the suite
// stays offline.
type Deps struct {
	// Payments replays the webhook deliveries and transaction reads.
	Payments fakes.PaymentSource
	// Classifier replays the reason classifications.
	Classifier fakes.ReasonClassifier
	// Store holds the raw evidence texts behind the service's doc ids.
	Store fakes.EvidenceStore
	// Graph applies the promotion bars and contradiction flags.
	Graph fakes.EvidenceGraph
	// SchemaDir holds the frozen testdata/api request schemas.
	SchemaDir string
	// Clock stamps live writes. A nil clock reads the current UTC time.
	Clock func() time.Time
	// Pool is the fit index the pool reader aggregates. Nil leaves pool
	// reads off: recall omits the pool fields and product_fit refuses.
	Pool pool.Index
	// FitRows is the fit index the per-shopper writers update. Nil leaves
	// fit writes off.
	FitRows fitindex.Index
	// IdentitySalt is the keyring salt (64 hex characters) the writers
	// hash with. Empty leaves fit writes off.
	IdentitySalt string
	// Lines resolves an order item to its product line. Nil leaves fit
	// writes off: without the catalog the layer cannot key a row.
	Lines fitindex.Lines
	// StandingDir persists per-shopper journals under one file per
	// shopper: the corrections journal and, beside it, the evidence
	// journal. Empty keeps both in memory: a restart then loses the raw
	// evidence and the ability to declare a standing correction against
	// a later belief.
	StandingDir string
	// Verifier checks raw webhook deliveries on the deployed route. Nil
	// keeps the payment source's default gate.
	Verifier ingest.Verifier
}

// CallEntry is one logged interface call: the tool, the shopper, and the
// host trace when the caller passed one. Ordered by sequence, the log
// shows that a recommendation followed a recall under the same trace.
type CallEntry struct {
	// Seq orders the log. The first call is 1.
	Seq int `json:"seq"`
	// At is when the call arrived, RFC 3339.
	At string `json:"at"`
	// Tool names the interface call, including webhook ingest.
	Tool string `json:"tool"`
	// ShopperID scopes the call to one shopper.
	ShopperID string `json:"shopper_id"`
	// TraceID carries the host trace when the caller passed one.
	TraceID *string `json:"trace_id"`
}

// evidenceRecord is one stored evidence row: the domain row, its wire id,
// how it was written, the store doc behind its graph stub, and the
// beliefs it counts toward. A row with no belief counts toward nothing.
// Counts is stored, not derived: feedback rows carry their verdict's kind.
// A fit feedback row carries no reason category; a seller feedback row
// carries the seller category its verdict names.
type evidenceRecord struct {
	id        string
	row       domain.Evidence
	counts    domain.BeliefKind
	writeOp   domain.WriteOp
	docID     string
	beliefIDs []string
	// feedback names the feedback this row was recorded as, or is nil
	// while the row carries none. It carries the purchase's feedback
	// identity, so a repeat delivery for the same purchase is told apart
	// from that purchase's first feedback without reading the words.
	feedback *recordedFeedback
}

// recordedFeedback is one purchase's recorded feedback: the id it was
// recorded under, the verdict that recorded it, and the belief it counts
// toward, empty when it counts toward nothing. A repeat delivery answers
// with these rather than writing again, so the purchase's evidence stays
// one row and its belief's status cannot move. A seeded row whose history
// names no id carries an empty id.
type recordedFeedback struct {
	id       string
	verdict  contract.FeedbackVerdict
	beliefID string
}

// beliefRecord is one stored belief: its identity, its graph content, the
// fit direction it points (none when the belief states none), and when it
// last changed. Status, standing and warnings always come from the graph
// at read time, never from stored copies.
type beliefRecord struct {
	id        string
	kind      domain.BeliefKind
	subject   domain.Subject
	value     string
	direction domain.FitDirection
	seeded    bool
	createdAt time.Time
	updatedAt time.Time
	content   string
	evIDs     []string
}

// pendingRecord is one refund still missing its reason: the question the
// agent must ask, and the answers recorded so far.
type pendingRecord struct {
	id            string
	eventID       string
	outcome       domain.Outcome
	items         []domain.Item
	link          domain.TransactionLink
	seeded        bool
	askedCount    int
	lastAskedAt   *time.Time
	awaitingSince time.Time
}

// correctionRecord is one shopper correction in arrival order.
type correctionRecord struct {
	view contract.CorrectionView
	at   time.Time
}

// shopperState partitions every row by shopper. One shopper's beliefs
// never leak into another's recall. The journal holds the standing
// corrections a later belief may declare against, and the writer holds
// the shopper's fit rows; both are per-shopper because the per-line hash
// is salted over this shopper. The service lock serialises every access:
// two agents speaking for one shopper are ordered by it. Pooling is
// atomic because the writer's consent check reads it from the writer's
// own lock, which must never wait on the service lock.
type shopperState struct {
	ingest      *ingest.Service
	beliefs     map[string]*beliefRecord
	evidence    map[string]*evidenceRecord
	pending     map[string]*pendingRecord
	corrections map[string]*correctionRecord
	corrOrder   []string
	timelines   map[string][]contract.TimelineEvent
	notices     map[string]string
	captures    map[string][]string
	webhooks    map[string]bool
	seq         map[string]int
	streaks     map[streakKey]*domain.OppositeStreak
	journal     *memory.StandingCorrections
	writer      *fitindex.Writer
	pooling     atomic.Bool
	lastBrand   domain.Subject
	// restoredLinks and restoredTimelines hold the evidence journal's
	// belief-facing state until the boot seed has rebuilt the beliefs it
	// belongs to; applyRestoredEvidence merges them then.
	restoredLinks     []evidence.BeliefEvidence
	restoredTimelines []evidence.BeliefTimeline
	// pairing links each declared correction view to the journal entry
	// it was declared against, by the entry's words, subject, line and
	// size. The belief id is not the link: a restart mints fresh ids, but
	// the words and the subject survive, so the rewritten belief that
	// re-declares the entry re-links exactly its own views.
	pairing map[string]standing.ViewLink
}

// streakKey scopes one opposite-direction run to the beliefs it could
// rival: one belief kind on one subject. The shopper partition owns the
// map, so the key carries only kind and subject. The streaks are runtime
// state, not beliefs: they decide when an opposite-direction answer
// births a rival, and nothing about them reaches the graph.
type streakKey struct {
	kind      domain.BeliefKind
	subjectID string
}

// streakFor returns the opposite-direction run for one (kind, subject),
// creating it on the first mismatch. The caller holds the service lock.
func (st *shopperState) streakFor(kind domain.BeliefKind, subjectID string) *domain.OppositeStreak {
	key := streakKey{kind: kind, subjectID: subjectID}
	streak, ok := st.streaks[key]
	if !ok {
		streak = &domain.OppositeStreak{}
		st.streaks[key] = streak
	}
	return streak
}

// Service is the running memory layer: eleven frozen tools plus webhook
// ingest and the call log, over one HTTP handler. It is safe for
// concurrent use.
//
// The pivot rides beside the frozen paths: the pool reader answers
// product_fit when Deps wired a pool index, one fit writer per shopper
// records the pooled rows when the identity salt, the fit index and a
// line resolver are all present, and the standing store keeps each
// shopper's journal across restarts when a directory was configured.
// The service trusts the edge's X-Memento-* headers and never re-reads
// Authorization: anything reaching this handler directly bypasses the
// key check, so the port stays on loopback. The call log is agent-only
// for the same reason: it carries every shopper's entries and the layer
// does not scope it per shopper.
type Service struct {
	deps            Deps
	graph           memory.Graph
	mu              sync.Mutex
	shoppers        map[string]*shopperState
	calls           []CallEntry
	schemas         map[string]string
	pool            *pool.Reader
	keyer           *identity.Keyer
	lines           fitindex.Lines
	standing        standing.Store
	evidenceJournal *evidence.Journal
	// population is the product lines the layer's own seeded population
	// covers. Recall resolves a request that names a brand and category
	// but no fit handle against it, so a caller that may not invent a
	// handle still reads the pool when exactly one line matches.
	population []domain.ProductLine
}

// NewService builds the service over its deps. It fails closed when a
// dep is missing or a frozen request schema cannot be loaded: a service
// that cannot describe its tools does not start.
func NewService(deps Deps) (*Service, error) {
	if deps.Payments == nil {
		return nil, fmt.Errorf("server: missing payment source")
	}
	if deps.Classifier == nil {
		return nil, fmt.Errorf("server: missing classifier")
	}
	if deps.Store == nil {
		return nil, fmt.Errorf("server: missing evidence store")
	}
	if deps.Graph == nil {
		return nil, fmt.Errorf("server: missing graph")
	}
	if deps.Clock == nil {
		deps.Clock = func() time.Time { return time.Now().UTC() }
	}
	schemas := map[string]string{}
	for _, tool := range contract.Tools() {
		raw, err := loadSchema(deps.SchemaDir, tool.RequestSchema)
		if err != nil {
			return nil, err
		}
		schemas[string(tool.Name)] = raw
	}
	svc := &Service{
		deps:       deps,
		graph:      graphAdapter{inner: deps.Graph},
		shoppers:   map[string]*shopperState{},
		schemas:    schemas,
		lines:      deps.Lines,
		population: seed.PopulationLines(),
	}
	if deps.Pool != nil {
		reader, err := pool.NewReader(deps.Pool)
		if err != nil {
			return nil, fmt.Errorf("server: pool reader: %w", err)
		}
		svc.pool = reader
	}
	if deps.IdentitySalt != "" {
		keyer, err := identity.NewKeyer(deps.IdentitySalt)
		if err != nil {
			return nil, fmt.Errorf("server: identity keyer: %w", err)
		}
		svc.keyer = keyer
	}
	if deps.StandingDir != "" {
		store, err := standing.NewDirStore(deps.StandingDir)
		if err != nil {
			return nil, fmt.Errorf("server: standing store: %w", err)
		}
		svc.standing = store
		journal, err := evidence.NewJournal(deps.StandingDir)
		if err != nil {
			return nil, fmt.Errorf("server: evidence journal: %w", err)
		}
		svc.evidenceJournal = journal
	} else {
		svc.standing = standing.NewMemStore()
	}
	return svc, nil
}

// state returns the shopper's partition, creating it on first use. The
// evidence journal is restored when one exists: the documents go back into
// the store under their ids, and the rows, questions, mint counters,
// citations and timelines come back into the partition before the boot
// seed replays, so the seed names what is already held once. The standing
// journal is restored after it: undeclared standings are re-recorded so a
// later belief declares them, and the pooling choice and correction views
// come back with them. A caller that fails a restore fails the call:
// starting empty would silently lose evidence or the ability to declare.
func (s *Service) state(ctx context.Context, shopper string) (*shopperState, error) {
	st, ok := s.shoppers[shopper]
	if !ok {
		st = &shopperState{
			ingest:      ingest.NewService(s.deps.Payments, ingest.WithVerifier(s.deps.Verifier)),
			beliefs:     map[string]*beliefRecord{},
			evidence:    map[string]*evidenceRecord{},
			pending:     map[string]*pendingRecord{},
			corrections: map[string]*correctionRecord{},
			timelines:   map[string][]contract.TimelineEvent{},
			notices:     map[string]string{},
			captures:    map[string][]string{},
			webhooks:    map[string]bool{},
			seq:         map[string]int{},
			streaks:     map[streakKey]*domain.OppositeStreak{},
			journal:     &memory.StandingCorrections{},
			pairing:     map[string]standing.ViewLink{},
		}
		st.pooling.Store(true)
		if s.deps.FitRows != nil && s.keyer != nil && s.lines != nil {
			writer, err := fitindex.NewWriter(s.deps.FitRows, s.keyer, shopper, s.lines,
				fitindex.WithPooling(st.pooling.Load))
			if err != nil {
				return nil, fmt.Errorf("server: fit writer for %q: %w", shopper, err)
			}
			st.writer = writer
		}
		if err := s.restoreEvidence(ctx, st, shopper); err != nil {
			return nil, err
		}
		if err := s.restoreStanding(ctx, st, shopper); err != nil {
			return nil, err
		}
		s.shoppers[shopper] = st
	}
	return st, nil
}

// mint returns the next wire id under one prefix, deterministic in call
// order so tests replay exactly. A reserved id holds its number: a
// correction id taken from the journal or the seed makes the next mint
// of that prefix start past it.
func (st *shopperState) mint(prefix string) string {
	st.seq[prefix]++
	return fmt.Sprintf("%s_%06d", prefix, st.seq[prefix])
}

// reserveID holds one id's number against the mint sequence under its
// prefix, so mint never returns an id the store already holds. An id in
// another shape — a fixture's opaque correction id — names no number and
// reserves nothing. The caller holds the lock.
func (st *shopperState) reserveID(id string) {
	prefix, digits, ok := strings.Cut(id, "_")
	if !ok || prefix == "" || digits == "" {
		return
	}
	n, err := strconv.Atoi(digits)
	if err != nil || n < 1 {
		return
	}
	if n > st.seq[prefix] {
		st.seq[prefix] = n
	}
}

// takeCorrection records one correction under its id, in arrival order,
// the first time the id is seen. A correction id identifies one
// correction for the life of the store: a restored id and the seed that
// replays it are one correction, so a later sighting keeps the first
// view in its first place instead of appending a second row. The id's
// number is reserved against the mint sequence, so a later mint cannot
// reuse it. The caller holds the lock.
func (st *shopperState) takeCorrection(id string, rec *correctionRecord) {
	if _, taken := st.corrections[id]; taken {
		return
	}
	st.reserveID(id)
	st.corrections[id] = rec
	st.corrOrder = append(st.corrOrder, id)
}

// log records one interface call. The caller holds the service lock.
func (s *Service) log(tool, shopper string, trace *string) {
	entry := CallEntry{
		Seq:       len(s.calls) + 1,
		At:        s.deps.Clock().Format(time.RFC3339),
		Tool:      tool,
		ShopperID: shopper,
		TraceID:   trace,
	}
	s.calls = append(s.calls, entry)
}

// Calls returns the call log in arrival order, optionally narrowed to one
// host trace. The returned entries are copies. The log is agent-only: it
// carries every shopper's entries, and narrowing it to one shopper is the
// HTTP route's job, reading the edge's X-Memento-Shopper header.
func (s *Service) Calls(trace string) []CallEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []CallEntry{}
	for _, entry := range s.calls {
		if trace != "" && (entry.TraceID == nil || *entry.TraceID != trace) {
			continue
		}
		out = append(out, entry)
	}
	return out
}

// traceOf normalizes an optional trace id for logging and echo.
func traceOf(raw string) *string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	out := raw
	return &out
}

// countsTowardView maps a belief kind to its wire form, or null when the
// evidence counts toward nothing.
func countsTowardView(kind domain.BeliefKind) *domain.BeliefKind {
	if kind == domain.BeliefKindNone {
		return nil
	}
	out := kind
	return &out
}

// evidenceView renders one stored row in its wire form.
func evidenceView(rec *evidenceRecord) contract.EvidenceView {
	row := rec.row
	var reason *contract.ReasonView
	if row.Reason != nil {
		reason = &contract.ReasonView{
			Category:   row.Reason.Category,
			Text:       row.Reason.Text,
			Confidence: row.Reason.Confidence,
		}
	}
	return contract.EvidenceView{
		EvidenceID: rec.id,
		Outcome: contract.OutcomeView{
			Kind:        row.Outcome.Kind,
			Status:      row.Outcome.Status,
			AmountMinor: row.Outcome.AmountMinor,
			Currency:    row.Outcome.Currency,
		},
		Item: contract.ItemView{
			BrandID:   row.Item.BrandID,
			BrandName: row.Item.BrandName,
			LineName:  row.Item.LineName,
			SKU:       row.Item.SKU,
			Size:      row.Item.Size,
			Category:  row.Item.Category,
		},
		Reason:       reason,
		CountsToward: countsTowardView(rec.counts),
		WriteOp:      rec.writeOp,
		Link: contract.TransactionLinkView{
			OrderID:   row.Link.OrderID,
			CaptureID: row.Link.CaptureID,
			RefundID:  row.Link.RefundID,
			DisputeID: row.Link.DisputeID,
			CustomID:  row.Link.CustomID,
			EventID:   row.Link.EventID,
		},
		Seeded:     row.Seeded,
		OccurredAt: row.OccurredAt.Format(time.RFC3339),
	}
}

// readBelief inspects one belief in the graph and maps it to its ledger
// reading. The graph is the source of truth for status and standing.
func (s *Service) readBelief(ctx context.Context, content string) (memory.Reading, error) {
	return memory.ReadBelief(ctx, s.graph, content)
}

// beliefView renders one stored belief with its live graph reading and
// its evidence in write order.
func (s *Service) beliefView(ctx context.Context, st *shopperState, rec *beliefRecord) (contract.BeliefView, error) {
	reading, err := s.readBelief(ctx, rec.content)
	if err != nil {
		return contract.BeliefView{}, err
	}
	evs := make([]contract.EvidenceView, 0, len(rec.evIDs))
	for _, id := range rec.evIDs {
		ev, ok := st.evidence[id]
		if !ok {
			continue
		}
		evs = append(evs, evidenceView(ev))
	}
	var against *contract.CorrectionView
	if reading.Contradicted {
		latest := latestCorrection(st, rec.id)
		if latest == nil {
			return contract.BeliefView{}, fmt.Errorf("%w: belief %q", ErrOverrideMissing, rec.id)
		}
		view := latest.view
		against = &view
	}
	return contract.BeliefView{
		BeliefID:       rec.id,
		Kind:           rec.kind,
		Subject:        rec.subject,
		Value:          rec.value,
		Status:         reading.Status,
		Standing:       reading.Standing,
		Contradicted:   reading.Contradicted,
		ContradictedBy: against,
		Evidence:       evs,
		Seeded:         rec.seeded,
		UpdatedAt:      rec.updatedAt.Format(time.RFC3339),
	}, nil
}

// latestCorrection returns the newest correction declared against one
// belief, or nil when none applies.
func latestCorrection(st *shopperState, beliefID string) *correctionRecord {
	var latest *correctionRecord
	for _, id := range st.corrOrder {
		rec, ok := st.corrections[id]
		if !ok || rec.view.ContradictsBeliefID != beliefID {
			continue
		}
		if latest == nil || !rec.at.Before(latest.at) {
			latest = rec
		}
	}
	return latest
}

// refreshUpdated stamps a belief's last change. The caller holds the lock.
func (rec *beliefRecord) touch(at time.Time) {
	rec.updatedAt = at
}

// timeline appends one row to a belief's history. The caller holds the lock.
func (st *shopperState) timeline(beliefID string, event contract.TimelineEvent) {
	st.timelines[beliefID] = append(st.timelines[beliefID], event)
}

// indexEvidence stores one evidence text and returns its doc id. The id
// travels into the graph stub so prose stays out of the graph.
func (s *Service) indexEvidence(ctx context.Context, row domain.Evidence, fitHandle string) (string, error) {
	doc := evidence.ForReason(row, fitHandle)
	id, err := s.deps.Store.IndexDoc(ctx, doc.Text)
	if err != nil {
		return "", fmt.Errorf("server: index evidence: %w", err)
	}
	return id, nil
}

// writeEvidenceRow indexes one reasoned row, writes it to the graph with
// the doc id in the stub, and links it to its belief at the row's own
// event time.
func (s *Service) writeEvidenceRow(ctx context.Context, st *shopperState, rec *beliefRecord, evRec *evidenceRecord, key string) error {
	return s.writeEvidenceRowAt(ctx, st, rec, evRec, key, evRec.row.OccurredAt)
}

// writeEvidenceRowAt is writeEvidenceRow with the evidence event's time
// given separately. A classified outcome is recorded at the outcome's
// time; feedback is recorded when the shopper gives it, even though the
// row it describes is the purchase's own capture.
func (s *Service) writeEvidenceRowAt(ctx context.Context, st *shopperState, rec *beliefRecord, evRec *evidenceRecord, key string, at time.Time) error {
	docID, err := s.indexEvidence(ctx, evRec.row, "")
	if err != nil {
		return err
	}
	stub, err := evidence.GraphStub(docID, key)
	if err != nil {
		return fmt.Errorf("server: build graph stub: %w", err)
	}
	row := evRec.row
	row.OccurredAt = at
	written, err := memory.WriteEvidence(ctx, s.graph, rec.content, rec.kind, rec.direction, stub, row)
	if err != nil {
		return fmt.Errorf("server: write evidence: %w", err)
	}
	if !written {
		return fmt.Errorf("server: reasoned evidence wrote nothing")
	}
	evRec.docID = docID
	evRec.writeOp = domain.WriteRecordAction
	evRec.beliefIDs = append(evRec.beliefIDs, rec.id)
	rec.evIDs = append(rec.evIDs, evRec.id)
	rec.touch(at)
	return nil
}
