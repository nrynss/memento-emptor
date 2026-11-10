// Command t55 drives the T5.5 comparison turns: every comparison shopper
// through arms 1-3, three repeats each, against the frozen inputs.
//
// This is run-record scaffolding only: prompts come from the frozen
// harness, extraction from the frozen pilot rule, the record schema is the
// frozen pilot RunFile, and grading stays in the frozen grade command. What
// this driver adds is the transport the frozen pilot drive command cannot
// do for T5.5: repeat-indexed run ids, an in-code cumulative spend cap, and
// the arm-3 memory fetch through the real layer calls over a live Lambo
// session (the T2.4b/T2.6 driver pattern: real graph client, throwaway
// sessions erased after the run).
//
// The frozen pilot drive command (bench/pilot/cmd/drive) is reused in
// spirit: the proxy mechanics below (billing reads, conversation + message,
// trace polling, scrubbing) follow it call for call, and any divergence is
// a defect. What is not reused verbatim: the pilot command pins run ids to
// one turn per shopper per arm and enforces only per-run caps, while T5.5
// needs three repeats and a cumulative $5.00 cap in code.
//
// Arm-3 memory (FROZEN section 3, F4): for each shopper the driver seeds a
// live Lambo session from the frozen history through the real write path
// (server.LoadSeed, the same function the demo loader uses), waits for the
// daemon to settle, then serves the memory through the real
// recall_for_purchase plus pending_questions calls. The request and the
// full response are recorded in the run file, and the response text is what
// the prompt's memory block carries. The stub LayerContext is never used.
//
// Seeding maps each history row the way the live path would file it (each
// rule cites the service function it mirrors; anything outside the frozen
// set's shapes stops the run instead of inventing a filing):
//   - general statements never enter (no live ingest reads them).
//   - kept rows are observations with no structural edge (no graph write).
//   - fit feedback joins its capture and births or joins a fit belief
//     (beliefForFeedback); the value mirrors feedbackBeliefValue.
//   - classified fit/seller rows attach to the matching belief
//     (beliefForReason), except opposite-direction fit rows against an
//     existing fit belief, which land unattached below the rival-birth
//     threshold (the T2.7 direction rule, threshold 3 in T2.8).
//   - classified no-fault rows (damaged, changed mind) close with no
//     belief; only a below-floor unclear row stays an open question (F3).
//
// Mint order (ev_*, q_*, blf_*) follows the live service counters so the
// served ids match what the live path would have minted.
//
// Live services touched: the Astropods messaging proxy (model turns) and
// the Lambo daemon locally with the native embedder (arm-3 memory). No
// PayPal or Elastic evidence reads: histories are frozen replay inputs,
// and the evidence store is the in-memory lexical stand-in exactly as in
// the T2.6 live leg. Keys travel only in process memory via tools/env.sh.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/bench/harness"
	"github.com/nrynss/memento-emptor/bench/pilot"
	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/api/server"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/seed"
)

// Caps and guards. Per-run caps are the frozen harness DefaultCaps,
// enforced before any model call. The cumulative and single-turn guards
// are the T5.5 brief's binding budget.
const (
	// cumulativeBudgetUSD is the hard ceiling across all T5.5 turns.
	cumulativeBudgetUSD = 5.00
	// singleTurnGuardUSD aborts the turn and the run when one trace
	// costs more than this. Pilot turns cost ~$0.001-0.002.
	singleTurnGuardUSD = 0.10
)

// browserUA is required: the default scripting UA trips the edge bot check
// (same as the frozen pilot drive command).
const browserUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"

// anchor is the arbitrary origin event-time spans count from, matching the
// harness anchor so seeded times agree with the offline materialization.
var anchor = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

// waitMS is the receipt wait per Lambo write, the T2.6 driver's wait.
const waitMS = 34000

// fail prints to stderr and exits nonzero.
func fail(err error) {
	fmt.Fprintf(os.Stderr, "t55: %v\n", err)
	os.Exit(1)
}

// ---------------------------------------------------------------------------
// Closed deps: the seed path never classifies and never reads payments, so
// both fakes fail closed. If the service ever calls one, the run stops
// instead of inventing a classification or a payment read.
// ---------------------------------------------------------------------------

// closedClassifier refuses every classification.
type closedClassifier struct{}

// Classify always fails: seeding replays pre-classified rows only.
func (closedClassifier) Classify(_ context.Context, _ string) (fakes.Classification, error) {
	return fakes.Classification{}, errors.New("t55: classifier must never be called during seeding")
}

// closedPayments refuses every payment read.
type closedPayments struct{}

// VerifyWebhook always fails: seeding replays frozen histories, not webhooks.
func (closedPayments) VerifyWebhook(_ context.Context, _ string) (fakes.WebhookEvent, error) {
	return fakes.WebhookEvent{}, errors.New("t55: payment source must never be read during seeding")
}

// GetOrder always fails.
func (closedPayments) GetOrder(_ context.Context, _ string) (fakes.OrderRecord, error) {
	return fakes.OrderRecord{}, errors.New("t55: payment source must never be read during seeding")
}

// GetRefund always fails.
func (closedPayments) GetRefund(_ context.Context, _ string) (fakes.RefundRecord, error) {
	return fakes.RefundRecord{}, errors.New("t55: payment source must never be read during seeding")
}

// GetDispute always fails.
func (closedPayments) GetDispute(_ context.Context, _ string) (fakes.DisputeRecord, error) {
	return fakes.DisputeRecord{}, errors.New("t55: payment source must never be read during seeding")
}

// t55Contract is the store's embedding contract. It carries no model name,
// dimensions, endpoint or key — only its kind, so the in-memory lexical
// store stands in for the live Elastic proof exactly as in T2.6.
type t55Contract struct{ kind string }

// EmbeddingKind names the contract family.
func (c t55Contract) EmbeddingKind() string { return c.kind }

