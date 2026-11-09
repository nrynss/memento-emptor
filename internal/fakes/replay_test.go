package fakes

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// replayRoot returns the repository root beside this package.
func replayRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(fixtureRoot(t), "..")
}

// replayFile reads one committed replay fixture.
func replayFile(t *testing.T, name string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixtureRoot(t), "replay", name))
	if err != nil {
		t.Fatalf("read testdata/replay/%s: %v", name, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse testdata/replay/%s: %v", name, err)
	}
	return doc
}

// offlineDeps builds the replay's fakes over the committed recordings.
func offlineDeps(t *testing.T) Deps {
	t.Helper()
	root := fixtureRoot(t)
	classifier, err := NewRecordingReasonClassifier(filepath.Join(root, "jev"))
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewRecordingEvidenceStore(filepath.Join(root, "elastic"))
	if err != nil {
		t.Fatal(err)
	}
	return Deps{
		Payments:   NewRecordingPaymentSource(DemoStoryInputs()),
		Classifier: classifier,
		Store:      store,
		Graph:      NewRecordingEvidenceGraph(GraphContents(), StoryRecalls()),
	}
}

// runOffline runs the replay the way the gate requires: keys unset or
// empty, proxies broken, no network to fall back on.
func runOffline(t *testing.T) Result {
	t.Helper()
	for _, name := range KeyVariables() {
		t.Setenv(name, "")
	}
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	t.Setenv("ALL_PROXY", "http://127.0.0.1:1")
	if err := AssertKeysAbsent(); err != nil {
		t.Fatalf("offline precondition failed: %v", err)
	}
	result, err := Run(context.Background(), offlineDeps(t))
	if err != nil {
		t.Fatalf("offline replay failed: %v", err)
	}
	return result
}

// TestReplayOffline is the gate's proof: the whole story replays with no
// key and no network, and the promotion arc lands where the research says
// it must: Candidate after the second return, Earned after the third,
// contradicted-but-kept after the correction, and sizing untouched by the
// dispute and the lone feedback.
func TestReplayOffline(t *testing.T) {
	result := runOffline(t)

	want := []Waypoint{
		{Step: 2, Content: BeliefRunsSmall, State: domain.GraphNone, Dependents: 1, Contradicted: false, PendingCount: 0},
		{Step: 3, Content: BeliefRunsSmall, State: domain.GraphCandidate, Dependents: 2, Contradicted: false, PendingCount: 0},
		{Step: 4, Content: BeliefRunsSmall, State: domain.GraphVenerable, Dependents: 3, Contradicted: false, PendingCount: 0},
		{Step: 5, Content: BeliefRunsSmall, State: domain.GraphVenerable, Dependents: 3, Contradicted: true, PendingCount: 0},
		{Step: 6, Content: BeliefRunsSmall, State: domain.GraphVenerable, Dependents: 3, Contradicted: true, PendingCount: 0},
		{Step: 6, Content: BeliefShipsReliably, State: domain.GraphNone, Dependents: 1, Contradicted: false, PendingCount: 0},
		{Step: 9, Content: BeliefSizeMNow, State: domain.GraphNone, Dependents: 1, Contradicted: false, PendingCount: 1},
		{Step: 9, Content: BeliefRunsSmall, State: domain.GraphVenerable, Dependents: 3, Contradicted: true, PendingCount: 1},
	}
	if len(result.Waypoints) != len(want) {
		t.Fatalf("replay records %d waypoints, want %d", len(result.Waypoints), len(want))
	}
	for i, waypoint := range want {
		if result.Waypoints[i] != waypoint {
			t.Errorf("waypoint %d = %+v, want %+v", i, result.Waypoints[i], waypoint)
		}
	}

	if len(result.Final.Pending.Questions) != 1 {
		t.Fatalf("final pending count = %d, want 1", len(result.Final.Pending.Questions))
	}
	question := result.Final.Pending.Questions[0]
	if question.QuestionID != "q_9f04ab" {
		t.Errorf("pending question = %q, want q_9f04ab", question.QuestionID)
	}
	fitCount := 0
	for _, row := range result.Final.Evidence {
		for _, id := range row.BeliefIDs {
			if id == "blf_8f2c41" {
				fitCount++
			}
		}
	}
	if fitCount != 3 {
		t.Errorf("sizing belief holds %d evidence rows, want 3: the pending refund adds no fourth row", fitCount)
	}
}

