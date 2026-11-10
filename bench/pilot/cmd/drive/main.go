// Command drive runs one pilot turn: it builds the harness prompt for a
// comparison shopper and arm, sends it as a single user message to the
// deployed reference agent through the platform messaging proxy, waits for
// the turn's trace, and writes the scrubbed run file.
//
// The platform login stays the CLI's: before each call the driver shells
// out to `ast billing get` (which refreshes the CLI's short-lived token
// and doubles as the spend guardrail read) and reads the fresh token from
// the OS keychain into memory. The token is never printed, never written
// to a file, and never enters the repo.
//
// Usage:
//
//	go run ./bench/pilot/cmd/drive -shopper cmp-broken -arm 1
//
// Run from bench/pilot so the default histories and out paths resolve.
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
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/bench/harness"
	"github.com/nrynss/memento-emptor/bench/pilot"
	"github.com/nrynss/memento-emptor/seed"
)

// ErrCapped is returned when the prompt alone exceeds the pilot caps.
var ErrCapped = errors.New("drive: run exceeds its caps before any model call")

// ErrNoTrace is returned when no trace for the conversation appears in time.
var ErrNoTrace = errors.New("drive: no trace for the conversation in time")

// browserUA is required: the default scripting UA trips the edge bot check.
const browserUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"

// pilotBudgetUSD is the binding total pilot spend cap across all runs.
const pilotBudgetUSD = 5.00

func main() {
	historiesDir := flag.String("histories", "../../testdata/seed/histories", "arm-facing histories directory")
	outDir := flag.String("out", "runs", "run file output directory")
	shopper := flag.String("shopper", "", "comparison shopper id, such as cmp-broken")
	armNum := flag.Int("arm", 0, "comparison arm: 1 or 2")
	deploymentID := flag.String("deployment-id", "", "platform deployment id of the reference agent")
	agentName := flag.String("agent-name", "memento-agent", "deployed agent name for trace reads")
	printOnly := flag.Bool("print-only", false, "print the prompt and caps, send nothing")
	flag.Parse()

	if err := run(*historiesDir, *outDir, *shopper, *armNum, *deploymentID, *agentName, *printOnly); err != nil {
		fmt.Fprintf(os.Stderr, "drive: %v\n", err)
		os.Exit(1)
	}
}

func run(historiesDir, outDir, shopper string, armNum int, deploymentID, agentName string, printOnly bool) error {
	ctx := context.Background()
	if shopper == "" || (armNum != 1 && armNum != 2) || deploymentID == "" {
		return errors.New("drive: -shopper, -arm 1|2 and -deployment-id are required")
	}
	if shopper == "demo-shopper" {
		return errors.New("drive: the pilot scores cmp-* histories, never the demo shopper")
	}
	histories, err := harness.LoadHistories(historiesDir)
	if err != nil {
		return err
	}
	var history *seed.History
	for i := range histories {
		if histories[i].ShopperID == shopper {
			history = &histories[i]
			break
		}
	}
	if history == nil {
		return fmt.Errorf("drive: unknown shopper %q", shopper)
	}
	arm := harness.Arm(armNum)
	prompt, calls := harness.PromptFor(*history, arm)
	caps := harness.DefaultCaps()
	promptTokens := harness.CountTokens(prompt)
	fmt.Printf("prompt: %d words, allows=%v\n", promptTokens, caps.Allows(promptTokens))
	if printOnly {
		fmt.Println("---- prompt ----")
		fmt.Println(prompt)
		return nil
	}
	if !caps.Allows(promptTokens) {
		return fmt.Errorf("%w: %d words", ErrCapped, promptTokens)
	}
	client := &proxyClient{deploymentID: deploymentID, http: &http.Client{Timeout: 60 * time.Second}}
	billingBefore, err := refreshLogin(ctx, agentName)
	if err != nil {
		return err
	}
	fmt.Printf("billing before: usage $%.2f\n", billingBefore)
	conversationID, err := client.createConversation(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("conversation: %s\n", conversationID)
	sentAt := time.Now().UTC()
	messageID, err := client.send(ctx, conversationID, prompt)
	if err != nil {
		return err
	}
	fmt.Printf("sent: message %s\n", messageID)
	trace, err := waitForTrace(ctx, agentName, conversationID, sentAt)
	if err != nil {
		return err
	}
	billingAfter, err := refreshLogin(ctx, agentName)
	if err != nil {
		return err
	}
	fmt.Printf("billing after: usage $%.2f\n", billingAfter)
	record := pilot.NewRunFile(
		fmt.Sprintf("pilot-a%d-%s", armNum, strings.TrimPrefix(shopper, "cmp-")),
		*history, arm, calls, trace.spans, prompt, trace.reply, trace.cost,
		sentAt.Format(time.RFC3339), trace.at.Format(time.RFC3339),
		scrubber(deploymentID, conversationID, trace),
	)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("drive: mkdir %s: %w", outDir, err)
	}
	path := filepath.Join(outDir, record.RunID+".json")
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("drive: encode run: %w", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("drive: write %s: %w", path, err)
	}
	fmt.Printf("run file: %s\n", path)
	fmt.Printf("OPERATIONAL trace=%s conv=%s cost=%.6f billing-delta=%.2f\n",
		trace.id, conversationID, trace.cost, billingAfter-billingBefore)
	return nil
}

// proxyClient drives the platform messaging proxy for one deployment.
type proxyClient struct {
	deploymentID string
	http         *http.Client
}

func (c *proxyClient) base() string {
	return "https://astropods.com/api/v1/deployments/" + c.deploymentID + "/messaging"
}

// token reads the CLI's fresh org access token from the OS keychain. The
// org id comes from the CLI's own config; the secret comes from the
// keychain and lives only in this process's memory.
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
// recency, so concurrent turns on the same deployment cannot cross.
func waitForTrace(ctx context.Context, agentName, conversationID string, after time.Time) (liveTrace, error) {
	deadline := time.Now().Add(6 * time.Minute)
	failures := 0
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return liveTrace{}, fmt.Errorf("drive: wait: %w", err)
		}
		ids, err := recentTraceIDs(agentName, after)
		if err != nil {
			// Transient CLI or platform errors must not lose the run:
			// the turn is already in flight and spending. Keep polling.
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
	return liveTrace{}, fmt.Errorf("%w: %s", ErrNoTrace, conversationID)
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

// astJSON runs the platform CLI and returns its stdout. Stderr is kept in
// the error so a failed poll says why instead of failing silently.
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
// billing read is the pilot's spend guardrail: call it before and after
// every run.
func refreshLogin(ctx context.Context, agentName string) (float64, error) {
	_ = agentName
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