// Validate reports whether the contract is usable.
func (c t55Contract) Validate() error {
	if c.kind == "" {
		return errors.New("t55: contract names no kind")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Live Lambo graph client over one session (the T2.4b/T2.6 driver pattern).
// ---------------------------------------------------------------------------

// rpcClient speaks MCP JSON-RPC to one lambo serve child over stdio.
type rpcClient struct {
	cmd *exec.Cmd
	in  io.WriteCloser
	out io.Reader
	seq int
}

// rpcRequest is one JSON-RPC request envelope.
type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// rpcResponse is the envelope subset the driver reads.
type rpcResponse struct {
	Result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StructuredContent json.RawMessage `json:"structuredContent"`
	} `json:"result"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// startLambo provisions the store under cfg and starts the MCP server for
// session over its stdio pipes.
func startLambo(bin, cfg, session, agent string) (*rpcClient, error) {
	provision := exec.Command(bin, "--config", cfg, "provision")
	if raw, err := provision.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("lambo provision: %w: %s", err, strings.TrimSpace(string(raw)))
	}
	cmd := exec.Command(bin, "--config", cfg, "serve", "--session", session, "--agent", agent)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("lambo stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("lambo stdout: %w", err)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("lambo start: %w", err)
	}
	client := &rpcClient{cmd: cmd, in: stdin, out: stdout}
	init := map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05", "capabilities": map[string]any{},
			"clientInfo": map[string]any{"name": "t55-driver", "version": "0.1.0"},
		},
	}
	raw, err := json.Marshal(init)
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, err
	}
	if _, err := fmt.Fprintf(stdin, "%s\n", raw); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("lambo initialize: %w", err)
	}
	if _, err := client.readLine(); err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("lambo initialize reply: %w", err)
	}
	client.seq = 2
	return client, nil
}

// readLine reads one newline-terminated JSON-RPC message.
func (c *rpcClient) readLine() ([]byte, error) {
	var line []byte
	buf := make([]byte, 1)
	for {
		n, err := c.out.Read(buf)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			continue
		}
		if buf[0] == '\n' {
			return line, nil
		}
		line = append(line, buf[0])
	}
}

// call invokes one MCP tool and returns its result envelope.
func (c *rpcClient) call(name string, args map[string]any) (rpcResponse, error) {
	var zero rpcResponse
	c.seq++
	req := rpcRequest{JSONRPC: "2.0", ID: c.seq, Method: "tools/call",
		Params: map[string]any{"name": name, "arguments": args}}
	raw, err := json.Marshal(req)
	if err != nil {
		return zero, err
	}
	if _, err := fmt.Fprintf(c.in, "%s\n", raw); err != nil {
		return zero, fmt.Errorf("lambo call %s: %w", name, err)
	}
	line, err := c.readLine()
	if err != nil {
		return zero, fmt.Errorf("lambo call %s reply: %w", name, err)
	}
	var resp rpcResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return zero, fmt.Errorf("lambo call %s decode: %w", name, err)
	}
	if resp.Error != nil {
		return zero, fmt.Errorf("lambo call %s: %s", name, resp.Error.Message)
	}
	return resp, nil
}

// textOf joins the result's text content blocks.
func textOf(resp rpcResponse) string {
	parts := []string{}
	for _, block := range resp.Result.Content {
		if block.Type == "text" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// stop closes the child's stdin and waits for it to exit.
func (c *rpcClient) stop() {
	_ = c.in.Close()
	_ = c.cmd.Wait()
}

// eraseSession removes one session. It is safe to repeat.
func eraseSession(bin, cfg, session string) (string, error) {
	raw, err := exec.Command(bin, "--config", cfg, "erase-session", "--session", session, "--confirm", session).CombinedOutput()
	text := strings.TrimSpace(string(raw))
	if err != nil {
		return text, fmt.Errorf("lambo erase-session %s: %w: %s", session, err, text)
	}
	return text, nil
}

// lamboGraph is a Graph and EvidenceGraph over one live Lambo session.
type lamboGraph struct {
	client *rpcClient
	agent  string
}

// Compile-time proof the live client satisfies the seams the service
// builds on.
var _ fakes.EvidenceGraph = (*lamboGraph)(nil)

// waitApplied blocks until the daemon applies one receipt.
func (g *lamboGraph) waitApplied(receipt, what string) error {
	resp, err := g.client.call("lambo_stats", map[string]any{
		"agent_id": g.agent, "receipt": receipt, "wait_ms": waitMS,
	})
	if err != nil {
		return fmt.Errorf("t55: wait %s: %w", what, err)
	}
	text := textOf(resp)
	if strings.Contains(text, "embedder unavailable") || strings.Contains(text, "error sending request") {
		return fmt.Errorf("t55: embedder down after %s: %s", what, lastLine(text))
	}
	if !strings.Contains(text, "applied") {
		return fmt.Errorf("t55: receipt for %s never applied: %.200s", what, text)
	}
	return nil
}

// lastLine returns the final non-empty line of a report.
func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}

// receiptOf reads the write receipt from a tools/call envelope.
func receiptOf(resp rpcResponse) (string, error) {
	var structured struct {
		Receipt string `json:"receipt"`
	}
	if err := json.Unmarshal(resp.Result.StructuredContent, &structured); err != nil {
		return "", fmt.Errorf("t55: decode receipt: %w", err)
	}
	if structured.Receipt == "" {
		return "", fmt.Errorf("t55: write issued no receipt")
	}
	return structured.Receipt, nil
}

// Derive records one belief or statement.
func (g *lamboGraph) Derive(ctx context.Context, content, kind string, eventTime time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	args := map[string]any{
		"agent_id": g.agent,
		"concepts": []map[string]any{{"content": content, "concept_type": kind}},
	}
	if !eventTime.IsZero() {
		args["event_time"] = eventTime.UTC().Format(time.RFC3339Nano)
	}
	resp, err := g.client.call("lambo_derive", args)
	if err != nil {
		return err
	}
	receipt, err := receiptOf(resp)
	if err != nil {
		return err
	}
	return g.waitApplied(receipt, fmt.Sprintf("derive %.40q", content))
}

// RecordAction records one structural write.
func (g *lamboGraph) RecordAction(ctx context.Context, action string, dependsOn, against []string, eventTime time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	args := map[string]any{"agent_id": g.agent, "action": action}
	if len(dependsOn) > 0 {
		args["depends_on"] = dependsOn
	}
	if len(against) > 0 {
		args["against"] = against
	}
	if !eventTime.IsZero() {
		args["event_time"] = eventTime.UTC().Format(time.RFC3339Nano)
	}
	resp, err := g.client.call("lambo_record_action", args)
	if err != nil {
		return err
	}
	receipt, err := receiptOf(resp)
	if err != nil {
		return err
	}
	return g.waitApplied(receipt, fmt.Sprintf("record_action %.40q", action))
}

// Recall lists the live session's hit labels for one query. The service
// ranks from its stored beliefs and never calls this; it exists so the
// client satisfies the seam honestly.
func (g *lamboGraph) Recall(_ context.Context, query string) ([]string, error) {
	resp, err := g.client.call("lambo_recall", map[string]any{"agent_id": g.agent, "query": query})
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, line := range strings.Split(textOf(resp), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out, nil
}

// inspectShape is the structured subset of lambo_inspect the driver maps
// to a belief report.
type inspectShape struct {
	Admission struct {
		Status string `json:"status"`
		Checks []struct {
			Gate    string  `json:"gate"`
			Current float64 `json:"current"`
		} `json:"checks"`
	} `json:"admission"`
	Focus struct {
		BlastRadius int `json:"blast_radius"`
	} `json:"focus"`
	Warnings []string `json:"warnings"`
}

// Inspect reports the promotion state of one belief.
func (g *lamboGraph) Inspect(ctx context.Context, content string) (fakes.BeliefReport, error) {
	if err := ctx.Err(); err != nil {
		return fakes.BeliefReport{}, err
	}
	resp, err := g.client.call("lambo_inspect", map[string]any{
		"agent_id": g.agent, "focus": content, "depth": 2,
	})
	if err != nil {
		return fakes.BeliefReport{}, err
	}
	var shape inspectShape
	if err := json.Unmarshal(resp.Result.StructuredContent, &shape); err != nil {
		return fakes.BeliefReport{}, fmt.Errorf("t55: decode inspect: %w", err)
	}
	report := fakes.BeliefReport{Content: content, Dependents: shape.Focus.BlastRadius}
	switch strings.ToLower(shape.Admission.Status) {
	case "venerable":
		report.State = domain.GraphVenerable
	case "candidate":
		report.State = domain.GraphCandidate
	default:
		report.State = domain.GraphNone
	}
	for _, check := range shape.Admission.Checks {
		switch check.Gate {
		case "distinct_interactions":
			report.DistinctInteractions = int(check.Current)
		case "coverage":
			report.Coverage = check.Current
		}
	}
	if len(shape.Warnings) > 0 {
		report.Contradicted = true
		report.Warning = "Contradicted by " + strings.Join(shape.Warnings, "; ")
	}
	return report, nil
}

// ---------------------------------------------------------------------------
// History to seed mapping (mirrors the live path per-row decisions).
// ---------------------------------------------------------------------------

// at returns the event time for a seed day.
func at(day float64) time.Time {
	return anchor.Add(time.Duration(day * float64(time.Hour*24)))
}

// atStr formats an event time the way the wire does.
func atStr(day float64) string {
	return at(day).Format(time.RFC3339)
}

// buildSeed converts one frozen comparison history into the service seed
// the live path would have filed for it. Every branch names the live-path
// rule it mirrors; any row outside the frozen set's shapes is a fatal
// error, never an invented filing.
func buildSeed(history seed.History) (server.Seed, error) {
	s := server.Seed{
		ShopperID:        history.ShopperID,
		Beliefs:          []contract.BeliefView{},
		Links:            map[string][]string{},
		Pending:          contract.PendingQuestionsResponse{ShopperID: history.ShopperID, Questions: []contract.QuestionView{}},
		Timelines:        map[string][]contract.TimelineEvent{},
		Notices:          map[string]string{},
		FeedbackVerdicts: map[string]contract.FeedbackVerdict{},
	}
	rows := append([]seed.Row(nil), history.Rows...)
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Day < rows[j].Day })

	evSeq, qSeq, blfSeq := 0, 0, 0
	mintEv := func() string {
		evSeq++
		return fmt.Sprintf("ev_%06d", evSeq)
	}
	// beliefByBrand files fit rows the way beliefForReason does: the
	// first classified direction births the belief; a same-direction row
	// joins it; an opposite-direction row lands unattached below the
	// rival-birth threshold (T2.7/T2.8), so it is not seeded.
	type fitFiling struct {
		belief *contract.BeliefView
	}
	fitByBrand := map[string]*fitFiling{}
	sellerBySubject := map[string]*contract.BeliefView{}

	for _, row := range rows {
		if row.Signal == domain.SignalGeneralStatement {
			// Statements never enter the live path: no ingest reads
			// them, they derive at most, and recall never serves
			// them. Skipped, not seeded.
			continue
		}
		if row.Outcome == nil {
			return server.Seed{}, fmt.Errorf("t55: seed %s: day %.0f carries no outcome and no statement", history.ShopperID, row.Day)
		}
		ev := row.SeededEvidence(anchor)

		switch row.Signal {
		case domain.SignalKeptPastWindow:
			// Kept rows are observations with no structural edge:
			// the live path mints an evidence id for the row but
			// writes nothing to the graph. The mint keeps later ids
			// aligned with the live counters.
			mintEv()
			continue
		case domain.SignalFitFeedback:
			// Fit feedback joins its capture and births or joins a
			// fit belief (beliefForFeedback). Only a fits-well
			// verdict is in the frozen set; anything else stops the
			// run instead of guessing the verdict mapping.
			if !strings.Contains(strings.ToLower(row.Note), "fits well") {
				return server.Seed{}, fmt.Errorf("t55: seed %s: feedback note without fits-well: %q", history.ShopperID, row.Note)
			}
			evID := mintEv()
			filing, ok := fitByBrand[row.Item.BrandID]
			if !ok {
				blfSeq++
				view := contract.BeliefView{
					BeliefID: blfSeqID(blfSeq),
					Kind:     domain.BeliefKindFit,
					Subject:  domain.Subject{Kind: domain.SubjectBrand, ID: row.Item.BrandID, Name: row.Item.BrandName},
					// Mirrors feedbackBeliefValue(size) in
					// internal/api/server/content.go.
					Value:     fmt.Sprintf("size %s fits this shopper now", row.Item.Size),
					Seeded:    true,
					UpdatedAt: atStr(row.Day),
				}
				s.Beliefs = append(s.Beliefs, view)
				filing = &fitFiling{belief: &s.Beliefs[len(s.Beliefs)-1]}
				fitByBrand[row.Item.BrandID] = filing
			}
			counts := domain.BeliefKindFit
			filing.belief.Evidence = append(filing.belief.Evidence, contract.EvidenceView{
				EvidenceID: evID,
				Outcome: contract.OutcomeView{
					Kind:        ev.Outcome.Kind,
					Status:      ev.Outcome.Status,
					AmountMinor: ev.Outcome.AmountMinor,
					Currency:    ev.Outcome.Currency,
				},
				Item: itemView(row),
				Reason: &contract.ReasonView{
					// The frozen demo precedent renders
					// feedback with category none and the
					// recorded words. The seed note is the
					// only text the frozen history carries,
					// used verbatim and documented here, not
					// invented shopper words.
					Category:   domain.CategoryNone,
					Text:       row.Note,
					Confidence: 1.0,
				},
				CountsToward: &counts,
				WriteOp:      domain.WriteRecordAction,
				Link:         linkView(row),
				Seeded:       true,
				OccurredAt:   atStr(row.Day),
			})
			filing.belief.UpdatedAt = atStr(row.Day)
			s.Links[evID] = []string{filing.belief.BeliefID}
			s.FeedbackVerdicts[evID] = contract.FeedbackFitsWell
		case domain.SignalClassifiedOutcome:
			if row.Reason == nil {
				// A classified row with no reason cannot exist
				// in the live path (ingest files a question;
				// the classifier always answers). Stop.
				return server.Seed{}, fmt.Errorf("t55: seed %s: classified row day %.0f carries no reason", history.ShopperID, row.Day)
			}
			counts := domain.Evidence{Reason: row.Reason}.CountsToward()
			switch counts {
			case domain.BeliefKindSeller:
				evID := mintEv()
				key := "seller-for-" + row.Item.BrandID
				view, ok := sellerBySubject[key]
				if !ok {
					// Mirrors ensureSellerBelief: one seller
					// belief behind the brand's transaction,
					// created on first sight.
					blfSeq++
					s.Beliefs = append(s.Beliefs, contract.BeliefView{
						BeliefID: blfSeqID(blfSeq),
						Kind:     domain.BeliefKindSeller,
						Subject: domain.Subject{
							Kind: domain.SubjectSeller,
							ID:   key,
							Name: strings.TrimSpace(row.Item.BrandName) + " seller",
						},
						// Mirrors sellerBeliefValue in
						// internal/api/server/tools.go.
						Value:     "ships reliably for this shopper",
						Seeded:    true,
						UpdatedAt: atStr(row.Day),
					})
					view = &s.Beliefs[len(s.Beliefs)-1]
					sellerBySubject[key] = view
				}
				kind := domain.BeliefKindSeller
				view.Evidence = append(view.Evidence, evidenceView(evID, row, &kind))
				view.UpdatedAt = atStr(row.Day)
				s.Links[evID] = []string{view.BeliefID}
			case domain.BeliefKindFit:
				want := row.Reason.Category.Direction()
				filing, ok := fitByBrand[row.Item.BrandID]
				if !ok {
					// Mirrors beliefForReason with no prior
					// fit belief: the first classified
					// direction births the belief. The value
					// mirrors fitBeliefValue(category, brand,
					// size) in content.go.
					blfSeq++
					s.Beliefs = append(s.Beliefs, contract.BeliefView{
						BeliefID:  blfSeqID(blfSeq),
						Kind:      domain.BeliefKindFit,
						Subject:   domain.Subject{Kind: domain.SubjectBrand, ID: row.Item.BrandID, Name: row.Item.BrandName},
						Value:     fitValue(row.Reason.Category, row.Item.BrandName, row.Item.Size),
						Seeded:    true,
						UpdatedAt: atStr(row.Day),
					})
					filing = &fitFiling{belief: &s.Beliefs[len(s.Beliefs)-1]}
					fitByBrand[row.Item.BrandID] = filing
					evID := mintEv()
					kind := domain.BeliefKindFit
					filing.belief.Evidence = append(filing.belief.Evidence, evidenceView(evID, row, &kind))
					s.Links[evID] = []string{filing.belief.BeliefID}
					continue
				}
				if domain.DirectionForValue(filing.belief.Value) != want {
					// Opposite direction against an existing
					// fit belief lands unattached below the
					// rival-birth threshold (T2.7/T2.8): the
					// live path writes no graph row and
					// closes the question. The mint still
					// advances so later ids align.
					mintEv()
					continue
				}
				evID := mintEv()
				kind := domain.BeliefKindFit
				filing.belief.Evidence = append(filing.belief.Evidence, evidenceView(evID, row, &kind))
				filing.belief.UpdatedAt = atStr(row.Day)
				s.Links[evID] = []string{filing.belief.BeliefID}
			default:
				// Counts toward nothing. Only a below-floor
				// unclear row stays an open question (the F3
				// rule); classified no-fault rows close with
				// no belief (TestChangedMind... proves it).
				evID := mintEv()
				_ = evID
				if row.Reason.Category == domain.CategoryUnclear {
					qSeq++
					s.Pending.Questions = append(s.Pending.Questions, contract.QuestionView{
						QuestionID:    fmt.Sprintf("q_%06d", qSeq),
						Outcome:       outcomeView(row),
						Item:          itemView(row),
						Link:          linkView(row),
						Seeded:        true,
						AskedCount:    0,
						LastAskedAt:   nil,
						AwaitingSince: atStr(row.Day),
					})
				}
			}
		default:
			return server.Seed{}, fmt.Errorf("t55: seed %s: unknown signal %q day %.0f", history.ShopperID, string(row.Signal), row.Day)
		}
	}
	return s, nil
}

// blfSeqID mints belief ids in creation order, matching the live counter.
func blfSeqID(n int) string {
	return fmt.Sprintf("blf_%06d", n)
}

// fitValue mirrors fitBeliefValue in internal/api/server/content.go: too
// small sizes up one rung, anything else sizes down.
func fitValue(category domain.Category, brandName, size string) string {
	if category == domain.CategoryFitTooSmall {
		return fmt.Sprintf("runs small, take size %s", neighbour(size, true))
	}
	return fmt.Sprintf("runs large, take size %s", neighbour(size, false))
}

// sizeLadder mirrors the ladder in internal/api/server/content.go.
var sizeLadder = []string{"XXS", "XS", "S", "M", "L", "XL", "XXL"}

// neighbour moves one rung on the ladder, staying at the edge.
func neighbour(size string, up bool) string {
	for i, rung := range sizeLadder {
		if !strings.EqualFold(rung, size) {
			continue
		}
		if up && i+1 < len(sizeLadder) {
			return sizeLadder[i+1]
		}
		if !up && i-1 >= 0 {
			return sizeLadder[i-1]
		}
		return sizeLadder[i]
	}
	return size
}

// itemView renders a history row's item in wire form.
func itemView(row seed.Row) contract.ItemView {
	return contract.ItemView{
		BrandID:   row.Item.BrandID,
		BrandName: row.Item.BrandName,
		LineName:  row.Item.LineName,
		SKU:       row.Item.SKU,
		Size:      row.Item.Size,
		Category:  row.Item.Category,
	}
}

// outcomeView renders a history row's outcome in wire form.
func outcomeView(row seed.Row) contract.OutcomeView {
	return contract.OutcomeView{
		Kind:        row.Outcome.Kind,
		Status:      row.Outcome.Status,
		AmountMinor: row.Outcome.AmountMinor,
		Currency:    row.Outcome.Currency,
	}
}

// linkView renders a history row's transaction link in wire form.
func linkView(row seed.Row) contract.TransactionLinkView {
	return contract.TransactionLinkView{
		OrderID:   row.Link.OrderID,
		CaptureID: row.Link.CaptureID,
		RefundID:  row.Link.RefundID,
		DisputeID: row.Link.DisputeID,
		CustomID:  row.Link.CustomID,
		EventID:   row.Link.EventID,
	}
}

// evidenceView renders one counted row in wire form.
func evidenceView(evID string, row seed.Row, counts *domain.BeliefKind) contract.EvidenceView {
	return contract.EvidenceView{
		EvidenceID:   evID,
		Outcome:      outcomeView(row),
		Item:         itemView(row),
		Reason:       &contract.ReasonView{Category: row.Reason.Category, Text: row.Reason.Text, Confidence: row.Reason.Confidence},
		CountsToward: counts,
		WriteOp:      domain.WriteRecordAction,
		Link:         linkView(row),
		Seeded:       true,
		OccurredAt:   atStr(row.Day),
	}
}

// ---------------------------------------------------------------------------
// Arm-3 memory: settle the daemon, then serve recall + pending questions.
// ---------------------------------------------------------------------------

// arm3Memory holds one served arm-3 memory: the requests and the full live
// responses.
type arm3Memory struct {
	recallReq   contract.RecallRequest
	recallResp  contract.RecallResponse
	pendingResp contract.PendingQuestionsResponse
	settleNote  string
}

// settleBeliefs polls the live graph until every seeded belief content
// reads stable across two polls at least a minute after seeding, or the
// deadline fails the run loudly. Promotion is eventual; a reading taken
// before the daemon's evaluation would misstate the served standing.
func settleBeliefs(graph *lamboGraph, contents []string) (string, error) {
	if len(contents) == 0 {
		return "no beliefs seeded; nothing to settle", nil
	}
	type reading struct {
		state string
		deps  int
		cov   float64
	}
	last := map[string]reading{}
	polls := 0
	start := time.Now()
	deadline := start.Add(5 * time.Minute)
	for {
		polls++
		stable := time.Since(start) >= time.Minute
		var summary []string
		for _, content := range contents {
			report, err := graph.Inspect(context.Background(), content)
			if err != nil {
				return "", fmt.Errorf("settle inspect %.40q: %w", content, err)
			}
			now := reading{state: string(report.State), deps: report.Dependents, cov: report.Coverage}
			summary = append(summary, fmt.Sprintf("%s=%s/%d/%.3f", shortContent(content), now.state, now.deps, now.cov))
			prev, ok := last[content]
			if !ok || prev != now {
				stable = false
			}
			last[content] = now
		}
		fmt.Printf("settle poll %d (%ds): %s\n", polls, int(time.Since(start).Seconds()), strings.Join(summary, " "))
		if stable {
			return fmt.Sprintf("%d polls over %ds: %s", polls, int(time.Since(start).Seconds()), strings.Join(summary, " ")), nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("settle timed out after %d polls", polls)
		}
		time.Sleep(10 * time.Second)
	}
}

// shortContent trims a belief content for settle logs.
func shortContent(content string) string {
	if len(content) > 40 {
		return content[len(content)-40:]
	}
	return content
}

// beliefContents lists the graph contents a seed writes, so settle polls
// exactly what the service asked the graph to hold.
func beliefContents(s server.Seed) []string {
	out := []string{}
	for _, belief := range s.Beliefs {
		out = append(out, fmt.Sprintf("memento %s %s %s %s",
			string(belief.Kind), string(belief.Subject.Kind), belief.Subject.ID, belief.Value))
	}
	return out
}

// renderMemoryBlock renders the served memory as the prompt's memory
// block: the recall request and the full live responses, nothing added.
func renderMemoryBlock(shopper, brand string, mem arm3Memory) string {
	recallReq, _ := json.Marshal(map[string]string{
		"shopper_id": shopper, "brand_id": brand, "trace_id": mem.recallReq.TraceID,
	})
	recallJSON, _ := json.MarshalIndent(mem.recallResp, "", "  ")
	pendingJSON, _ := json.MarshalIndent(mem.pendingResp, "", "  ")
	var out strings.Builder
	out.WriteString("Memory: Memento layer. Live recall for this shopper and brand, served by the layer:\n")
	out.WriteString("Recall request: " + string(recallReq) + "\n")
	out.WriteString("Recall response:\n" + string(recallJSON) + "\n")
	out.WriteString("Pending questions request: {\"shopper_id\": \"" + shopper + "\"}\n")
	out.WriteString("Pending questions response:\n" + string(pendingJSON))
	return out.String()
}

// ---------------------------------------------------------------------------
// Neutral-instrument transport (mechanics follow bench/pilot/cmd/drive).
// ---------------------------------------------------------------------------

// proxyClient drives the platform messaging proxy for one deployment.
type proxyClient struct {
	deploymentID string
	http         *http.Client
}

func (c *proxyClient) base() string {
	return "https://astropods.com/api/v1/deployments/" + c.deploymentID + "/messaging"
}

// token reads the CLI's fresh org access token from the OS keychain. The
// secret lives only in this process's memory.
func token(ctx context.Context) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("drive: home: %w", err)
	}
	raw, err := os.ReadFile(filepath.Join(home, ".ast", "credentials.json"))
	if err != nil {
		return "", fmt.Errorf("drive: read CLI config: %w", err)
	}
	var config struct {
		Profiles map[string]struct {
			OrgTokens map[string]any `json:"org_tokens"`
		} `json:"profiles"`
		CurrentProfile string `json:"current_profile"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return "", fmt.Errorf("drive: decode CLI config: %w", err)
	}
	profile := config.Profiles[config.CurrentProfile]
	org := ""
	for key := range profile.OrgTokens {
		org = key
	}
	if org == "" {
		return "", errors.New("drive: no org in CLI config")
	}
	out, err := exec.CommandContext(ctx, "secret-tool", "lookup",
		"service", "astro-cli", "username", "default_"+org+"_access_token").Output()
	if err != nil {
		return "", fmt.Errorf("drive: read CLI token from keychain: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *proxyClient) call(ctx context.Context, method, path string, body any) ([]byte, error) {
	tok, err := token(ctx)
	if err != nil {
		return nil, err
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("drive: encode request: %w", err)
		}
		reader = bytes.NewReader(raw)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.base()+path, reader)
	if err != nil {
		return nil, fmt.Errorf("drive: build request: %w", err)
	}
	request.Header.Set("User-Agent", browserUA)
	request.Header.Set("Authorization", "Bearer "+tok)
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("drive: %s %s: %w", method, path, err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("drive: read %s %s: %w", method, path, err)
	}
	if response.StatusCode >= 400 {
		if response.StatusCode == 401 {
			return nil, fmt.Errorf("drive: %s %s: token expired, refresh and retry: %s", method, path, strings.TrimSpace(string(raw)))
		}
		return nil, fmt.Errorf("drive: %s %s: status %d: %s", method, path, response.StatusCode, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}

func (c *proxyClient) createConversation(ctx context.Context) (string, error) {
	raw, err := c.call(ctx, "POST", "/conversations", map[string]any{})
	if err != nil {
		return "", err
	}
	var created struct {
		ConversationID string `json:"conversation_id"`
	}
	if err := json.Unmarshal(raw, &created); err != nil {
		return "", fmt.Errorf("drive: decode conversation: %w", err)
	}
	if created.ConversationID == "" {
		return "", errors.New("drive: empty conversation id")
	}
	return created.ConversationID, nil
}

func (c *proxyClient) send(ctx context.Context, conversationID, prompt string) (string, error) {
	raw, err := c.call(ctx, "POST", "/conversations/"+conversationID+"/messages", map[string]string{"content": prompt})
	if err != nil {
		return "", err
	}
	var sent struct {
		MessageID string `json:"message_id"`
	}
	if err := json.Unmarshal(raw, &sent); err != nil {
		return "", fmt.Errorf("drive: decode message: %w", err)
	}
	return sent.MessageID, nil
}

// liveTrace is the turn the driver waits for.
type liveTrace struct {
	id    string
	reply string
	cost  float64
	at    time.Time
	spans []pilot.ToolSpan
}

// waitForTrace polls the trace list until a trace for the conversation
// appears, then reads its detail. It matches on conversation id, never on
// recency.
func waitForTrace(ctx context.Context, agentName, conversationID string, after time.Time) (liveTrace, error) {
	deadline := time.Now().Add(6 * time.Minute)
	failures := 0
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return liveTrace{}, fmt.Errorf("drive: wait: %w", err)
		}
		ids, err := recentTraceIDs(agentName, after)
		if err != nil {
			failures++
			fmt.Fprintf(os.Stderr, "drive: trace poll failed (%d): %v\n", failures, err)
			if failures > 10 {
				return liveTrace{}, fmt.Errorf("drive: trace polling keeps failing: %w", err)
			}
			time.Sleep(10 * time.Second)
			continue
		}
		for _, id := range ids {
			trace, err := readTrace(agentName, id)
			if err != nil {
				continue
			}
			if trace.conversation == conversationID {
				return trace.native, nil
			}
		}
		time.Sleep(10 * time.Second)
	}
	return liveTrace{}, fmt.Errorf("drive: no trace for the conversation in time: %s", conversationID)
}

// tracedTurn carries the raw detail fields the driver needs.
type tracedTurn struct {
	native       liveTrace
	conversation string
}

func recentTraceIDs(agentName string, after time.Time) ([]string, error) {
	raw, err := astJSON("agent", "trace", "--name", agentName, "--limit", "8", "--json")
	if err != nil {
		return nil, err
	}
	var listed struct {
		Traces []struct {
			TraceID   string `json:"trace_id"`
			Timestamp string `json:"timestamp"`
		} `json:"traces"`
	}
	if err := json.Unmarshal(raw, &listed); err != nil {
		return nil, fmt.Errorf("drive: decode trace list: %w", err)
	}
	var ids []string
	for _, t := range listed.Traces {
		at, err := time.Parse(time.RFC3339, t.Timestamp)
		if err != nil || at.Before(after.Add(-time.Minute)) {
			continue
		}
		ids = append(ids, t.TraceID)
	}
	return ids, nil
}

func readTrace(agentName, id string) (tracedTurn, error) {
	raw, err := astJSON("agent", "trace", "--name", agentName, "-t", id, "--json")
	if err != nil {
		return tracedTurn{}, err
	}
	var detail struct {
		Trace struct {
			TraceID   string  `json:"trace_id"`
			Timestamp string  `json:"timestamp"`
			TotalCost float64 `json:"total_cost"`
			Input     string  `json:"input"`
			Output    struct {
				Text string `json:"text"`
			} `json:"output"`
			Metadata struct {
				Attributes map[string]string `json:"attributes"`
			} `json:"metadata"`
		} `json:"trace"`
		Observations []struct {
			Name      string `json:"name"`
			Type      string `json:"type"`
			LatencyMs int64  `json:"latency_ms"`
		} `json:"observations"`
	}
	if err := json.Unmarshal(raw, &detail); err != nil {
		return tracedTurn{}, fmt.Errorf("drive: decode trace %s: %w", id, err)
	}
	at, err := time.Parse(time.RFC3339, detail.Trace.Timestamp)
	if err != nil {
		return tracedTurn{}, fmt.Errorf("drive: decode trace time: %w", err)
	}
	spans := make([]pilot.ToolSpan, 0, len(detail.Observations))
	for _, o := range detail.Observations {
		spans = append(spans, pilot.ToolSpan{Name: o.Name, Type: o.Type, LatencyMs: o.LatencyMs})
	}
	return tracedTurn{
		native: liveTrace{
			id:    detail.Trace.TraceID,
			reply: detail.Trace.Output.Text,
			cost:  detail.Trace.TotalCost,
			at:    at,
			spans: spans,
		},
		conversation: detail.Trace.Metadata.Attributes["gen_ai.conversation.id"],
	}, nil
}

// astJSON runs the platform CLI and returns its stdout.
func astJSON(args ...string) ([]byte, error) {
	path, err := exec.LookPath("ast")
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return nil, fmt.Errorf("drive: find ast: %w", err)
		}
		path = filepath.Join(home, ".ast", "bin", "ast")
	}
	var stderr bytes.Buffer
	command := exec.Command(path, args...)
	command.Stderr = &stderr
	out, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("drive: ast %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

var billingUsage = regexp.MustCompile(`Usage spend:\s*\$([0-9.]+)`)

// refreshLogin shells out to `ast billing get`, which refreshes the CLI's
// short-lived token as a side effect, and returns the usage spend. The
// billing read is the spend guardrail: called before and after every run.
func refreshLogin(ctx context.Context) (float64, error) {
	path, err := exec.LookPath("ast")
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return 0, fmt.Errorf("drive: find ast: %w", err)
		}
		path = filepath.Join(home, ".ast", "bin", "ast")
	}
	out, err := exec.CommandContext(ctx, path, "billing", "get").Output()
	if err != nil {
		return 0, fmt.Errorf("drive: billing get: %w", err)
	}
	match := billingUsage.FindSubmatch(out)
	if match == nil {
		return 0, errors.New("drive: billing output carries no usage spend")
	}
	var usage float64
	if _, err := fmt.Sscanf(string(match[1]), "%f", &usage); err != nil {
		return 0, fmt.Errorf("drive: parse billing usage: %w", err)
	}
	return usage, nil
}

