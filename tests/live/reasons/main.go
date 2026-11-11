// Live T2.2b driver: classifies the eleven labelled shopper answers
// through the real classifier service via the frozen client and compares
// category and confidence against the recorded T0.7 table.
//
// This is a main program, never a test: no *_test.go lives under
// tests/live/, so CI builds this package but never runs it and spends
// nothing. Run it once by hand with the key loaded:
//
//	source tools/env.sh
//	bash tests/live/reasons/run.sh
//
// Pass and fail. The run passes when every phrase reproduces the
// recorded category and the recorded ask-again verdict: those are the
// properties that gate beliefs. Confidence drift on a vague phrase is
// reported loudly but does not fail the run: vague answers sample
// nondeterministically, and the recorded research already warns the
// eleven-phrase set is a calibration sample, not proof. What must not
// move is the verdict, and the floor margin it rests on.
//
// The key travels only in the Authorization header. Raw replies land in
// out/ (git-ignored): they carry per-request generation ids, never the
// key. The transcript below is stdout; it names no secret.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/reasons"
)

// referenceCostPerCall is T0.7's measured per-call cost in USD. A live
// call costing ten times this trips the guard and stops the run.
const referenceCostPerCall = 0.0000214

// expectedBuild is the model string the endpoint resolved on the T0.7
// run. A different resolution re-opens the floor and stops the run.
const expectedBuild = "typesafe/jev-1.14-20261014"

// pauseBetweenCalls spaces the eleven calls the way T0.7 did: no
// retries were needed then, and none are expected now.
const pauseBetweenCalls = 2 * time.Second

// recording is one labelled fixture: the shopper's words plus the
// recorded choice and confidence the live run must reproduce.
type recording struct {
	file       string
	answer     string
	choice     string
	confidence float64
}

// loadRecordings reads every labelled fixture under dir in filename
// order, so the run follows the T0.7 table phrase 1 through 11.
func loadRecordings(dir string) ([]recording, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read fixtures: %w", err)
	}
	names := []string{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	out := []recording{}
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("read fixture %s: %w", name, err)
		}
		var doc struct {
			Request struct {
				State struct {
					Answer string `json:"answer"`
				} `json:"state"`
			} `json:"request"`
			Response struct {
				Answers struct {
					Reason struct {
						Choice     string  `json:"choice"`
						Confidence float64 `json:"confidence"`
					} `json:"reason"`
				} `json:"answers"`
			} `json:"response"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("parse fixture %s: %w", name, err)
		}
		if doc.Request.State.Answer == "" || doc.Response.Answers.Reason.Choice == "" {
			return nil, fmt.Errorf("fixture %s has no answer or choice", name)
		}
		out = append(out, recording{
			file:       name,
			answer:     doc.Request.State.Answer,
			choice:     doc.Response.Answers.Reason.Choice,
			confidence: doc.Response.Answers.Reason.Confidence,
		})
	}
	return out, nil
}

// liveReply is the billing and provenance surface of one live reply.
// The frozen client decodes the classification itself; this only
// records what the transcript and the guards need.
type liveReply struct {
	model      string
	cost       float64
	inputToken int
}

// captureDo sends through inner with the key attached, saves the raw
// reply body to out/, and reports the reply's model and usage. The
// body is re-wrapped so the frozen client decodes the same bytes.
func captureDo(inner func(*http.Request) (*http.Response, error), key, out string, seen *int, reply *liveReply) func(*http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		req.Header.Set("Authorization", "Bearer "+key)
		resp, err := inner(req)
		if err != nil {
			return nil, err
		}
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		*seen++
		var parsed struct {
			Model string `json:"model"`
			Usage struct {
				Cost        float64 `json:"cost"`
				InputTokens int     `json:"input_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			resp.Body = io.NopCloser(bytes.NewReader(raw))
			return resp, nil
		}
		*reply = liveReply{model: parsed.Model, cost: parsed.Usage.Cost, inputToken: parsed.Usage.InputTokens}
		if err := os.WriteFile(filepath.Join(out, fmt.Sprintf("%02d-response.json", *seen)), raw, 0o600); err != nil {
			return nil, err
		}
		resp.Body = io.NopCloser(bytes.NewReader(raw))
		return resp, nil
	}
}

func main() {
	fixtures := flag.String("fixtures", "testdata/jev", "directory holding the eleven labelled recordings")
	out := flag.String("out", "tests/live/reasons/out", "directory for raw live replies (git-ignored)")
	flag.Parse()

	if err := run(*fixtures, *out); err != nil {
		fmt.Fprintf(os.Stderr, "t22b-live: %v\n", err)
		os.Exit(1)
	}
}

