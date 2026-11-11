// Command t59 drives the T5.9 correction-round turns: every one of the 9
// comparison shoppers (the 5 frozen T5.5 shoppers plus the 4 hardened T5.8
// shoppers) through arms 1-3, one correction turn each (27 turns).
//
// The arm's first recommendation is NEVER re-run: it is reused byte for
// byte from the already-committed T5.5/T5.8 run file (r1 of each cell; all
// repeats in a cell agree on the effective first size). The correction
// always contradicts, by one uniform rule: where the arm's effective first
// size was L the shopper says "Actually I take M now", otherwise
// "Actually I take L now". Each run file records the rule firing.
//
// This is run-record scaffolding only, extending the T5.8 driver shape:
// prompts come from the frozen harness (base section) plus the reused
// first-turn prompt, extraction from the frozen pilot rule, the record
// carries the frozen grader score, and grading reuses the frozen grader.
// What this driver adds is the correction-turn transport: the uniform
// correction rule, the second-turn prompt construction, the in-code
// cumulative spend cap, and the arm-3 correction through the real
// record_correction path over a live Lambo session (the T2.4b/T2.6/T5.8
// driver pattern: real graph client, throwaway sessions erased after).
//
// Arm-3 correction honesty: the reference agent acts on recall, so the
// only belief a correction can be declared against is one the pre-turn
// recall served. Where the pre-correction recall serves a Venerable fit
// belief, the driver records the correction against it through the real
// RecordCorrection call, serves the post-correction recall, and asserts
// the decision-vs-belief rule (correction with precedence, belief kept:
// same standing, same evidence, contradicted with the override beside
// it). Where recall serves nothing, no record_correction call exists on
// the agent-visible surface, so the driver makes none and records why:
// the correction travels in conversation text only, exactly as it would
// through the production agent. Recording against an unserved
// below-Venerable state belief would be invisible to recall and is
// refused here, not invented.
//
// Second-turn topology: one new conversation per correction turn carrying
// a single user message (first prompt verbatim + previous reply quoted +
// correction + one re-ask). The first turns live only in the committed
// files, so there is no live conversation to continue; the single message
// keeps every correction turn self-contained and uniform across arms.
// Arms 1-2 reuse the first prompt byte for byte (asserted: the memory is
// intact by construction). Arm 3 rebuilds the memory block from the
// post-correction recall served live.
//
// New files only: bench/runs/t59-*.json plus bench/runs/t59-scores.json.
// Nothing frozen is touched; the freeze manifest gains no rows because no
// frozen input changes (correction texts live in this driver as the
// uniform rule, not in a data file).
//
// Live services touched: the Astropods messaging proxy (model turns) and
// the Lambo daemon locally with the native embedder (arm-3 memory +
// corrections). No PayPal or Elastic evidence reads: histories are frozen
// replay inputs. Keys travel only in process memory via tools/env.sh.
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

	"github.com/nrynss/memento-emptor/bench/grader"
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
// are the T5.8 brief's binding budget.
const (
	// cumulativeBudgetUSD is the hard ceiling across all T5.8 turns.
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
	fmt.Fprintf(os.Stderr, "t59: %v\n", err)
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
	return fakes.Classification{}, errors.New("t59: classifier must never be called during seeding")
}

// closedPayments refuses every payment read.
type closedPayments struct{}

// VerifyWebhook always fails: seeding replays frozen histories, not webhooks.
func (closedPayments) VerifyWebhook(_ context.Context, _ string) (fakes.WebhookEvent, error) {
	return fakes.WebhookEvent{}, errors.New("t59: payment source must never be read during seeding")
}

// GetOrder always fails.
func (closedPayments) GetOrder(_ context.Context, _ string) (fakes.OrderRecord, error) {
	return fakes.OrderRecord{}, errors.New("t59: payment source must never be read during seeding")
}

// GetRefund always fails.
func (closedPayments) GetRefund(_ context.Context, _ string) (fakes.RefundRecord, error) {
	return fakes.RefundRecord{}, errors.New("t59: payment source must never be read during seeding")
}

// GetDispute always fails.
func (closedPayments) GetDispute(_ context.Context, _ string) (fakes.DisputeRecord, error) {
	return fakes.DisputeRecord{}, errors.New("t59: payment source must never be read during seeding")
}

// t59Contract is the store's embedding contract. It carries no model name,
// dimensions, endpoint or key — only its kind, so the in-memory lexical
// store stands in for the live Elastic proof exactly as in T2.6.
type t59Contract struct{ kind string }

// EmbeddingKind names the contract family.
func (c t59Contract) EmbeddingKind() string { return c.kind }