// scrubber replaces platform ids with the committed placeholders.
func scrubber(deploymentID, conversationID string, trace liveTrace) func(string) string {
	replacer := strings.NewReplacer(
		deploymentID, pilot.PlaceholderDeployment,
		conversationID, pilot.PlaceholderConversation,
		trace.id, pilot.PlaceholderTrace,
	)
	return replacer.Replace
}

// sizeQuote returns the reply line carrying the extracted size.
func sizeQuote(reply, size string) string {
	if size == "" {
		return ""
	}
	for _, line := range strings.Split(reply, "\n") {
		for _, field := range strings.Fields(line) {
			cleaned := strings.ToUpper(strings.Trim(field, ".,;:!?*\"'()[]{}"))
			if cleaned == size {
				return strings.TrimSpace(line)
			}
		}
	}
	return ""
}

// summarizeMemory keeps the arm memory fetch honest without duplicating
// the prompt: the response already sits inside the prompt verbatim for
// arms 1-2 (frozen histories), so the record carries its length and first
// line. Arm 3 carries the full live response instead (the freeze requires
// the complete run file to hold the request and the full response).
func summarizeMemory(response string) string {
	first := ""
	if lines := strings.Split(response, "\n"); len(lines) > 0 {
		first = strings.TrimSpace(lines[0])
	}
	return fmt.Sprintf("response %d chars, first line: %s", len(response), first)
}