func run(fixtures, out string) error {
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		return fmt.Errorf("OPENROUTER_API_KEY is empty: source tools/env.sh first")
	}
	want, err := loadRecordings(fixtures)
	if err != nil {
		return err
	}
	if len(want) != 11 {
		return fmt.Errorf("labelled set holds %d phrases, want 11", len(want))
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return fmt.Errorf("make out dir: %w", err)
	}

	httpClient := &http.Client{Timeout: 90 * time.Second}
	seen := 0
	var reply liveReply
	client := reasons.NewClient(reasons.DefaultModel, reasons.DefaultEndpoint,
		captureDo(httpClient.Do, key, out, &seen, &reply))

	var totalCost float64
	var totalTokens int
	mismatches := 0
	drift := 0
	lastModel := ""
	fmt.Printf("%-3s %-50s %-18s %-18s %-8s %-9s %-12s\n", "#", "answer", "recorded", "live", "conf", "ask-again", "cost-usd")
	for i, rec := range want {
		if i > 0 {
			time.Sleep(pauseBetweenCalls)
		}
		ctx, stop := context.WithTimeout(context.Background(), 2*time.Minute)
		class, err := client.Classify(ctx, rec.answer)
		stop()
		if err != nil {
			return fmt.Errorf("phrase %d %q: %w (after %d of 11 calls)", i+1, rec.answer, err, i)
		}
		totalCost += reply.cost
		totalTokens += reply.inputToken
		lastModel = reply.model

		// The spend guard runs on every call: a single expensive reply
		// stops the run before the next one is sent. The model guard
		// runs on the first call, where a version change re-opens the
		// floor; later drift is reported below as a mismatch.
		if reply.cost > 10*referenceCostPerCall {
			return fmt.Errorf("phrase %d cost %.9f USD, over 10x the %.7f reference: stopping per the guard",
				i+1, reply.cost, referenceCostPerCall)
		}
		if i == 0 {
			fmt.Printf("live model resolves to %s\n", reply.model)
			if reply.model != expectedBuild {
				return fmt.Errorf("model resolved to %s, want %s: version change re-opens the floor, stopping per the guard", reply.model, expectedBuild)
			}
		}
		if reply.model != expectedBuild {
			fmt.Printf("WARNING: phrase %d resolved model %s, want %s\n", i+1, reply.model, expectedBuild)
			mismatches++
		}

		recorded := fakes.Classification{Category: domain.Category(rec.choice), Confidence: rec.confidence}
		categoryOK := class.Category == recorded.Category
		confidenceDrift := class.Confidence - recorded.Confidence
		if confidenceDrift < 0 {
			confidenceDrift = -confidenceDrift
		}
		verdictOK := fakes.AskAgain(class) == fakes.AskAgain(recorded)
		if !categoryOK || !verdictOK {
			mismatches++
		}
		if confidenceDrift > 1e-9 {
			drift++
		}
		fmt.Printf("%-3d %-50.50s %-18s %-18s %-8.2f %-9t %.9f\n",
			i+1, rec.answer,
			fmt.Sprintf("%s@%.2f", rec.choice, rec.confidence),
			fmt.Sprintf("%s@%.2f", class.Category, class.Confidence),
			class.Confidence, fakes.AskAgain(class), reply.cost)
		if !categoryOK {
			fmt.Printf("    MISMATCH: category live %s, recorded %s\n", class.Category, rec.choice)
		}
		if confidenceDrift > 1e-9 {
			fmt.Printf("    DRIFT: confidence live %.4f, recorded %.4f\n", class.Confidence, rec.confidence)
		}
		if !verdictOK {
			fmt.Printf("    VERDICT FLIP: live ask-again %t, recorded %t\n", fakes.AskAgain(class), fakes.AskAgain(recorded))
		}
	}

	fmt.Printf("total: %.9f USD for %d calls and %d input tokens (reference about $0.00024)\n", totalCost, len(want), totalTokens)
	fmt.Printf("model: last reply resolved %s (expected %s)\n", lastModel, expectedBuild)
	if mismatches > 0 {
		return fmt.Errorf("%d of 11 phrases differ from the recorded table in category or verdict", mismatches)
	}
	fmt.Println("match: all 11 phrases reproduce the recorded category and ask-again verdict")
	if drift > 0 {
		fmt.Printf("note: confidence drifted on %d vague phrase(s); every verdict still holds (see DRIFT lines)\n", drift)
	}
	return nil
}