// TestReplayMatchesFixtures proves the committed fixtures are the
// replay's output: the log, the final ledger and the payment leg all
// round-trip byte for byte in meaning.
func TestReplayMatchesFixtures(t *testing.T) {
	result := runOffline(t)

	encode := func(v any) map[string]any {
		t.Helper()
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		return doc
	}
	stepsDoc := replayFile(t, "story.json")
	stepsRaw, err := json.Marshal(result.Steps)
	if err != nil {
		t.Fatal(err)
	}
	var stepsAny any
	if err := json.Unmarshal(stepsRaw, &stepsAny); err != nil {
		t.Fatal(err)
	}
	stepsWant, ok := stepsDoc["steps"]
	if !ok {
		t.Fatal("story.json has no steps")
	}
	if !jsonEqual(stepsAny, stepsWant) {
		t.Error("replay log differs from testdata/replay/story.json")
	}

	ledgerDoc := replayFile(t, "p4-ledger.json")
	if !jsonEqual(normalizeWire(encode(result.Final)), ledgerDoc) {
		t.Error("replay final state differs from testdata/replay/p4-ledger.json")
	}

	inputsDoc := replayFile(t, "p2-inputs.json")
	if !jsonEqual(encode(DemoStoryInputs()), inputsDoc) {
		t.Error("payment leg differs from testdata/replay/p2-inputs.json")
	}
}

// TestReplayMatchesContract binds the committed ledger fixture to the
// frozen interface: the same rows P2.5 serves and P4 renders, copied
// verbatim, not retyped.
func TestReplayMatchesContract(t *testing.T) {
	apiDir := filepath.Join(fixtureRoot(t), "api")
	load := func(name string) map[string]any {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(apiDir, name))
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		return doc
	}
	ledgerDoc := replayFile(t, "p4-ledger.json")
	beliefs := load("ledger_beliefs.response.example.json")["beliefs"]
	if !jsonEqual(ledgerDoc["beliefs"], beliefs) {
		t.Error("p4-ledger.json beliefs differ from the frozen ledger_beliefs example")
	}
	evidence := load("ledger_evidence.response.example.json")["evidence"]
	if !jsonEqual(ledgerDoc["evidence"], evidence) {
		t.Error("p4-ledger.json evidence differs from the frozen ledger_evidence example")
	}
	pending := load("pending_questions.response.example.json")
	if !jsonEqual(ledgerDoc["pending"], pending) {
		t.Error("p4-ledger.json pending differs from the frozen pending_questions example")
	}
}

// normalizeWire renames the frozen domain.Subject's Go field names to the
// wire lowercase the frozen snapshots use. The subject type carries no
// json tags and contracts freeze at the end of P1, so the replay's
// marshaled output and the hand-written snapshots differ only here;
// decoding is case-insensitive, so the interface is unaffected.
func normalizeWire(doc map[string]any) map[string]any {
	var walk func(v any)
	walk = func(v any) {
		switch node := v.(type) {
		case map[string]any:
			if subject, ok := node["subject"].(map[string]any); ok {
				for _, key := range []string{"Kind", "ID", "Name"} {
					if value, present := subject[key]; present {
						delete(subject, key)
						subject[strings.ToLower(key)] = value
					}
				}
			}
			for _, value := range node {
				walk(value)
			}
		case []any:
			for _, value := range node {
				walk(value)
			}
		}
	}
	walk(doc)
	return doc
}