// ---------------------------------------------------------------------------
// Driver.
// ---------------------------------------------------------------------------

type options struct {
	historiesDir string
	outDir       string
	schemaDir    string
	shoppers     []string
	arms         []int
	repeats      []int
	deploymentID string
	agentName    string
	lamboBin     string
	lamboWork    string
	sessionTag   string
}

func parseList(raw string) []string {
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func parseInts(raw string) ([]int, error) {
	out := []int{}
	for _, part := range parseList(raw) {
		var n int
		if _, err := fmt.Sscanf(part, "%d", &n); err != nil {
			return nil, fmt.Errorf("bad int %q", part)
		}
		out = append(out, n)
	}
	return out, nil
}

// committedSpend sums the trace costs of the run files already committed
// (or staged) in the out dir, so the cumulative cap holds across driver
// invocations and batches.
func committedSpend(outDir string) (float64, error) {
	paths, err := filepath.Glob(filepath.Join(outDir, "t55-*.json"))
	if err != nil {
		return 0, err
	}
	total := 0.0
	for _, path := range paths {
		if strings.HasSuffix(path, "scores.json") {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return 0, err
		}
		var record pilot.RunFile
		if err := json.Unmarshal(raw, &record); err != nil {
			return 0, fmt.Errorf("decode %s: %w", path, err)
		}
		total += record.TraceCostUSD
	}
	return total, nil
}

// brandForRecall picks the brand under consideration: the brand of the
// first history row carrying an item, the same candidate the harness
// recommends into.
func brandForRecall(history seed.History) (brandID, brandName string, err error) {
	for _, row := range history.Rows {
		if row.Item.BrandID != "" {
			return row.Item.BrandID, row.Item.BrandName, nil
		}
	}
	return "", "", fmt.Errorf("t55: history %s carries no brand", history.ShopperID)
}

func main() {
	historiesDir := flag.String("histories", "testdata/seed/histories", "arm-facing histories directory")
	outDir := flag.String("out", "bench/runs", "run file output directory")
	schemaDir := flag.String("schemas", "testdata/api", "frozen request schemas directory")
	shoppersFlag := flag.String("shoppers", "cmp-broken,cmp-never-arrived,cmp-changed-mind,cmp-size-changed,cmp-unclear", "comma-separated shopper ids")
	armsFlag := flag.String("arms", "1,2,3", "comma-separated arms")
	repeatsFlag := flag.String("repeats", "1,2,3", "comma-separated repeat numbers")
	deploymentID := flag.String("deployment-id", "", "platform deployment id of the neutral instrument")
	agentName := flag.String("agent-name", "pilot-baseline", "deployed agent name for trace reads")
	lamboBin := flag.String("lambo-bin", "", "lambo binary (default ~/work/lambo/target/release/lambo)")
	lamboWork := flag.String("lambo-work", "", "lambo working dir for configs and stores (default fresh temp dir)")
	sessionTag := flag.String("session-tag", "", "lambo session tag (default unix time)")
	flag.Parse()

	arms, err := parseInts(*armsFlag)
	if err != nil {
		fail(err)
	}
	repeats, err := parseInts(*repeatsFlag)
	if err != nil {
		fail(err)
	}
	opts := options{
		historiesDir: *historiesDir,
		outDir:       *outDir,
		schemaDir:    *schemaDir,
		shoppers:     parseList(*shoppersFlag),
		arms:         arms,
		repeats:      repeats,
		deploymentID: *deploymentID,
		agentName:    *agentName,
		lamboBin:     *lamboBin,
		lamboWork:    *lamboWork,
		sessionTag:   *sessionTag,
	}
	if opts.deploymentID == "" {
		fail(errors.New("t55: -deployment-id is required (the neutral instrument's deployment id)"))
	}
	if opts.lamboBin == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fail(err)
		}
		opts.lamboBin = filepath.Join(home, "work/lambo/target/release/lambo")
	}
	if opts.sessionTag == "" {
		opts.sessionTag = fmt.Sprintf("%d", time.Now().Unix())
	}
	if err := run(opts); err != nil {
		fail(err)
	}
}