// Validate reports whether the contract is usable.
func (c t59Contract) Validate() error {
	if c.kind == "" {
		return errors.New("t59: contract names no kind")
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
			"clientInfo": map[string]any{"name": "t59-driver", "version": "0.1.0"},
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
		return fmt.Errorf("t59: wait %s: %w", what, err)
	}
	text := textOf(resp)
	if strings.Contains(text, "embedder unavailable") || strings.Contains(text, "error sending request") {
		return fmt.Errorf("t59: embedder down after %s: %s", what, lastLine(text))
	}
	if !strings.Contains(text, "applied") {
		return fmt.Errorf("t59: receipt for %s never applied: %.200s", what, text)
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
		return "", fmt.Errorf("t59: decode receipt: %w", err)
	}
	if structured.Receipt == "" {
		return "", fmt.Errorf("t59: write issued no receipt")
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
		return fakes.BeliefReport{}, fmt.Errorf("t59: decode inspect: %w", err)
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
	// Both maps hold indexes into s.Beliefs, never pointers: later
	// appends may reallocate the slice, and a held pointer would keep
	// writing the stale backing array and silently drop evidence. The
	// hardened set files multi-evidence beliefs, so indexing is load
	// bearing here.
	fitByBrand := map[string]int{}
	sellerBySubject := map[string]int{}

	for _, row := range rows {
		if row.Signal == domain.SignalGeneralStatement {
			// Statements never enter the live path: no ingest reads
			// them, they derive at most, and recall never serves
			// them. Skipped, not seeded.
			continue
		}
		if row.Outcome == nil {
			return server.Seed{}, fmt.Errorf("t59: seed %s: day %.0f carries no outcome and no statement", history.ShopperID, row.Day)
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
				return server.Seed{}, fmt.Errorf("t59: seed %s: feedback note without fits-well: %q", history.ShopperID, row.Note)
			}
			evID := mintEv()
			idx, ok := fitByBrand[row.Item.BrandID]
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
				idx = len(s.Beliefs) - 1
				fitByBrand[row.Item.BrandID] = idx
			}
			counts := domain.BeliefKindFit
			s.Beliefs[idx].Evidence = append(s.Beliefs[idx].Evidence, contract.EvidenceView{
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
			s.Beliefs[idx].UpdatedAt = atStr(row.Day)
			s.Links[evID] = []string{s.Beliefs[idx].BeliefID}
			s.FeedbackVerdicts[evID] = contract.FeedbackFitsWell
		case domain.SignalClassifiedOutcome:
			if row.Reason == nil {
				// A classified row with no reason cannot exist
				// in the live path (ingest files a question;
				// the classifier always answers). Stop.
				return server.Seed{}, fmt.Errorf("t59: seed %s: classified row day %.0f carries no reason", history.ShopperID, row.Day)
			}
			counts := domain.Evidence{Reason: row.Reason}.CountsToward()
			switch counts {
			case domain.BeliefKindSeller:
				evID := mintEv()
				key := "seller-for-" + row.Item.BrandID
				idx, ok := sellerBySubject[key]
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
					idx = len(s.Beliefs) - 1
					sellerBySubject[key] = idx
				}
				kind := domain.BeliefKindSeller
				s.Beliefs[idx].Evidence = append(s.Beliefs[idx].Evidence, evidenceView(evID, row, &kind))
				s.Beliefs[idx].UpdatedAt = atStr(row.Day)
				s.Links[evID] = []string{s.Beliefs[idx].BeliefID}
			case domain.BeliefKindFit:
				want := row.Reason.Category.Direction()
				idx, ok := fitByBrand[row.Item.BrandID]
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
					idx = len(s.Beliefs) - 1
					fitByBrand[row.Item.BrandID] = idx
					evID := mintEv()
					kind := domain.BeliefKindFit
					s.Beliefs[idx].Evidence = append(s.Beliefs[idx].Evidence, evidenceView(evID, row, &kind))
					s.Links[evID] = []string{s.Beliefs[idx].BeliefID}
					continue
				}
				if domain.DirectionForValue(s.Beliefs[idx].Value) != want {
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
				s.Beliefs[idx].Evidence = append(s.Beliefs[idx].Evidence, evidenceView(evID, row, &kind))
				s.Beliefs[idx].UpdatedAt = atStr(row.Day)
				s.Links[evID] = []string{s.Beliefs[idx].BeliefID}
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
			return server.Seed{}, fmt.Errorf("t59: seed %s: unknown signal %q day %.0f", history.ShopperID, string(row.Signal), row.Day)
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
//
// The marshal below is the raw contract value, not the service's encode
// path, so it writes whatever the domain type says. domain.Subject gained
// its json tags in T7.6, which makes the subject read kind/id/name. The two
// committed T5.9 recordings that carry a belief,
// t59-a3-noisy-large.json and t59-a3-noisy-small.json, predate the tags and
// show Kind/ID/Name; they stay as recorded, and T7.7's record of that
// decision sits in bench/README.md.
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

// ---------------------------------------------------------------------------
// Driver.
// ---------------------------------------------------------------------------

type options struct {
	historiesDir string
	outDir       string
	schemaDir    string
	shoppers     []string
	arms         []int
	deploymentID string
	agentName    string
	lamboBin     string
	lamboWork    string
	sessionTag   string
	runsDir      string
	answersPath  string
}

// allShoppers is the full T5.9 set: the 5 frozen T5.5 shoppers plus the 4
// hardened T5.8 shoppers, in seed order.
var allShoppers = []string{
	"cmp-broken", "cmp-changed-mind", "cmp-never-arrived",
	"cmp-size-changed", "cmp-unclear",
	"cmp-noisy-small", "cmp-noisy-large",
	"cmp-brands-steady", "cmp-brands-split",
}

// sourceRunFile maps one shopper and arm to the already-committed first
// turn this correction round reuses. T5.5 cells live in bench/runs/ as
// t55 (3 repeats; r1 is the reused one — every repeat in a cell agrees on
// the effective first size). T5.8 cells live in bench/runs/t58/ as t58
// (1 repeat). Nothing is re-run: the first turns below are inputs.
func sourceRunFile(runsDir, shopper string, arm int) string {
	short := strings.TrimPrefix(shopper, "cmp-")
	switch shopper {
	case "cmp-noisy-small", "cmp-noisy-large", "cmp-brands-steady", "cmp-brands-split":
		return filepath.Join(runsDir, "t58", fmt.Sprintf("t58-a%d-%s-r1.json", arm, short))
	default:
		return filepath.Join(runsDir, fmt.Sprintf("t55-a%d-%s-r1.json", arm, short))
	}
}

// t59RunID names one correction turn: shopper short name and arm, no
// repeat (one correction turn per cell).
func t59RunID(shopper string, arm int) string {
	return fmt.Sprintf("t59-a%d-%s", arm, strings.TrimPrefix(shopper, "cmp-"))
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
// (or staged): the T5.9 turns in the out dir plus the frozen T5.5 turns
// and the T5.8 turns beside them, so the cumulative cap holds across
// driver invocations and batches against the whole comparison spend.
// Score sidecars (t59-scores.json, scores.json) carry no trace cost and
// are skipped by suffix, as are the pilot runs (a separate budget line).
func committedSpend(outDir string) (float64, error) {
	roots := []string{outDir, filepath.Dir(outDir)}
	hardened, err := filepath.Glob(filepath.Join(filepath.Dir(outDir), "t58", "t58-*.json"))
	if err != nil {
		return 0, err
	}
	paths := append([]string{}, hardened...)
	for _, root := range roots {
		for _, prefix := range []string{"t55-*.json", "t59-*.json"} {
			matched, err := filepath.Glob(filepath.Join(root, prefix))
			if err != nil {
				return 0, err
			}
			paths = append(paths, matched...)
		}
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
		// First-turn files carry one top-level trace cost. T59 files
		// carry the second turn's cost under `second` and repeat the
		// reused first turn's cost under `first_trace_cost_usd`: the
		// latter is already counted through its source file and is
		// never counted twice.
		var first struct {
			TraceCostUSD float64 `json:"trace_cost_usd"`
		}
		if err := json.Unmarshal(raw, &first); err != nil {
			return 0, fmt.Errorf("decode %s: %w", path, err)
		}
		var correction struct {
			Second struct {
				TraceCostUSD float64 `json:"trace_cost_usd"`
			} `json:"second"`
		}
		if err := json.Unmarshal(raw, &correction); err != nil {
			return 0, fmt.Errorf("decode %s: %w", path, err)
		}
		total += first.TraceCostUSD + correction.Second.TraceCostUSD
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
	return "", "", fmt.Errorf("t59: history %s carries no brand", history.ShopperID)
}

func main() {
	mode := flag.String("mode", "drive", "drive correction turns or grade them (drive|grade)")
	historiesDir := flag.String("histories", "testdata/seed/histories", "arm-facing histories directory")
	runsDir := flag.String("runs", "bench/runs", "committed first-turn runs directory (drive inputs)")
	outDir := flag.String("out", "bench/runs/t59", "correction-turn output directory")
	answersPath := flag.String("answers", "testdata/seed/hidden/answers.json", "hidden answers file (grade mode only)")
	schemaDir := flag.String("schemas", "testdata/api", "frozen request schemas directory")
	shoppersFlag := flag.String("shoppers", strings.Join(allShoppers, ","), "comma-separated shopper ids")
	armsFlag := flag.String("arms", "1,2,3", "comma-separated arms")
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
	opts := options{
		historiesDir: *historiesDir,
		outDir:       *outDir,
		schemaDir:    *schemaDir,
		shoppers:     parseList(*shoppersFlag),
		arms:         arms,
		deploymentID: *deploymentID,
		agentName:    *agentName,
		lamboBin:     *lamboBin,
		lamboWork:    *lamboWork,
		sessionTag:   *sessionTag,
		runsDir:      *runsDir,
		answersPath:  *answersPath,
	}
	switch *mode {
	case "drive":
		if opts.deploymentID == "" {
			fail(errors.New("t59: -deployment-id is required (the neutral instrument's deployment id)"))
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
	case "grade":
		if err := grade(opts); err != nil {
			fail(err)
		}
	default:
		fail(fmt.Errorf("t59: unknown -mode %q (drive|grade)", *mode))
	}
}

func run(opts options) error {
	ctx := context.Background()
	if os.Getenv("ELASTIC_URL") == "" || os.Getenv("LAMBO_ES_API_KEY") == "" {
		return errors.New("t59: ELASTIC_URL and LAMBO_ES_API_KEY are empty: source tools/env.sh first (native embedder inference)")
	}
	if raw, err := exec.Command(opts.lamboBin, "--version").CombinedOutput(); err != nil {
		return fmt.Errorf("t59: lambo binary: %w: %s", err, strings.TrimSpace(string(raw)))
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
	fmt.Printf("committed spend: $%.6f of $%.2f budget\n", spent, cumulativeBudgetUSD)

	work := opts.lamboWork
	if work == "" {
		var err error
		work, err = os.MkdirTemp("", "t59-lambo-")
		if err != nil {
			return fmt.Errorf("t59: temp lambo dir: %w", err)
		}
		defer os.RemoveAll(work)
	}

	client := &proxyClient{deploymentID: opts.deploymentID, http: &http.Client{Timeout: 60 * time.Second}}
	caps := harness.DefaultCaps()

	for _, shopperID := range opts.shoppers {
		history, ok := byShopper[shopperID]
		if !ok {
			return fmt.Errorf("t59: unknown shopper %q", shopperID)
		}
		if shopperID == "demo-shopper" {
			return errors.New("t59: the comparison scores cmp-* histories, never the demo shopper")
		}
		for _, armNum := range opts.arms {
			if armNum != 1 && armNum != 2 && armNum != 3 {
				return fmt.Errorf("t59: unknown arm %d", armNum)
			}
			arm := harness.Arm(armNum)
			source, err := loadSourceRun(opts.runsDir, shopperID, armNum)
			if err != nil {
				return err
			}
			runID := t59RunID(shopperID, armNum)
			cost, err := driveCorrectionTurn(ctx, opts, client, caps, history, arm, runID, source)
			if err != nil {
				return err
			}
			spent += cost
			fmt.Printf("cumulative trace spend incl. committed: $%.6f\n", spent)
			if spent > cumulativeBudgetUSD {
				return fmt.Errorf("t59: cumulative $%.6f exceeds the $%.2f budget after %s: stopping",
					spent, cumulativeBudgetUSD, runID)
			}
		}
	}
	fmt.Printf("t59: done, session trace spend this invocation within $%.2f budget\n", cumulativeBudgetUSD)
	return nil
}

// seedShopperMemory seeds a live Lambo session from one frozen history
// through the real write path and settles the daemon. The caller erases
// the session through the returned cleanup after the shopper's turn. It
// returns the session name for the run record.
func seedShopperMemory(opts options, history seed.History) (string, *arm3Memory, func(), error) {
	noop := func() {}
	s, err := buildSeed(history)
	if err != nil {
		return "", nil, noop, err
	}
	session := fmt.Sprintf("t59-%s-%s", history.ShopperID, opts.sessionTag)
	agent := "memento-t59"
	work, err := lamboWorkDir(opts, history.ShopperID)
	if err != nil {
		return "", nil, noop, err
	}
	cfgPath := filepath.Join(work, "lambo.toml")
	storePath := filepath.Join(work, "lambo.db")
	elasticURL := os.Getenv("ELASTIC_URL")
	config := "# T5.9 live config, generated by the driver (not committed).\n" +
		"[store]\nkind = \"sqlite\"\npath = \"" + storePath + "\"\n\n" +
		"[embedder]\nkind = \"elastic\"\nurl = \"" + elasticURL + "\"\n" +
		"model = \".jina-embeddings-v3\"\ndim = 1024\napi_key_env = \"LAMBO_ES_API_KEY\"\n" +
		"connect_timeout_ms = 30000\ntimeout_ms = 60000\ncanary = \"verify\"\n\n" +
		"# Cadence only, from the proven T2.6 config; the bars live in code.\n" +
		"[daemon]\ngc_interval = 1\ngc_idle_floor = 1\ngc_max_interval_secs = 3600\n" +
		"canonization_eval_interval_secs = 5\n"
	if err := os.MkdirAll(work, 0o755); err != nil {
		return "", nil, noop, fmt.Errorf("t59: lambo work dir: %w", err)
	}
	if err := os.WriteFile(cfgPath, []byte(config), 0o600); err != nil {
		return "", nil, noop, fmt.Errorf("t59: write lambo config: %w", err)
	}
	rpc, err := startLambo(opts.lamboBin, cfgPath, session, agent)
	if err != nil {
		return "", nil, noop, err
	}
	cleanup := func() {
		sessionRegistry.drop(history.ShopperID)
		rpc.stop()
		if text, err := eraseSession(opts.lamboBin, cfgPath, session); err != nil {
			fmt.Fprintf(os.Stderr, "t59: erase session %s: %s (%v)\n", session, text, err)
		} else {
			fmt.Printf("erased session %s: %s\n", session, text)
		}
	}
	graph := &lamboGraph{client: rpc, agent: agent}
	store, err := evidence.NewStore(t59Contract{kind: "t59-arm3"})
	if err != nil {
		cleanup()
		return "", nil, noop, fmt.Errorf("t59: open evidence store: %w", err)
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
		return "", nil, noop, fmt.Errorf("t59: new service: %w", err)
	}
	if err := svc.LoadSeed(context.Background(), s); err != nil {
		cleanup()
		return "", nil, noop, fmt.Errorf("t59: load seed for %s: %w", history.ShopperID, err)
	}
	note, err := settleBeliefs(graph, beliefContents(s))
	if err != nil {
		cleanup()
		return "", nil, noop, err
	}
	fmt.Printf("seeded %s: %d beliefs, %d pending; %s\n",
		history.ShopperID, len(s.Beliefs), len(s.Pending.Questions), note)
	sessionRegistry.store(history.ShopperID, svc)
	return session, &arm3Memory{settleNote: note}, cleanup, nil
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
		return nil, fmt.Errorf("t59: recall for %s: %w", history.ShopperID, err)
	}
	pendingResp, err := svc.PendingQuestions(context.Background(), contract.PendingQuestionsRequest{ShopperID: history.ShopperID})
	if err != nil {
		return nil, fmt.Errorf("t59: pending questions for %s: %w", history.ShopperID, err)
	}
	return &arm3Memory{recallReq: recallReq, recallResp: recallResp, pendingResp: pendingResp}, nil
}

// lamboWorkDir returns the per-shopper lambo working directory.
func lamboWorkDir(opts options, shopper string) (string, error) {
	base := opts.lamboWork
	if base == "" {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "t59-"+opts.sessionTag+"-"+shopper)
	return dir, nil
}

// correctionFor maps the arm's already-recorded effective first size to
// the shopper's correction, by the one uniform rule: the correction
// always contradicts. The default names L; only where the arm already
// recommended L does it name M instead. Refusals (empty) and S
// recommendations both take the default L, which contradicts either.
func correctionFor(firstEffective string) (text, want string) {
	if strings.EqualFold(strings.TrimSpace(firstEffective), "L") {
		return "Actually I take M now", "M"
	}
	return "Actually I take L now", "L"
}

// reAsk is the single uniform re-ask after the correction, the same words
// in every arm. It repeats the harness's cite-only instruction so the
// second turn is graded on the same terms as the first.
const reAsk = "Given that correction, what size do you recommend for the candidate? Cite only what you were given."

// loadSourceRun reads the already-committed first turn this correction
// round reuses. The file is a frozen T5.5/T5.8 pilot.RunFile; the first
// turn is never re-run.
func loadSourceRun(runsDir, shopper string, arm int) (pilot.RunFile, error) {
	path := sourceRunFile(runsDir, shopper, arm)
	raw, err := os.ReadFile(path)
	if err != nil {
		return pilot.RunFile{}, fmt.Errorf("t59: read source run %s: %w", path, err)
	}
	var source pilot.RunFile
	if err := json.Unmarshal(raw, &source); err != nil {
		return pilot.RunFile{}, fmt.Errorf("t59: decode source run %s: %w", path, err)
	}
	if source.Prompt == "" {
		return pilot.RunFile{}, fmt.Errorf("t59: source run %s carries no prompt", path)
	}
	if source.Contaminated {
		return pilot.RunFile{}, fmt.Errorf("t59: source run %s is contaminated, refusing to reuse", path)
	}
	return source, nil
}

// secondPromptPlain builds the arms-1-and-2 correction message: the first
// prompt verbatim (the arm's memory intact by construction), the previous
// reply quoted as the recommendation under correction, then the
// correction and one re-ask.
func secondPromptPlain(firstPrompt, firstReply, corrText string) string {
	var out strings.Builder
	out.WriteString(firstPrompt)
	out.WriteString("\n\nYour recommendation in reply to the above was:\n")
	out.WriteString(firstReply)
	out.WriteString("\n\nShopper correction: ")
	out.WriteString(corrText)
	out.WriteString("\n")
	out.WriteString(reAsk)
	return out.String()
}

// secondPromptArm3 builds the arm-3 correction message: the frozen base
// section plus the memory block rendered from the post-correction recall
// served live, then the same transcript tail as the other arms.
func secondPromptArm3(history seed.History, post arm3Memory, firstReply, corrText string) string {
	brandID, _, err := brandForRecall(history)
	if err != nil {
		return ""
	}
	var out strings.Builder
	out.WriteString(harness.BaseSection(history))
	out.WriteString("\n")
	out.WriteString(renderMemoryBlock(history.ShopperID, brandID, post))
	out.WriteString("\n\nYour recommendation in reply to the layer memory above was:\n")
	out.WriteString(firstReply)
	out.WriteString("\n\nShopper correction: ")
	out.WriteString(corrText)
	out.WriteString("\n")
	out.WriteString(reAsk)
	return out.String()
}

// T59First is the reused first turn, copied from the committed source run.
type T59First struct {
	// Prompt is the source run's prompt, verbatim.
	Prompt string `json:"prompt"`
	// Reply is the source run's reply, verbatim.
	Reply string `json:"reply"`
	// Size is the effective first size (human override else deterministic).
	Size string `json:"size"`
	// Cites are the effective first-turn cites.
	Cites []string `json:"cites"`
	// PromptWords and ReplyWords recount the reused turn.
	PromptWords int `json:"prompt_words"`
	// ReplyWords recounts the reused reply.
	ReplyWords int `json:"reply_words"`
	// TraceCostUSD is the source turn's live cost, counted toward the cap.
	TraceCostUSD float64 `json:"trace_cost_usd"`
}

// T59Correction is the one shopper correction and the uniform rule firing.
type T59Correction struct {
	// Text is the shopper's correction words.
	Text string `json:"text"`
	// WantSize is the size the correction names.
	WantSize string `json:"want_size"`
	// FirstEffective is the reused first size the rule read.
	FirstEffective string `json:"first_effective"`
	// Rule documents the uniform rule firing.
	Rule string `json:"rule"`
}

// T59Arm3 is the arm-3 live correction path, or why it did not run.
type T59Arm3 struct {
	// Session is the throwaway live Lambo session (erased after the turn).
	Session string `json:"session"`
	// SettleNote is the daemon settle report before any correction.
	SettleNote string `json:"settle_note"`
	// PreRecall is the full pre-correction recall + pending response.
	PreRecall contract.RecallResponse `json:"pre_recall"`
	// PrePending is the full pre-correction pending response.
	PrePending contract.PendingQuestionsResponse `json:"pre_pending"`
	// CorrectionRecorded is true when the real record_correction ran.
	CorrectionRecorded bool `json:"correction_recorded"`
	// NoBeliefReason explains a false above: recall served no belief, so
	// no ContradictsBeliefID exists on the agent-visible surface.
	NoBeliefReason string `json:"no_belief_reason,omitempty"`
	// RecordRequest is the live correction request, when recorded.
	RecordRequest *contract.RecordCorrectionRequest `json:"record_request,omitempty"`
	// RecordResponse is the live correction response, when recorded.
	RecordResponse *contract.RecordCorrectionResponse `json:"record_response,omitempty"`
	// PostRecall is the full post-correction recall response.
	PostRecall contract.RecallResponse `json:"post_recall"`
	// PostPending is the full post-correction pending response.
	PostPending contract.PendingQuestionsResponse `json:"post_pending"`
	// BeliefKept states the decision-vs-belief assertion the driver
	// proved (or why it is vacuous), recorded per turn.
	BeliefKept string `json:"belief_kept"`
}

// T59Second is the committed record of the one live correction turn.
type T59Second struct {
	// Prompt is the full correction message the run sent, verbatim.
	Prompt string `json:"prompt"`
	// ArmMemoryTool names the arm's memory handling for this turn.
	ArmMemoryTool string `json:"arm_memory_tool"`
	// ArmMemoryResponse carries the arm's memory evidence for this turn.
	ArmMemoryResponse string `json:"arm_memory_response"`
	// AgentToolSpans are the trace observations, in order.
	AgentToolSpans []pilot.ToolSpan `json:"agent_tool_spans"`
	// RecallObserved is true when the trace shows a recall_for_purchase span.
	RecallObserved bool `json:"recall_observed"`
	// Reply is the agent's correction-turn recommendation, verbatim.
	Reply string `json:"reply"`
	// Contaminated is true when the reply names demo-shopper facts.
	Contaminated bool `json:"contaminated"`
	// ContaminationHits are the demo tokens found in the reply.
	ContaminationHits []string `json:"contamination_hits,omitempty"`
	// ContaminationOverruled is true when the frozen check fired only on
	// tokens that appear verbatim in the run's own second prompt (a
	// self-quote of the live session's served ids, provably not
	// demo-shopper leakage), with the verification in ContaminationNote.
	// Set on human review in grade mode, never during the drive.
	ContaminationOverruled bool `json:"contamination_overruled,omitempty"`
	// ContaminationNote records the overrule verification.
	ContaminationNote string `json:"contamination_note,omitempty"`
	// TraceCostUSD is the platform cost of this turn.
	TraceCostUSD float64 `json:"trace_cost_usd"`
	// PromptWords and ReplyWords count the turn by the harness counter.
	PromptWords int `json:"prompt_words"`
	// ReplyWords counts the reply by the harness counter.
	ReplyWords int `json:"reply_words"`
	// Capped is true when the turn exceeded its caps and stopped.
	Capped bool `json:"capped"`
	// SentAt and TraceAt order the turn against the billing reads.
	SentAt  string `json:"sent_at"`
	TraceAt string `json:"trace_at"`
	// Extraction is the reply-to-grader reduction (deterministic now,
	// human fields set on review before grading).
	Extraction pilot.Extraction `json:"extraction"`
}

// T59Turn is the complete committed record of one correction turn: the
// reused first turn, the uniform correction, the arm-3 live path where it
// ran, the one live second turn, and (after grading) the frozen score.
type T59Turn struct {
	// RunID is the correction-turn label, such as t59-a2-broken.
	RunID string `json:"run_id"`
	// ShopperID names the comparison shopper.
	ShopperID string `json:"shopper_id"`
	// Arm is the harness arm number.
	Arm int `json:"arm"`
	// ArmName is the harness arm name.
	ArmName string `json:"arm_name"`
	// Model is the comparison model string.
	Model string `json:"model"`
	// Deployment is the neutral instrument deployment name.
	Deployment string `json:"deployment"`
	// SourceRunID is the committed first turn reused here.
	SourceRunID string `json:"source_run_id"`
	// First is the reused first turn.
	First T59First `json:"first"`
	// FirstTraceCostUSD repeats the reused turn's live cost for the cap.
	FirstTraceCostUSD float64 `json:"first_trace_cost_usd"`
	// Correction is the shopper correction and the rule firing.
	Correction T59Correction `json:"correction"`
	// Arm3 carries the live correction path; nil outside arm 3.
	Arm3 *T59Arm3 `json:"arm3,omitempty"`
	// Second is the live correction turn.
	Second T59Second `json:"second"`
	// Score is set by grade mode; nil before grading.
	Score *grader.Score `json:"score,omitempty"`
}

// driveCorrectionTurn runs one correction turn and commits its file. It
// returns the turn's trace cost. Guardrails: the frozen per-run caps stop
// the turn before any model call when the prompt alone exceeds them; the
// cumulative budget stops the batch before a turn that could exceed it;
// the single-turn guard aborts everything when one trace costs too much.
func driveCorrectionTurn(ctx context.Context, opts options, client *proxyClient, caps harness.Caps, history seed.History, arm harness.Arm, runID string, source pilot.RunFile) (float64, error) {
	firstEffective := pilot.EffectiveSize(source.Extraction.Size, source.Extraction.HumanSize)
	corrText, want := correctionFor(firstEffective)
	rule := fmt.Sprintf("first-effective %q → want %q: L-correction flips to M, everything else (M, S, refusal) takes L", firstEffective, want)
	correction := T59Correction{Text: corrText, WantSize: want, FirstEffective: firstEffective, Rule: rule}
	first := T59First{
		Prompt:       source.Prompt,
		Reply:        source.Reply,
		Size:         firstEffective,
		Cites:        source.Extraction.EffectiveCites(),
		PromptWords:  harness.CountTokens(source.Prompt),
		ReplyWords:   harness.CountTokens(source.Reply),
		TraceCostUSD: source.TraceCostUSD,
	}

	var prompt, armTool, armResp string
	var arm3 *T59Arm3
	cleanup, err := func() (func(), error) {
		noop := func() {}
		if arm != harness.ArmMemento {
			return noop, nil
		}
		session, mem, cleanup, err := seedShopperMemory(opts, history)
		if err != nil {
			return noop, err
		}
		served, err := recordCorrectionAgainstServed(history, runID, mem, corrText)
		if err != nil {
			cleanup()
			return noop, err
		}
		served.record.Session = session
		arm3 = served.record
		armTool = served.tool
		armResp = served.response
		prompt = secondPromptArm3(history, served.post, source.Reply, corrText)
		if prompt == "" {
			cleanup()
			return noop, fmt.Errorf("t59: arm 3 turn %s: no brand for recall", runID)
		}
		return cleanup, nil
	}()
	if err != nil {
		return 0, err
	}
	defer cleanup()
	if arm != harness.ArmMemento {
		if source.Prompt == "" {
			return 0, fmt.Errorf("t59: turn %s: source prompt is empty", runID)
		}
		prompt = secondPromptPlain(source.Prompt, source.Reply, corrText)
		if !strings.HasPrefix(prompt, source.Prompt) {
			return 0, fmt.Errorf("t59: turn %s: second prompt does not reuse the first prompt verbatim (arm memory must stay intact)", runID)
		}
		armTool = source.ArmMemoryTool + " (reused; no new fetch on the correction turn)"
		armResp = fmt.Sprintf("first-turn memory reused byte-for-byte (%d prompt words verified by prefix)", first.PromptWords)
	}
	promptTokens := harness.CountTokens(prompt)
	fmt.Printf("turn %s: prompt %d words, allows=%v\n", runID, promptTokens, caps.Allows(promptTokens))
	if err := os.MkdirAll(opts.outDir, 0o755); err != nil {
		return 0, fmt.Errorf("t59: mkdir %s: %w", opts.outDir, err)
	}
	path := filepath.Join(opts.outDir, runID+".json")
	record := &T59Turn{
		RunID: runID, ShopperID: history.ShopperID,
		Arm: int(arm), ArmName: arm.String(),
		Model: pilot.Model, Deployment: "pilot-baseline",
		SourceRunID: source.RunID, First: first,
		FirstTraceCostUSD: source.TraceCostUSD,
		Correction:        correction, Arm3: arm3,
	}
	if !caps.Allows(promptTokens) {
		record.Second = cappedSecond(prompt, armTool, armResp, promptTokens)
		if err := writeT59Record(path, record); err != nil {
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
		record.Second = completeSecond(prompt, armTool, armResp, trace, sentAt, opts.deploymentID, conversationID)
		record.Second.Capped = true
		record.Second.Extraction.Notes = fmt.Sprintf("single-turn guard: trace cost $%.6f exceeds $%.2f; batch aborted per the brief", trace.cost, singleTurnGuardUSD)
		if err := writeT59Record(path, record); err != nil {
			return trace.cost, err
		}
		return trace.cost, fmt.Errorf("t59: turn %s cost $%.6f over the $%.2f single-turn guard: committed as marked, aborting",
			runID, trace.cost, singleTurnGuardUSD)
	}
	record.Second = completeSecond(prompt, armTool, armResp, trace, sentAt, opts.deploymentID, conversationID)
	if err := writeT59Record(path, record); err != nil {
		return trace.cost, err
	}
	fmt.Printf("run file: %s\n", path)
	fmt.Printf("OPERATIONAL trace=%s conv=%s cost=%.6f billing-delta=%.2f\n",
		trace.id, conversationID, trace.cost, billingAfter-billingBefore)
	return trace.cost, nil
}

// arm3Served is the arm-3 live correction path for one turn: the record
// fragment, the tool/response log lines, and the post-correction memory
// the second-turn prompt renders.
type arm3Served struct {
	record   *T59Arm3
	tool     string
	response string
	post     arm3Memory
}

// recordCorrectionAgainstServed runs the arm-3 correction through the
// real record_correction path where the pre-correction recall serves a
// belief to declare against, and proves the decision-vs-belief rule on
// the post-correction recall. Where recall serves nothing, no
// record_correction call exists on the agent-visible surface, so none is
// made: the function records why instead of inventing a belief id.
func recordCorrectionAgainstServed(history seed.History, runID string, mem *arm3Memory, corrText string) (*arm3Served, error) {
	svc := sessionRegistry.lookup(history.ShopperID)
	if svc == nil {
		return nil, fmt.Errorf("t59: no live session for %s (seed first)", history.ShopperID)
	}
	pre, err := serveOnce(svc, history, runID+"-pre")
	if err != nil {
		return nil, err
	}
	pre.settleNote = mem.settleNote
	record := &T59Arm3{
		SettleNote: mem.settleNote,
		PreRecall:  pre.recallResp,
		PrePending: pre.pendingResp,
	}
	if len(pre.recallResp.Beliefs) == 0 {
		post, err := serveOnce(svc, history, runID+"-post")
		if err != nil {
			return nil, err
		}
		if len(post.recallResp.Beliefs) != 0 {
			return nil, fmt.Errorf("t59: turn %s: post-correction recall serves %d beliefs after a belief-free pre-recall",
				runID, len(post.recallResp.Beliefs))
		}
		record.CorrectionRecorded = false
		record.NoBeliefReason = "pre-correction recall served no belief for the brand, so no ContradictsBeliefID exists on the agent-visible surface; no record_correction call (nothing to declare against); the correction travels in conversation text only"
		record.PostRecall = post.recallResp
		record.PostPending = post.pendingResp
		record.BeliefKept = "vacuous: no served belief before or after; no graph write, nothing dropped"
		return &arm3Served{record: record, tool: toolLineNoBelief(history), response: responseNoBelief(pre, post), post: *post}, nil
	}
	beliefID := pre.recallResp.Beliefs[0].BeliefID
	preEvidence := len(pre.recallResp.Beliefs[0].Evidence)
	req := contract.RecordCorrectionRequest{
		ShopperID: history.ShopperID, Text: corrText,
		ContradictsBeliefID: beliefID, TraceID: runID,
	}
	resp, err := svc.RecordCorrection(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("t59: turn %s: record correction: %w", runID, err)
	}
	if !resp.Contradicted || resp.BeliefID != beliefID {
		return nil, fmt.Errorf("t59: turn %s: correction response contradicts=%v belief=%q, want true %q",
			runID, resp.Contradicted, resp.BeliefID, beliefID)
	}
	if resp.Standing != domain.GraphVenerable {
		return nil, fmt.Errorf("t59: turn %s: contradicted belief standing=%q, want venerable (the belief stays)",
			runID, string(resp.Standing))
	}
	post, err := serveOnce(svc, history, runID+"-post")
	if err != nil {
		return nil, err
	}
	kept, err := assertBeliefKept(pre, post, resp, preEvidence)
	if err != nil {
		return nil, fmt.Errorf("t59: turn %s: %w", runID, err)
	}
	record.CorrectionRecorded = true
	reqCopy, respCopy := req, resp
	record.RecordRequest = &reqCopy
	record.RecordResponse = &respCopy
	record.PostRecall = post.recallResp
	record.PostPending = post.pendingResp
	record.BeliefKept = kept
	return &arm3Served{record: record, tool: toolLineRecorded(history, runID, beliefID), response: responseRecorded(pre, post, req, resp), post: *post}, nil
}

// assertBeliefKept proves the decision-vs-belief rule on the
// post-correction recall: the prior belief is retained with the same
// standing and evidence, contradicted with the recorded override beside
// it, and the guidance follows the correction.
func assertBeliefKept(pre, post *arm3Memory, resp contract.RecordCorrectionResponse, preEvidence int) (string, error) {
	beliefID := resp.BeliefID
	var served *contract.BeliefView
	for i := range post.recallResp.Beliefs {
		if post.recallResp.Beliefs[i].BeliefID == beliefID {
			served = &post.recallResp.Beliefs[i]
		}
	}
	if served == nil {
		return "", fmt.Errorf("post-correction recall dropped belief %q (the belief stays)", beliefID)
	}
	if served.Standing != domain.GraphVenerable {
		return "", fmt.Errorf("post-correction belief %q standing=%q, want venerable", beliefID, string(served.Standing))
	}
	if !served.Contradicted || served.ContradictedBy == nil {
		return "", fmt.Errorf("post-correction belief %q is not shown contradicted", beliefID)
	}
	if served.ContradictedBy.CorrectionID != resp.Correction.CorrectionID {
		return "", fmt.Errorf("post-correction belief %q override %q, want recorded %q",
			beliefID, served.ContradictedBy.CorrectionID, resp.Correction.CorrectionID)
	}
	if len(served.Evidence) != preEvidence {
		return "", fmt.Errorf("post-correction belief %q evidence rows=%d, want the pre-correction %d",
			beliefID, len(served.Evidence), preEvidence)
	}
	overriding := post.recallResp.OverridingCorrection
	if overriding == nil || overriding.CorrectionID != resp.Correction.CorrectionID {
		return "", errors.New("post-correction recall carries no overriding correction with precedence")
	}
	if !overriding.TakesPrecedence {
		return "", errors.New("post-correction overriding correction does not take precedence")
	}
	if post.recallResp.Guidance.Basis != contract.GuidanceCorrection {
		return "", fmt.Errorf("post-correction guidance basis=%q, want correction", string(post.recallResp.Guidance.Basis))
	}
	_ = pre
	return fmt.Sprintf("belief %s kept: standing venerable, %d evidence rows, contradicted with override %s (precedence), guidance basis correction",
		beliefID, len(served.Evidence), overriding.CorrectionID), nil
}

// toolLineRecorded names the arm-3 memory fetch for a recorded correction.
func toolLineRecorded(history seed.History, runID, beliefID string) string {
	brandID, _, _ := brandForRecall(history)
	return fmt.Sprintf("recall_for_purchase shopper_id=%s brand_id=%s trace_id=%s-pre + record_correction shopper_id=%s contradicts=%s trace_id=%s + recall_for_purchase shopper_id=%s brand_id=%s trace_id=%s-post + pending_questions shopper_id=%s",
		history.ShopperID, brandID, runID, history.ShopperID, beliefID, runID, history.ShopperID, brandID, runID, history.ShopperID)
}

// toolLineNoBelief names the arm-3 memory handling where no correction ran.
func toolLineNoBelief(history seed.History) string {
	brandID, _, _ := brandForRecall(history)
	return fmt.Sprintf("recall_for_purchase shopper_id=%s brand_id=%s (served no belief: no record_correction call) + pending_questions shopper_id=%s",
		history.ShopperID, brandID, history.ShopperID)
}

// responseRecorded carries the full live responses for a recorded correction.
func responseRecorded(pre, post *arm3Memory, req contract.RecordCorrectionRequest, resp contract.RecordCorrectionResponse) string {
	reqJSON, _ := json.Marshal(req)
	respJSON, _ := json.Marshal(resp)
	preJSON, _ := json.Marshal(pre.recallResp)
	postJSON, _ := json.Marshal(post.recallResp)
	return "pre-correction recall_for_purchase response:\n" + string(preJSON) +
		"\nrecord_correction request:\n" + string(reqJSON) +
		"\nrecord_correction response:\n" + string(respJSON) +
		"\npost-correction recall_for_purchase response:\n" + string(postJSON)
}

// responseNoBelief carries the full live responses where no correction ran.
func responseNoBelief(pre, post *arm3Memory) string {
	preJSON, _ := json.Marshal(pre.recallResp)
	postJSON, _ := json.Marshal(post.recallResp)
	return "pre-correction recall_for_purchase response (no belief served):\n" + string(preJSON) +
		"\npost-correction recall_for_purchase response (unchanged, no correction recorded):\n" + string(postJSON)
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

// cappedSecond commits a turn stopped by the per-run caps before any model
// call: no reply, no spend, marked capped.
func cappedSecond(prompt, armTool, armResp string, promptTokens int) T59Second {
	return T59Second{
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

// completeSecond assembles the committed record of one live correction turn.
func completeSecond(prompt, armTool, armResp string, trace liveTrace, sentAt time.Time, deploymentID, conversationID string) T59Second {
	scrub := scrubber(deploymentID, conversationID, trace)
	size, rule := pilot.ExtractSize(trace.reply)
	contaminated, hits := pilot.CheckContamination(trace.reply)
	recall := false
	for _, span := range trace.spans {
		if span.Name == "tool recall_for_purchase" {
			recall = true
		}
	}
	// History for deterministic ID-ref extraction is not needed here:
	// cites resolve at grade time against the frozen histories, and the
	// verbatim-id scan needs the rows. The driver records the reply; the
	// grade step fills IDRefs deterministically. Human refs are set on
	// review before grading.
	return T59Second{
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
		},
	}
}

// writeT59Record commits one correction-turn file.
func writeT59Record(path string, record *T59Turn) error {
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("t59: encode run: %w", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("t59: write %s: %w", path, err)
	}
	return nil
}

// T59ScoredRow is one graded correction turn in t59-scores.json, beside
// the run files. Shape follows the T5.5/T5.8 scores with the correction
// measures added.
type T59ScoredRow struct {
	RunID                  string  `json:"run_id"`
	ShopperID              string  `json:"shopper_id"`
	Arm                    int     `json:"arm"`
	AnswerRightSize        string  `json:"answer_right_size"`
	FirstSize              string  `json:"first_size"`
	RecommendedSecond      string  `json:"recommended_second"`
	RightSizeFirst         bool    `json:"right_size_first"`
	WrongCause             int     `json:"wrong_cause"`
	CorrectionHonoured     *bool   `json:"correction_honoured"`
	CorrectionRecorded     *bool   `json:"correction_recorded,omitempty"`
	BeliefKept             *bool   `json:"belief_kept,omitempty"`
	BeliefKeptNote         string  `json:"belief_kept_note,omitempty"`
	SupportedCites         int     `json:"supported_cites"`
	Cited                  int     `json:"cited"`
	TokensFirst            int     `json:"tokens_first"`
	TokensSecond           int     `json:"tokens_second"`
	OverCap                bool    `json:"over_cap"`
	Contaminated           bool    `json:"contaminated"`
	ContaminationOverruled bool    `json:"contamination_overruled,omitempty"`
	TraceCostUSD           float64 `json:"trace_cost_usd"`
	FirstTraceCostUSD      float64 `json:"first_trace_cost_usd"`
}

// overruleContamination decides whether a fired frozen contamination
// check is a self-quote false positive. It returns true only when every
// flagged hit appears verbatim in the run's own second prompt: a token
// the run was itself given cannot be leakage of another shopper's
// memory. Any hit outside the run's own prompt keeps the run discarded.
// The verification is returned as the recorded note.
func overruleContamination(turn T59Turn) (bool, string) {
	if len(turn.Second.ContaminationHits) == 0 {
		return false, ""
	}
	for _, hit := range turn.Second.ContaminationHits {
		if !strings.Contains(turn.Second.Prompt, hit) {
			return false, ""
		}
	}
	return true, fmt.Sprintf("frozen check fired on %q; each hit appears verbatim in the run's own second prompt memory block (self-quote of the live session's served correction id), so the reply read this shopper's memory, not the demo shopper's; graded with this recorded exception",
		strings.Join(turn.Second.ContaminationHits, ","))
}

// T59Scores is the committed grading summary beside the run files. It is
// a new file: the frozen scores.json files are inputs and are never
// touched.
type T59Scores struct {
	Runs []T59ScoredRow `json:"runs"`
}

// grade scores every t59-*.json correction turn with the frozen grader:
// the reused first turn plus the shopper correction plus the live second
// turn. It runs after the author has reviewed every reply and set the
// human extraction fields where the deterministic step needs judgement.
// Grade mode reads no key and spends nothing.
func grade(opts options) error {
	histories, err := harness.LoadHistories(opts.historiesDir)
	if err != nil {
		return err
	}
	byShopper := make(map[string]seed.History, len(histories))
	for _, h := range histories {
		byShopper[h.ShopperID] = h
	}
	answers, err := grader.LoadAnswers(opts.answersPath)
	if err != nil {
		return err
	}
	paths, err := filepath.Glob(filepath.Join(opts.outDir, "t59-*.json"))
	if err != nil {
		return fmt.Errorf("grade: list runs: %w", err)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return errors.New("grade: no t59 run files to score")
	}
	scores := T59Scores{Runs: []T59ScoredRow{}}
	for _, path := range paths {
		if strings.HasSuffix(path, "scores.json") {
			continue
		}
		row, err := gradeOneT59(path, byShopper, answers)
		if err != nil {
			return err
		}
		scores.Runs = append(scores.Runs, row)
	}
	raw, err := json.MarshalIndent(scores, "", "  ")
	if err != nil {
		return fmt.Errorf("grade: encode scores: %w", err)
	}
	out := filepath.Join(opts.outDir, "t59-scores.json")
	if err := os.WriteFile(out, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("grade: write %s: %w", out, err)
	}
	fmt.Printf("graded %d runs -> %s\n", len(scores.Runs), out)
	return nil
}

func gradeOneT59(path string, byShopper map[string]seed.History, answers map[string]seed.Answer) (T59ScoredRow, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return T59ScoredRow{}, fmt.Errorf("grade: read %s: %w", path, err)
	}
	var turn T59Turn
	if err := json.Unmarshal(raw, &turn); err != nil {
		return T59ScoredRow{}, fmt.Errorf("grade: decode %s: %w", path, err)
	}
	if turn.Second.Contaminated {
		overruled, note := overruleContamination(turn)
		if !overruled {
			return T59ScoredRow{}, fmt.Errorf("t59: grade: %s second reply is contaminated: discard the run, never grade it", path)
		}
		turn.Second.ContaminationOverruled = true
		turn.Second.ContaminationNote = note
	}
	history, ok := byShopper[turn.ShopperID]
	if !ok {
		return T59ScoredRow{}, fmt.Errorf("grade: %s: unknown shopper %q", path, turn.ShopperID)
	}
	answer, ok := answers[turn.ShopperID]
	if !ok {
		return T59ScoredRow{}, fmt.Errorf("grade: %s: no hidden answer for %q", path, turn.ShopperID)
	}
	// Blindness: no run prompt may carry a hidden Why string. The check
	// covers both the reused first prompt and the live second prompt.
	for _, other := range answers {
		if other.Why != "" && (strings.Contains(turn.First.Prompt, other.Why) || strings.Contains(turn.Second.Prompt, other.Why)) {
			return T59ScoredRow{}, fmt.Errorf("%w: %s carries the rationale for %s", ErrBlindness, path, other.ShopperID)
		}
	}
	secondSize := pilot.EffectiveSize(turn.Second.Extraction.Size, turn.Second.Extraction.HumanSize)
	turn.Second.Extraction.IDRefs = pilot.ExtractIDRefs(history, turn.Second.Reply)
	score, err := grader.ScoreRun(grader.Run{
		ShopperID: turn.ShopperID,
		History:   history,
		Answer:    answer,
		First: grader.Turn{
			Prompt: turn.First.Prompt,
			Reply:  turn.First.Reply,
			Size:   turn.First.Size,
			Cites:  turn.First.Cites,
		},
		Correction: &grader.Correction{Text: turn.Correction.Text, WantSize: turn.Correction.WantSize},
		Second: &grader.Turn{
			Prompt: turn.Second.Prompt,
			Reply:  turn.Second.Reply,
			Size:   secondSize,
			Cites:  turn.Second.Extraction.EffectiveCites(),
		},
	})
	if err != nil {
		return T59ScoredRow{}, fmt.Errorf("grade: %s: %w", path, err)
	}
	turn.Score = &score
	encoded, err := json.MarshalIndent(turn, "", "  ")
	if err != nil {
		return T59ScoredRow{}, fmt.Errorf("grade: encode %s: %w", path, err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		return T59ScoredRow{}, fmt.Errorf("grade: write %s: %w", path, err)
	}
	row := T59ScoredRow{
		RunID:                  turn.RunID,
		ShopperID:              turn.ShopperID,
		Arm:                    turn.Arm,
		AnswerRightSize:        answer.RightSize,
		FirstSize:              turn.First.Size,
		RecommendedSecond:      secondSize,
		RightSizeFirst:         score.RightSize,
		WrongCause:             score.WrongCause,
		CorrectionHonoured:     score.CorrectionHonoured,
		SupportedCites:         score.Supported,
		Cited:                  score.Cited,
		TokensFirst:            score.TokensFirst,
		TokensSecond:           score.TokensSecond,
		OverCap:                score.OverCap,
		Contaminated:           turn.Second.Contaminated,
		ContaminationOverruled: turn.Second.ContaminationOverruled,
		TraceCostUSD:           turn.Second.TraceCostUSD,
		FirstTraceCostUSD:      turn.First.TraceCostUSD,
	}
	if turn.Arm3 != nil {
		recorded := turn.Arm3.CorrectionRecorded
		row.CorrectionRecorded = &recorded
		row.BeliefKeptNote = turn.Arm3.BeliefKept
		if recorded {
			// Belief-kept is proved by the driver's live assertion,
			// recorded per turn; the grade carries it, not re-proves it.
			kept := true
			row.BeliefKept = &kept
		}
	}
	return row, nil
}

// ErrBlindness is returned when a run prompt carries hidden answer material.
var ErrBlindness = errors.New("grade: run prompt carries hidden answer material")
