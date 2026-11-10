// Command seed plants the demo shopper's live graph on first boot and
// then exits: the same arc the T2.4b live leg proved (three fit returns
// take the sizing belief to Venerable, a non-arrival dispute stays
// seller-only, the correction declares against the belief without
// deleting it). It is idempotent through a marker file on the disk: a
// second boot with the marker present seeds nothing. Writes speak the
// stdio MCP dialect the release binary serves; event times anchor the
// seeded history the way the proven runs do.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// The seeded contents: demo fiction, identical in shape to the proven
// live leg. Short and keyed, so evidence cites the belief and the
// correction declares against it.
const (
	belief     = "Brand X oxford runs small for this shopper: take size L"
	assertion  = "Shopper says they are usually size M"
	kept1      = "Kept past the window: Brand Y tee, no feedback"
	kept2      = "Kept past the window: Brand Z cap, no feedback"
	ret1       = "Fit return 1: Brand X oxford size M refunded; too tight in the shoulders"
	ret2       = "Fit return 2: Brand X oxford size M refunded; too tight across the chest"
	ret3       = "Fit return 3: Brand X oxford size M refunded; sleeves too short in M"
	seller     = "The seller ships reliably for this shopper"
	dispute    = "Dispute opened: Brand X parcel never arrived; carrier lost it"
	correction = "Shopper correction: lost weight, take size M for Brand X"
	recallQ    = "Brand X oxford sizing for this shopper"
	probe      = "Seed probe concept: the embedder answers"
)

// waitMS is the receipt wait per write, from the proven driver. The
// daemon evaluates on its own cadence, so convergence polls separately.
const waitMS = 34000

// convergeTimeout bounds the promotion poll. Seeded edges join fast;
// the daemon still needs its eval cycles.
const convergeTimeout = 6 * time.Minute

// errEmbedderDown is the sentinel for a store the network cannot embed
// through. The caller falls back to the spare contract; anything else
// fails the boot seed loudly.
var errEmbedderDown = fmt.Errorf("seed: embedder unreachable from this host")

// client speaks one stdio MCP session: sequential ids, one readline
// per response. Notifications send no id and read nothing back.
type client struct {
	agent string
	seq   int
	in    io.WriteCloser
	scan  *bufio.Scanner
}