func run(opts options) error {
	ctx := context.Background()
	if os.Getenv("ELASTIC_URL") == "" || os.Getenv("LAMBO_ES_API_KEY") == "" {
		return errors.New("t55: ELASTIC_URL and LAMBO_ES_API_KEY are empty: source tools/env.sh first (native embedder inference)")
	}
	if raw, err := exec.Command(opts.lamboBin, "--version").CombinedOutput(); err != nil {
		return fmt.Errorf("t55: lambo binary: %w: %s", err, strings.TrimSpace(string(raw)))
	} else {
		fmt.Printf("lambo: %s\n", strings.TrimSpace(string(raw)))
	}
	histories, err := harness.LoadHistories(opts.historiesDir)
	if err != nil {
		return err
	}
	byShopper := map[string]seed.History{}
	for _, h := range histories {
		byShopper[h.ShopperID] = h
	}
	spent, err := committedSpend(opts.outDir)
	if err != nil {
		return err
	}
	fmt.Printf("committed spend in %s: $%.6f of $%.2f budget\n", opts.outDir, spent, cumulativeBudgetUSD)

	work := opts.lamboWork
	if work == "" {
		var err error
		work, err = os.MkdirTemp("", "t55-lambo-")
		if err != nil {
			return fmt.Errorf("t55: temp lambo dir: %w", err)
		}
		defer os.RemoveAll(work)
	}

	client := &proxyClient{deploymentID: opts.deploymentID, http: &http.Client{Timeout: 60 * time.Second}}
	caps := harness.DefaultCaps()

	for _, shopperID := range opts.shoppers {
		history, ok := byShopper[shopperID]
		if !ok {
			return fmt.Errorf("t55: unknown shopper %q", shopperID)
		}
		if shopperID == "demo-shopper" {
			return errors.New("t55: the comparison scores cmp-* histories, never the demo shopper")
		}
		for _, armNum := range opts.arms {
			if armNum != 1 && armNum != 2 && armNum != 3 {
				return fmt.Errorf("t55: unknown arm %d", armNum)
			}
			arm := harness.Arm(armNum)
			var mem *arm3Memory
			var cleanup func()
			if armNum == 3 {
				var err error
				mem, cleanup, err = seedShopperMemory(opts, history)
				if err != nil {
					return err
				}
			}
			for _, repeat := range opts.repeats {
				runID := fmt.Sprintf("t55-a%d-%s-r%d", armNum, strings.TrimPrefix(shopperID, "cmp-"), repeat)
				cost, err := driveTurn(ctx, opts, client, caps, history, arm, runID, mem)
				if err != nil {
					if cleanup != nil {
						cleanup()
					}
					return err
				}
				spent += cost
				fmt.Printf("cumulative trace spend incl. committed: $%.6f\n", spent)
				if spent > cumulativeBudgetUSD {
					if cleanup != nil {
						cleanup()
					}
					return fmt.Errorf("t55: cumulative $%.6f exceeds the $%.2f budget after %s: stopping",
						spent, cumulativeBudgetUSD, runID)
				}
			}
			if cleanup != nil {
				cleanup()
			}
		}
	}
	fmt.Printf("t55: done, session trace spend this invocation within $%.2f budget\n", cumulativeBudgetUSD)
	return nil
}