// TestP3TurnsMatchContract binds the agent's fixture to the frozen
// interface: every inlined turn equals its contract example, except the
// record_reason request, which is story-specific by decision (the vague
// answer that leaves the question pending) while its response is the
// frozen ask-again example verbatim.
func TestP3TurnsMatchContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(fixtureRoot(t), "replay", "p3-turns.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		ShopperID string `json:"shopper_id"`
		Turns     []struct {
			N        int            `json:"n"`
			Tool     string         `json:"tool"`
			Request  map[string]any `json:"request"`
			Response map[string]any `json:"response"`
		} `json:"turns"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	example := func(tool, side string) map[string]any {
		t.Helper()
		name := tool + "." + side + ".example"
		if tool == "record_reason" && side == "response" {
			name = "record_reason.response.example-unclear"
		}
		raw, err := os.ReadFile(filepath.Join(fixtureRoot(t), "api", name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var filed map[string]any
		if err := json.Unmarshal(raw, &filed); err != nil {
			t.Fatal(err)
		}
		return filed
	}
	if len(doc.Turns) != 6 {
		t.Fatalf("p3-turns.json holds %d turns, want 6", len(doc.Turns))
	}
	for _, turn := range doc.Turns {
		if turn.Tool == "record_reason" {
			if turn.Request["answer_text"] != "not great" || turn.Request["question_id"] != "q_9f04ab" {
				t.Errorf("turn %d answers %v, want the vague answer to q_9f04ab", turn.N, turn.Request)
			}
			if !jsonEqual(turn.Response, example(turn.Tool, "response")) {
				t.Errorf("turn %d response differs from the frozen ask-again example", turn.N)
			}
			continue
		}
		if !jsonEqual(turn.Request, example(turn.Tool, "request")) {
			t.Errorf("turn %d request differs from the frozen %s request example", turn.N, turn.Tool)
		}
		if !jsonEqual(turn.Response, example(turn.Tool, "response")) {
			t.Errorf("turn %d response differs from the frozen %s response example", turn.N, turn.Tool)
		}
	}
}

// jsonEqual compares two decoded JSON documents by canonical encoding.
func jsonEqual(a, b any) bool {
	encode := func(v any) string {
		raw, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(raw)
	}
	return encode(a) == encode(b)
}

// keyPatterns are the shapes a leaked key would take. Recorded
// signatures are bare base64 with no prefix and never match; redacted
// placeholders never match either.
var keyPatterns = []*regexp.Regexp{
	regexp.MustCompile(`sk-or-v1-[A-Za-z0-9_\-]{8,}`),
	regexp.MustCompile(`sk-live-[A-Za-z0-9]{8,}`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`AIza[0-9A-Za-z_\-]{35}`),
	regexp.MustCompile(`gh[pous]_[A-Za-z0-9]{30,}`),
	regexp.MustCompile(`xox[bpas]-[A-Za-z0-9\-]+`),
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	regexp.MustCompile(`Bearer eyJ[A-Za-z0-9_\-\.]+`),
}

// TestNoKeyMaterial fails the build if any replayed fixture contains
// key-shaped material. It scans the replay fixtures plus every P0
// recording the manifest names.
func TestNoKeyMaterial(t *testing.T) {
	root := fixtureRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "replay", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		P0Sources map[string][]string `json:"p0_sources"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	var files []string
	entries, err := os.ReadDir(filepath.Join(root, "replay"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, filepath.Join(root, "replay", entry.Name()))
		}
	}
	for _, paths := range manifest.P0Sources {
		for _, path := range paths {
			if strings.HasSuffix(path, "/") {
				dirEntries, err := os.ReadDir(filepath.Join(replayRoot(t), path))
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range dirEntries {
					if !entry.IsDir() {
						files = append(files, filepath.Join(replayRoot(t), path, entry.Name()))
					}
				}
				continue
			}
			files = append(files, filepath.Join(replayRoot(t), path))
		}
	}
	for _, path := range files {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, pattern := range keyPatterns {
			if match := pattern.Find(contents); match != nil {
				t.Errorf("%s contains key-shaped material: %s...", path, match)
			}
		}
	}
}

// TestFakesSourceHasNoKeysOrNetwork proves the seam can hold: the fakes
// name no key variable, shell out to no keyring, and import no HTTP
// client or service hostname. A fake that phones home cannot pass this.
func TestFakesSourceHasNoKeysOrNetwork(t *testing.T) {
	dir := "."
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		text := string(contents)
		// The key-variable list itself lives in errors.go: that is the
		// guard, not a read. Everywhere else, naming a key variable is
		// a defect.
		if entry.Name() != "errors.go" {
			for _, banned := range KeyVariables() {
				if strings.Contains(text, banned) {
					t.Errorf("%s names %q", entry.Name(), banned)
				}
			}
		}
		for _, banned := range []string{
			"secret-tool",
			"net/http",
			"api-m.sandbox.paypal.com",
			"openrouter.ai",
			"trychannel3.com",
			"http.Client",
			"http.Get",
			"http.Post",
		} {
			if strings.Contains(text, banned) {
				t.Errorf("%s names %q", entry.Name(), banned)
			}
		}
	}
	if err := AssertKeysAbsent(); err != nil && !errors.Is(err, ErrKeysPresent) {
		t.Fatalf("guard failed unexpectedly: %v", err)
	}
}
