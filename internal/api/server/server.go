package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/ingest"
	"github.com/nrynss/memento-emptor/internal/memory"
)

// The service's sentinel conditions. Callers branch on these with errors.Is
// and never on message text.
var (
	// ErrUnknownBelief answers a belief id the shopper has no record of.
	ErrUnknownBelief = errors.New("server: unknown belief")
	// ErrUnknownQuestion answers a question id with no open pending question.
	ErrUnknownQuestion = errors.New("server: unknown question")
	// ErrQuestionClosed answers a record_reason for a question that is
	// already answered. The answer counted once; it counts only once.
	ErrQuestionClosed = errors.New("server: question is already answered")
	// ErrUnknownPurchase answers feedback for a transaction the layer
	// never saw. Feedback joins on the capture it concerns, never floats.
	ErrUnknownPurchase = errors.New("server: unknown purchase")
	// ErrFeedbackMismatch answers feedback whose variant does not match
	// the purchase it names. Fit feedback is tied to a specific purchase.
	ErrFeedbackMismatch = errors.New("server: feedback does not match the purchase")
	// ErrEmptyCorrection answers a correction with no shopper words.
	ErrEmptyCorrection = errors.New("server: correction has no text")
	// ErrMissingShopper answers a call with no shopper id. Every call is
	// scoped to one shopper, so an unscoped call answers nothing.
	ErrMissingShopper = errors.New("server: missing shopper id")
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
// Counts is stored, not derived: feedback rows carry their verdict's
// kind while their reason category stays none.
type evidenceRecord struct {
	id        string
	row       domain.Evidence
	counts    domain.BeliefKind
	writeOp   domain.WriteOp
	docID     string
	beliefIDs []string
}

// beliefRecord is one stored belief: its identity, its graph content, and
// when it last changed. Status, standing and warnings always come from
// the graph at read time, never from stored copies.
type beliefRecord struct {
	id        string
	kind      domain.BeliefKind
	subject   domain.Subject
	value     string
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
// never leak into another's recall.
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
	seq         map[string]int
}

// Service is the running memory layer: nine frozen tools plus webhook
// ingest and the call log, over one HTTP handler. It is safe for
// concurrent use.
type Service struct {
	deps     Deps
	graph    memory.Graph
	mu       sync.Mutex
	shoppers map[string]*shopperState
	calls    []CallEntry
	schemas  map[string]string
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
	return &Service{
		deps:     deps,
		graph:    graphAdapter{inner: deps.Graph},
		shoppers: map[string]*shopperState{},
		schemas:  schemas,
	}, nil
}

// state returns the shopper's partition, creating it on first use.
func (s *Service) state(shopper string) *shopperState {
	st, ok := s.shoppers[shopper]
	if !ok {
		st = &shopperState{
			ingest:      ingest.NewService(s.deps.Payments),
			beliefs:     map[string]*beliefRecord{},
			evidence:    map[string]*evidenceRecord{},
			pending:     map[string]*pendingRecord{},
			corrections: map[string]*correctionRecord{},
			timelines:   map[string][]contract.TimelineEvent{},
			notices:     map[string]string{},
			captures:    map[string][]string{},
			seq:         map[string]int{},
		}
		s.shoppers[shopper] = st
	}
	return st
}

// mint returns the next wire id under one prefix, deterministic in call
// order so tests replay exactly.
func (st *shopperState) mint(prefix string) string {
	st.seq[prefix]++
	return fmt.Sprintf("%s_%06d", prefix, st.seq[prefix])
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
// host trace. The returned entries are copies.
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
// the doc id in the stub, and links it to its belief.
func (s *Service) writeEvidenceRow(ctx context.Context, st *shopperState, rec *beliefRecord, evRec *evidenceRecord, key string) error {
	docID, err := s.indexEvidence(ctx, evRec.row, "")
	if err != nil {
		return err
	}
	stub, err := evidence.GraphStub(docID, key)
	if err != nil {
		return fmt.Errorf("server: build graph stub: %w", err)
	}
	written, err := memory.WriteEvidence(ctx, s.graph, rec.content, rec.kind, stub, evRec.row)
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
	rec.touch(evRec.row.OccurredAt)
	return nil
}