// seedShopperMemory seeds a live Lambo session from one frozen history
// through the real write path and settles the daemon. The caller erases
// the session through the returned cleanup after the shopper's repeats.
func seedShopperMemory(opts options, history seed.History) (*arm3Memory, func(), error) {
	noop := func() {}
	s, err := buildSeed(history)
	if err != nil {
		return nil, noop, err
	}
	session := fmt.Sprintf("t55-%s-%s", history.ShopperID, opts.sessionTag)
	agent := "memento-t55"
	work, err := lamboWorkDir(opts, history.ShopperID)
	if err != nil {
		return nil, noop, err
	}
	cfgPath := filepath.Join(work, "lambo.toml")
	storePath := filepath.Join(work, "lambo.db")
	elasticURL := os.Getenv("ELASTIC_URL")
	config := "# T5.5 live config, generated by the driver (not committed).\n" +
		"[store]\nkind = \"sqlite\"\npath = \"" + storePath + "\"\n\n" +
		"[embedder]\nkind = \"elastic\"\nurl = \"" + elasticURL + "\"\n" +
		"model = \".jina-embeddings-v3\"\ndim = 1024\napi_key_env = \"LAMBO_ES_API_KEY\"\n" +
		"connect_timeout_ms = 30000\ntimeout_ms = 60000\ncanary = \"verify\"\n\n" +
		"# Cadence only, from the proven T2.6 config; the bars live in code.\n" +
		"[daemon]\ngc_interval = 1\ngc_idle_floor = 1\ngc_max_interval_secs = 3600\n" +
		"canonization_eval_interval_secs = 5\n"
	if err := os.MkdirAll(work, 0o755); err != nil {
		return nil, noop, fmt.Errorf("t55: lambo work dir: %w", err)
	}
	if err := os.WriteFile(cfgPath, []byte(config), 0o600); err != nil {
		return nil, noop, fmt.Errorf("t55: write lambo config: %w", err)
	}
	rpc, err := startLambo(opts.lamboBin, cfgPath, session, agent)
	if err != nil {
		return nil, noop, err
	}
	cleanup := func() {
		sessionRegistry.drop(history.ShopperID)
		rpc.stop()
		if text, err := eraseSession(opts.lamboBin, cfgPath, session); err != nil {
			fmt.Fprintf(os.Stderr, "t55: erase session %s: %s (%v)\n", session, text, err)
		} else {
			fmt.Printf("erased session %s: %s\n", session, text)
		}
	}
	graph := &lamboGraph{client: rpc, agent: agent}
	store, err := evidence.NewStore(t55Contract{kind: "t55-arm3"})
	if err != nil {
		cleanup()
		return nil, noop, fmt.Errorf("t55: open evidence store: %w", err)
	}
	svc, err := server.NewService(server.Deps{
		Payments:   closedPayments{},
		Classifier: closedClassifier{},
		Store:      store,
		Graph:      graph,
		SchemaDir:  opts.schemaDir,
	})
	if err != nil {
		cleanup()
		return nil, noop, fmt.Errorf("t55: new service: %w", err)
	}
	if err := svc.LoadSeed(context.Background(), s); err != nil {
		cleanup()
		return nil, noop, fmt.Errorf("t55: load seed for %s: %w", history.ShopperID, err)
	}
	note, err := settleBeliefs(graph, beliefContents(s))
	if err != nil {
		cleanup()
		return nil, noop, err
	}
	fmt.Printf("seeded %s: %d beliefs, %d pending; %s\n",
		history.ShopperID, len(s.Beliefs), len(s.Pending.Questions), note)
	sessionRegistry.store(history.ShopperID, svc)
	return &arm3Memory{settleNote: note}, cleanup, nil
}