func main() {
	lamboBin := flag.String("lambo", "/app/bin/lambo", "lambo release binary")
	config := flag.String("config", "", "lambo.toml written at boot (required)")
	session := flag.String("session", "demo-shopper", "session to seed")
	agent := flag.String("agent", "memento-seed", "agent id the seed writes under")
	marker := flag.String("marker", "", "marker file proving a finished seed (required)")
	flag.Parse()

	if *config == "" || *marker == "" {
		log.Fatal("seed: -config and -marker are required")
	}
	if _, err := os.Stat(*marker); err == nil {
		log.Printf("seed: marker %s present, nothing to do", *marker)
		return
	}
	if up, err := probeOnly(*lamboBin, *config, *session+"-probe", *agent); err != nil {
		log.Printf("seed: probe: %v", err)
		os.Exit(1)
	} else if !up {
		log.Printf("seed: %v", errEmbedderDown)
		os.Exit(3)
	}
	if err := run(*lamboBin, *config, *session, *agent); err != nil {
		if strings.Contains(err.Error(), errEmbedderDown.Error()) {
			log.Printf("seed: %v", err)
			os.Exit(3)
		}
		log.Printf("seed: %v", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*marker, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o644); err != nil {
		log.Printf("seed: write marker: %v", err)
		os.Exit(1)
	}
	log.Printf("seed: finished, marker %s written", *marker)
}

func run(lamboBin, config, session, agent string) error {
	proc := exec.Command(lamboBin, "--config", config, "serve", "--session", session, "--agent", agent)
	stdin, err := proc.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := proc.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	proc.Stderr = os.Stderr
	if err := proc.Start(); err != nil {
		return fmt.Errorf("start serve: %w", err)
	}
	defer func() {
		_ = proc.Wait()
	}()
	client := &client{agent: agent, seq: 0, in: stdin, scan: bufio.NewScanner(stdout)}
	client.scan.Buffer(make([]byte, 1<<20), 1<<24)

	finish := func(fail error) error {
		_ = client.in.Close()
		if fail != nil {
			return fail
		}
		return nil
	}

	if err := client.handshake(); err != nil {
		return finish(fmt.Errorf("handshake: %w", err))
	}

	anchor := time.Now().UTC().Add(-11 * 24 * time.Hour)
	at := func(day int) string {
		return anchor.Add(time.Duration(day) * 24 * time.Hour).Format(time.RFC3339Nano)
	}

	steps := []struct {
		what    string
		derive  bool
		content string
		kind    string
		depends []string
		against []string
		when    string
	}{
		{"sizing belief", true, belief, "logic", nil, nil, at(0)},
		{"general statement", true, assertion, "observation", nil, nil, at(0)},
		{"kept Brand Y", true, kept1, "observation", nil, nil, at(1)},
		{"kept Brand Z", true, kept2, "observation", nil, nil, at(2)},
		{"fit return 1", false, ret1, "", []string{belief}, nil, at(3)},
		{"fit return 2", false, ret2, "", []string{belief}, nil, at(6)},
		{"fit return 3", false, ret3, "", []string{belief}, nil, at(9)},
	}
	for _, step := range steps {
		var receipt string
		var err error
		if step.derive {
			receipt, err = client.derive(step.content, step.kind, step.when)
		} else {
			receipt, err = client.recordAction(step.content, step.depends, nil, step.when)
		}
		if err != nil {
			return finish(fmt.Errorf("%s: %w", step.what, err))
		}
		if err := client.waitApplied(receipt, step.what); err != nil {
			return finish(err)
		}
	}

	if err := client.waitFor(belief, "[Logic, venerable]"); err != nil {
		return finish(fmt.Errorf("promotion: %w", err))
	}
	log.Print("seed: sizing belief is Venerable")

	if sellerReceipt, err := client.derive(seller, "logic", at(11)); err != nil {
		return finish(fmt.Errorf("seller belief: %w", err))
	} else if err := client.waitApplied(sellerReceipt, "seller belief"); err != nil {
		return finish(err)
	}
	disputeReceipt, err := client.recordAction(dispute, []string{seller}, nil, at(11))
	if err != nil {
		return finish(fmt.Errorf("dispute: %w", err))
	}
	if err := client.waitApplied(disputeReceipt, "dispute"); err != nil {
		return finish(err)
	}

	correctionReceipt, err := client.recordAction(correction, nil, []string{belief}, "")
	if err != nil {
		return finish(fmt.Errorf("correction: %w", err))
	}
	if err := client.waitApplied(correctionReceipt, "correction"); err != nil {
		return finish(err)
	}

	seen, err := client.inspect(belief, 2)
	if err != nil {
		return finish(fmt.Errorf("final inspect: %w", err))
	}
	if !strings.Contains(seen, "[Logic, venerable]") || !strings.Contains(seen, "Contradicts") {
		return finish(fmt.Errorf("final inspect misses Venerable-plus-Contradicts: %.200s", seen))
	}
	recalled, err := client.recall(recallQ)
	if err != nil {
		return finish(fmt.Errorf("final recall: %w", err))
	}
	if !strings.Contains(recalled, "Contradicted") || !strings.Contains(recalled, belief) {
		return finish(fmt.Errorf("final recall misses the warned belief: %.200s", recalled))
	}
	log.Print("seed: correction declared, warning intact")
	return finish(nil)
}

// send writes one framed request.
func (c *client) send(message map[string]any) error {
	line, err := json.Marshal(message)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	if _, err := c.in.Write(line); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

// recv reads one response line into a generic map.
func (c *client) recv() (map[string]any, error) {
	if !c.scan.Scan() {
		if err := c.scan.Err(); err != nil {
			return nil, fmt.Errorf("read: %w", err)
		}
		return nil, fmt.Errorf("serve closed stdout mid-run")
	}
	var out map[string]any
	decoder := json.NewDecoder(bytes.NewReader(c.scan.Bytes()))
	decoder.UseNumber()
	if err := decoder.Decode(&out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return out, nil
}

// call sends one tools/call and returns the result object.
func (c *client) call(name string, args map[string]any) (map[string]any, error) {
	c.seq++
	if err := c.send(map[string]any{
		"jsonrpc": "2.0", "id": c.seq,
		"method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	}); err != nil {
		return nil, err
	}
	resp, err := c.recv()
	if err != nil {
		return nil, err
	}
	result, _ := resp["result"].(map[string]any)
	if result == nil {
		return nil, fmt.Errorf("%s: no result: %.200s", name, c.scan.Bytes())
	}
	return result, nil
}

// textOf concatenates a result's content texts.
func textOf(result map[string]any) string {
	var parts []string
	items, _ := result["content"].([]any)
	for _, item := range items {
		block, _ := item.(map[string]any)
		if block["type"] == "text" {
			text, _ := block["text"].(string)
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

// receiptOf reads the write receipt: structured content first, then a
// receipt field inside the first JSON text block.
func receiptOf(result map[string]any) string {
	if structured, _ := result["structuredContent"].(map[string]any); structured != nil {
		if receipt, _ := structured["receipt"].(string); receipt != "" {
			return receipt
		}
	}
	items, _ := result["content"].([]any)
	for _, item := range items {
		block, _ := item.(map[string]any)
		text, _ := block["text"].(string)
		var body map[string]any
		if err := json.Unmarshal([]byte(text), &body); err == nil {
			if receipt, _ := body["receipt"].(string); receipt != "" {
				return receipt
			}
		}
	}
	return ""
}

// handshake runs the MCP open: initialize, the initialized
// notification (no reply), and a tools/list to prove the surface.
func (c *client) handshake() error {
	c.seq = 1
	if err := c.send(map[string]any{
		"jsonrpc": "2.0", "id": 1,
		"method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05", "capabilities": map[string]any{},
			"clientInfo": map[string]any{"name": "memento-seed", "version": "0.1.0"},
		},
	}); err != nil {
		return err
	}
	if _, err := c.recv(); err != nil {
		return err
	}
	if err := c.send(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}); err != nil {
		return err
	}
	c.seq = 2
	if err := c.send(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list"}); err != nil {
		return err
	}
	if _, err := c.recv(); err != nil {
		return err
	}
	return nil
}

// derive writes one belief or statement, returning its receipt.
func (c *client) derive(content, kind, eventTime string) (string, error) {
	args := map[string]any{
		"agent_id": c.agent,
		"concepts": []any{map[string]any{"content": content, "concept_type": kind}},
	}
	if eventTime != "" {
		args["event_time"] = eventTime
	}
	result, err := c.call("lambo_derive", args)
	if err != nil {
		return "", err
	}
	receipt := receiptOf(result)
	if receipt == "" {
		return "", fmt.Errorf("derive issued no receipt: %.200s", textOf(result))
	}
	return receipt, nil
}

// recordAction writes one structural edge, returning its receipt.
func (c *client) recordAction(action string, dependsOn, against []string, eventTime string) (string, error) {
	args := map[string]any{"agent_id": c.agent, "action": action}
	if dependsOn != nil {
		args["depends_on"] = dependsOn
	}
	if against != nil {
		args["against"] = against
	}
	if eventTime != "" {
		args["event_time"] = eventTime
	}
	result, err := c.call("lambo_record_action", args)
	if err != nil {
		return "", err
	}
	receipt := receiptOf(result)
	if receipt == "" {
		return "", fmt.Errorf("record_action issued no receipt: %.200s", textOf(result))
	}
	return receipt, nil
}

// waitApplied waits for one receipt to apply. An unreachable embedder
// reports the sentinel so the caller can fall back; any other miss is
// a hard failure.
func (c *client) waitApplied(receipt, what string) error {
	result, err := c.call("lambo_stats", map[string]any{
		"agent_id": c.agent, "receipt": receipt, "wait_ms": waitMS,
	})
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	text := textOf(result)
	switch {
	case strings.Contains(text, "embedder unavailable") || strings.Contains(text, "error sending request"):
		return fmt.Errorf("%s: %w: %.200s", what, errEmbedderDown, text)
	case strings.Contains(text, "applied"):
		return nil
	default:
		return fmt.Errorf("%s: receipt never applied: %.200s", what, text)
	}
}

// probeOnly opens a scratch session, derives one concept and waits
// for its receipt. It reports false (not an error) when the embedder
// is down, so the caller can try the spare contract before failing.
// The scratch session stays in the store: sessions isolate shoppers,
// and one observation per boot is the cheapest honest probe.
func probeOnly(lamboBin, config, session, agent string) (bool, error) {
	proc := exec.Command(lamboBin, "--config", config, "serve", "--session", session, "--agent", agent)
	stdin, err := proc.StdinPipe()
	if err != nil {
		return false, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := proc.StdoutPipe()
	if err != nil {
		return false, fmt.Errorf("stdout pipe: %w", err)
	}
	proc.Stderr = os.Stderr
	if err := proc.Start(); err != nil {
		return false, fmt.Errorf("start serve: %w", err)
	}
	defer func() {
		_ = proc.Wait()
	}()
	probeClient := &client{agent: agent, in: stdin, scan: bufio.NewScanner(stdout)}
	probeClient.scan.Buffer(make([]byte, 1<<20), 1<<24)
	defer func() {
		_ = probeClient.in.Close()
	}()
	if err := probeClient.handshake(); err != nil {
		return false, fmt.Errorf("handshake: %w", err)
	}
	return probeClient.probeEmbedder()
}

// probeEmbedder derives one scratch concept and waits for its receipt.
// It reports false (not an error) when the embedder is down, so the
// caller can try the spare contract before failing.
func (c *client) probeEmbedder() (bool, error) {
	receipt, err := c.derive(probe, "observation", "")
	if err != nil {
		return false, fmt.Errorf("probe derive: %w", err)
	}
	if err := c.waitApplied(receipt, "probe"); err != nil {
		if strings.Contains(err.Error(), errEmbedderDown.Error()) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// inspect returns the neighbourhood text around one concept.
func (c *client) inspect(focus string, depth int) (string, error) {
	result, err := c.call("lambo_inspect", map[string]any{
		"agent_id": c.agent, "focus": focus, "depth": depth,
	})
	if err != nil {
		return "", err
	}
	return textOf(result), nil
}

// recall returns the recall text for one query.
func (c *client) recall(query string) (string, error) {
	result, err := c.call("lambo_recall", map[string]any{"agent_id": c.agent, "query": query})
	if err != nil {
		return "", err
	}
	return textOf(result), nil
}

// waitFor polls inspect until the focus text contains the wanted mark.
// Promotion is eventual: the daemon evaluates on its own cadence, so
// the verdict right after the last receipt can predate the flush.
func (c *client) waitFor(focus, want string) error {
	deadline := time.Now().Add(convergeTimeout)
	var seen string
	for time.Now().Before(deadline) {
		text, err := c.inspect(focus, 2)
		if err != nil {
			return err
		}
		seen = text
		if strings.Contains(text, want) {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timed out waiting for %q: %.200s", want, seen)
}
