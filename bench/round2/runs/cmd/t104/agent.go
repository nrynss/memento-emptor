// Model transport for the T10.4 driver: the platform messaging proxy, the
// billing read that doubles as the spend guardrail, the trace poll, and the
// scrubber. The mechanics follow bench/pilot/cmd/drive and
// bench/runs/cmd/t55 call for call; any divergence is a defect.
//
// The platform login stays the CLI's: the driver shells out to
// `ast billing get`, which refreshes the CLI's short-lived token and reports
// the account's usage spend, and reads the access token from the OS keychain
// into process memory. No token is printed, written to a file, or committed.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/bench/pilot"
)

// browserUA is required: the default scripting UA trips the edge bot check
// (the same constant the frozen pilot drive command uses).
const browserUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"

// proxyClient drives the platform messaging proxy for one deployment.
type proxyClient struct {
	deploymentID string
	http         *http.Client
}

// base is the deployment's messaging endpoint.
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

// call makes one proxy request and returns the body.
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

// createConversation opens a fresh conversation.
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

// send posts one user message.
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

// liveTrace is one answered turn as the platform records it.
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
	deadline := time.Now().Add(10 * time.Minute)
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
		time.Sleep(5 * time.Second)
	}
	return liveTrace{}, fmt.Errorf("drive: no trace for the conversation in time")
}

// tracedTurn carries the raw detail fields the driver needs.
type tracedTurn struct {
	native       liveTrace
	conversation string
}

// recentTraceIDs lists the deployment's recent traces.
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

// readTrace reads one trace's detail.
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
			Name      string  `json:"name"`
			Type      string  `json:"type"`
			LatencyMs float64 `json:"latency_ms"`
		} `json:"observations"`
	}
	if err := json.Unmarshal(raw, &detail); err != nil {
		return tracedTurn{}, fmt.Errorf("drive: decode trace: %w", err)
	}
	at, err := time.Parse(time.RFC3339, detail.Trace.Timestamp)
	if err != nil {
		return tracedTurn{}, fmt.Errorf("drive: decode trace time: %w", err)
	}
	spans := make([]pilot.ToolSpan, 0, len(detail.Observations))
	for _, o := range detail.Observations {
		spans = append(spans, pilot.ToolSpan{Name: o.Name, Type: o.Type, LatencyMs: int64(o.LatencyMs)})
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

// billingUsage matches the usage spend line.
var billingUsage = regexp.MustCompile(`Usage spend:\s*\$([0-9.]+)`)

// billingCredit matches the remaining credit line when the account reports
// one.
var billingCredit = regexp.MustCompile(`Credit left:\s*\$([0-9.]+)`)

// billing is the guardrail reading: usage spend so far and the credit the
// account has left, when it reports one.
type billing struct {
	usage  float64
	credit float64
	hasCr  bool
}

// refreshLogin shells out to `ast billing get`, which refreshes the CLI's
// short-lived token as a side effect, and returns the spend figures. The
// billing read is the spend guardrail: called before and after every turn.
func refreshLogin(ctx context.Context) (billing, error) {
	path, err := exec.LookPath("ast")
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return billing{}, fmt.Errorf("drive: find ast: %w", err)
		}
		path = filepath.Join(home, ".ast", "bin", "ast")
	}
	out, err := exec.CommandContext(ctx, path, "billing", "get").Output()
	if err != nil {
		return billing{}, fmt.Errorf("drive: billing get: %w", err)
	}
	match := billingUsage.FindSubmatch(out)
	if match == nil {
		return billing{}, errors.New("drive: billing output carries no usage spend")
	}
	var out1 billing
	if _, err := fmt.Sscanf(string(match[1]), "%f", &out1.usage); err != nil {
		return billing{}, fmt.Errorf("drive: parse billing usage: %w", err)
	}
	if credit := billingCredit.FindSubmatch(out); credit != nil {
		if _, err := fmt.Sscanf(string(credit[1]), "%f", &out1.credit); err == nil {
			out1.hasCr = true
		}
	}
	return out1, nil
}

// scrubber replaces platform ids with the committed placeholders. An empty
// id is skipped: strings.Replacer with an empty old string would insert the
// placeholder at every position of the text.
func scrubber(deploymentID, conversationID string, trace liveTrace) func(string) string {
	pairs := []string{}
	if deploymentID != "" {
		pairs = append(pairs, deploymentID, pilot.PlaceholderDeployment)
	}
	if conversationID != "" {
		pairs = append(pairs, conversationID, pilot.PlaceholderConversation)
	}
	if trace.id != "" {
		pairs = append(pairs, trace.id, pilot.PlaceholderTrace)
	}
	replacer := strings.NewReplacer(pairs...)
	return replacer.Replace
}