// serveOnce serves one recall + pending-questions pair from a seeded
// service. Every repeat calls this fresh so each run file carries its own
// request and full response.
func serveOnce(svc *server.Service, history seed.History, traceID string) (*arm3Memory, error) {
	brandID, _, err := brandForRecall(history)
	if err != nil {
		return nil, err
	}
	recallReq := contract.RecallRequest{ShopperID: history.ShopperID, BrandID: brandID, TraceID: traceID}
	recallResp, err := svc.Recall(context.Background(), recallReq)
	if err != nil {
		return nil, fmt.Errorf("t55: recall for %s: %w", history.ShopperID, err)
	}
	pendingResp, err := svc.PendingQuestions(context.Background(), contract.PendingQuestionsRequest{ShopperID: history.ShopperID})
	if err != nil {
		return nil, fmt.Errorf("t55: pending questions for %s: %w", history.ShopperID, err)
	}
	return &arm3Memory{recallReq: recallReq, recallResp: recallResp, pendingResp: pendingResp}, nil
}

// lamboWorkDir returns the per-shopper lambo working directory.
func lamboWorkDir(opts options, shopper string) (string, error) {
	base := opts.lamboWork
	if base == "" {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "t55-"+opts.sessionTag+"-"+shopper)
	return dir, nil
}

// driveTurn runs one recommendation turn and commits its run file. It
// returns the turn's trace cost. Guardrails: the frozen per-run caps stop
// the turn before any model call when the prompt alone exceeds them; the
// cumulative budget stops the batch before a turn that could exceed it;
// the single-turn guard aborts everything when one trace costs too much.
func driveTurn(ctx context.Context, opts options, client *proxyClient, caps harness.Caps, history seed.History, arm harness.Arm, runID string, mem *arm3Memory) (float64, error) {
	var prompt string
	var armTool, armResp string
	if arm == harness.ArmMemento {
		if mem == nil {
			return 0, fmt.Errorf("t55: arm 3 turn %s has no seeded memory", runID)
		}
		served, err := serveFromSeed(opts, history, runID, mem)
		if err != nil {
			return 0, err
		}
		brandID, _, err := brandForRecall(history)
		if err != nil {
			return 0, err
		}
		block := renderMemoryBlock(history.ShopperID, brandID, *served)
		prompt = harness.BaseSection(history) + "\n" + block
		recallJSON, _ := json.Marshal(served.recallResp)
		pendingJSON, _ := json.Marshal(served.pendingResp)
		armTool = fmt.Sprintf("recall_for_purchase shopper_id=%s brand_id=%s trace_id=%s + pending_questions shopper_id=%s",
			history.ShopperID, brandID, runID, history.ShopperID)
		armResp = "recall_for_purchase response:\n" + string(recallJSON) + "\npending_questions response:\n" + string(pendingJSON)
	} else {
		built, toolCalls := harness.PromptFor(history, arm)
		prompt = built
		for _, call := range toolCalls {
			armTool = call.Name
			armResp = summarizeMemory(call.Response)
		}
	}
	promptTokens := harness.CountTokens(prompt)
	fmt.Printf("turn %s: prompt %d words, allows=%v\n", runID, promptTokens, caps.Allows(promptTokens))
	if err := os.MkdirAll(opts.outDir, 0o755); err != nil {
		return 0, fmt.Errorf("t55: mkdir %s: %w", opts.outDir, err)
	}
	path := filepath.Join(opts.outDir, runID+".json")
	if !caps.Allows(promptTokens) {
		record := cappedRecord(runID, history, arm, prompt, armTool, armResp, promptTokens)
		if err := writeRecord(path, record); err != nil {
			return 0, err
		}
		fmt.Printf("turn %s: capped before any model call, committed as marked\n", runID)
		return 0, nil
	}
	billingBefore, err := refreshLogin(ctx)
	if err != nil {
		return 0, err
	}
	fmt.Printf("billing before: usage $%.2f\n", billingBefore)
	conversationID, err := client.createConversation(ctx)
	if err != nil {
		return 0, err
	}
	fmt.Printf("conversation: %s\n", conversationID)
	sentAt := time.Now().UTC()
	if _, err := client.send(ctx, conversationID, prompt); err != nil {
		return 0, err
	}
	trace, err := waitForTrace(ctx, opts.agentName, conversationID, sentAt)
	if err != nil {
		return 0, err
	}
	billingAfter, err := refreshLogin(ctx)
	if err != nil {
		return 0, err
	}
	fmt.Printf("billing after: usage $%.2f\n", billingAfter)
	if trace.cost > singleTurnGuardUSD {
		record := completeRecord(runID, history, arm, prompt, armTool, armResp, nil, trace, sentAt, opts.deploymentID, conversationID)
		record.Capped = true
		record.Extraction.Notes = fmt.Sprintf("single-turn guard: trace cost $%.6f exceeds $%.2f; batch aborted per the brief", trace.cost, singleTurnGuardUSD)
		if err := writeRecord(path, record); err != nil {
			return trace.cost, err
		}
		return trace.cost, fmt.Errorf("t55: turn %s cost $%.6f over the $%.2f single-turn guard: committed as marked, aborting",
			runID, trace.cost, singleTurnGuardUSD)
	}
	record := completeRecord(runID, history, arm, prompt, armTool, armResp, memNotes(mem), trace, sentAt, opts.deploymentID, conversationID)
	if err := writeRecord(path, record); err != nil {
		return trace.cost, err
	}
	fmt.Printf("run file: %s\n", path)
	fmt.Printf("OPERATIONAL trace=%s conv=%s cost=%.6f billing-delta=%.2f\n",
		trace.id, conversationID, trace.cost, billingAfter-billingBefore)
	return trace.cost, nil
}

// serveFromSeed serves one fresh recall pair from the shopper's seeded
// live service. Every repeat calls this fresh so each run file carries
// its own request and full response.
func serveFromSeed(_ options, history seed.History, runID string, mem *arm3Memory) (*arm3Memory, error) {
	svc := sessionRegistry.lookup(history.ShopperID)
	if svc == nil {
		return nil, fmt.Errorf("t55: no live session for %s (seed first)", history.ShopperID)
	}
	served, err := serveOnce(svc, history, runID)
	if err != nil {
		return nil, err
	}
	served.settleNote = mem.settleNote
	return served, nil
}

// sessionRegistry threads the seeded live service from seeding to turns
// within one driver invocation. Sessions never cross invocations: every
// batch seeds fresh and erases after.
var sessionRegistry = &registry{m: map[string]*server.Service{}}

type registry struct {
	m map[string]*server.Service
}

func (r *registry) store(shopper string, svc *server.Service) {
	r.m[shopper] = svc
}

func (r *registry) lookup(shopper string) *server.Service {
	return r.m[shopper]
}

func (r *registry) drop(shopper string) {
	delete(r.m, shopper)
}

// memNotes carries the arm-3 seed provenance into the run record.
func memNotes(mem *arm3Memory) *string {
	if mem == nil || mem.settleNote == "" {
		return nil
	}
	note := "live layer seed, settled before recall: " + mem.settleNote
	return &note
}

// cappedRecord commits a turn stopped by the per-run caps before any model
// call: no reply, no spend, marked capped.
func cappedRecord(runID string, history seed.History, arm harness.Arm, prompt, armTool, armResp string, promptTokens int) pilot.RunFile {
	return pilot.RunFile{
		RunID:             runID,
		ShopperID:         history.ShopperID,
		Arm:               int(arm),
		ArmName:           arm.String(),
		Model:             pilot.Model,
		Deployment:        "pilot-baseline",
		Prompt:            prompt,
		ArmMemoryTool:     armTool,
		ArmMemoryResponse: armResp,
		AgentToolSpans:    []pilot.ToolSpan{},
		RecallObserved:    false,
		Reply:             "",
		Contaminated:      false,
		TraceCostUSD:      0,
		PromptWords:       promptTokens,
		ReplyWords:        0,
		Capped:            true,
		Extraction: pilot.Extraction{
			Size:     "",
			SizeRule: pilot.SizeNone,
		},
	}
}

// completeRecord assembles the committed record of one live turn.
func completeRecord(runID string, history seed.History, arm harness.Arm, prompt, armTool, armResp string, notes *string, trace liveTrace, sentAt time.Time, deploymentID, conversationID string) pilot.RunFile {
	scrub := scrubber(deploymentID, conversationID, trace)
	size, rule := pilot.ExtractSize(trace.reply)
	contaminated, hits := pilot.CheckContamination(trace.reply)
	recall := false
	for _, span := range trace.spans {
		if span.Name == "tool recall_for_purchase" {
			recall = true
		}
	}
	record := pilot.RunFile{
		RunID:             runID,
		ShopperID:         history.ShopperID,
		Arm:               int(arm),
		ArmName:           arm.String(),
		Model:             pilot.Model,
		Deployment:        "pilot-baseline",
		Prompt:            scrub(prompt),
		ArmMemoryTool:     armTool,
		ArmMemoryResponse: armResp,
		AgentToolSpans:    trace.spans,
		RecallObserved:    recall,
		Reply:             scrub(trace.reply),
		Contaminated:      contaminated,
		ContaminationHits: hits,
		TraceCostUSD:      trace.cost,
		PromptWords:       harness.CountTokens(prompt),
		ReplyWords:        harness.CountTokens(trace.reply),
		Capped:            false,
		SentAt:            sentAt.Format(time.RFC3339),
		TraceAt:           trace.at.Format(time.RFC3339),
		Extraction: pilot.Extraction{
			Size:      size,
			SizeRule:  rule,
			SizeQuote: sizeQuote(trace.reply, size),
			IDRefs:    pilot.ExtractIDRefs(history, trace.reply),
		},
	}
	if notes != nil {
		record.Extraction.Notes = *notes
	}
	return record
}

// writeRecord commits one run file.
func writeRecord(path string, record pilot.RunFile) error {
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("t55: encode run: %w", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("t55: write %s: %w", path, err)
	}
	return nil
}
