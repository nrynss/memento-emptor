package reasons

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// fixtureRoot returns the repository's testdata directory beside this package.
func fixtureRoot(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "testdata")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("testdata is missing at %s: %v", dir, err)
	}
	return dir
}

// labelledAnswers loads every recorded classifier fixture keyed on the
// exact recorded shopper answer, with the raw reply body to serve.
func labelledAnswers(t *testing.T) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(fixtureRoot(t), "jev"))
	if err != nil {
		t.Fatal(err)
	}
	answers := map[string][]byte{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(fixtureRoot(t), "jev", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Request struct {
				State struct {
					Answer string `json:"answer"`
				} `json:"state"`
			} `json:"request"`
			Response json.RawMessage `json:"response"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		answers[doc.Request.State.Answer] = doc.Response
	}
	if len(answers) == 0 {
		t.Fatal("no classifier recordings found")
	}
	return answers
}

// fixtureDo serves the recorded reply matching each outgoing request's
// answer, verbatim. Answers outside the labelled set fail loudly.
func fixtureDo(t *testing.T, served *map[string]int) func(*http.Request) (*http.Response, error) {
	t.Helper()
	answers := labelledAnswers(t)
	return func(req *http.Request) (*http.Response, error) {
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		var body struct {
			State struct {
				Answer string `json:"answer"`
			} `json:"state"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			return nil, err
		}
		reply, ok := answers[body.State.Answer]
		if !ok {
			return nil, fakes.ErrNoRecording
		}
		if served != nil {
			(*served)[body.State.Answer]++
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(reply)),
		}, nil
	}
}

// assertOffline proves the replay needs no key and spends nothing.
func assertOffline(t *testing.T) {
	t.Helper()
	for _, name := range fakes.KeyVariables() {
		t.Setenv(name, "")
	}
	if err := fakes.AssertKeysAbsent(); err != nil {
		t.Fatalf("keys unset but guard failed: %v", err)
	}
}

// TestClientAgreesWithFake proves the client satisfies the same seam and
// returns the same category and confidence as the fake on every labelled
// phrase.
func TestClientAgreesWithFake(t *testing.T) {
	assertOffline(t)
	dir := filepath.Join(fixtureRoot(t), "jev")
	fake, err := fakes.NewRecordingReasonClassifier(dir)
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(DefaultModel, DefaultEndpoint, fixtureDo(t, nil))
	ctx := context.Background()
	for _, answer := range fake.RecordedAnswers() {
		want, err := fake.Classify(ctx, answer)
		if err != nil {
			t.Fatal(err)
		}
		got, err := client.Classify(ctx, answer)
		if err != nil {
			t.Errorf("client classify %q: %v", answer, err)
			continue
		}
		if got.Category != want.Category || got.Confidence != want.Confidence {
			t.Errorf("client classify %q = (%v, %v), fake = (%v, %v)",
				answer, got.Category, got.Confidence, want.Category, want.Confidence)
		}
		if got.Provenance == "" {
			t.Errorf("client classify %q carries no provenance", answer)
		}
	}
}

// TestFloorSeparatesLabelledSet re-measures the floor question on the
// labelled set: every clear phrase classifies at full confidence with
// the right category, and every vague or ambiguous one asks again. Any
// drift from the recorded table fails here before it can gate a belief.
func TestFloorSeparatesLabelledSet(t *testing.T) {
	assertOffline(t)
	if fakes.ConfidenceFloor != 0.98 {
		t.Fatalf("confidence floor = %v, want 0.98: the recorded measurement no longer holds", fakes.ConfidenceFloor)
	}
	client := NewClient(DefaultModel, DefaultEndpoint, fixtureDo(t, nil))
	ctx := context.Background()
	clear := []struct {
		answer   string
		category domain.Category
	}{
		{"too tight in the shoulders", domain.CategoryFitTooSmall},
		{"the sleeves are way too short and I can't button it", domain.CategoryFitTooSmall},
		{"it swims on me, far too big", domain.CategoryFitTooLarge},
		{"arrived broken", domain.CategoryDamagedOrDefective},
		{"the screen was cracked when I opened the box", domain.CategoryDamagedOrDefective},
		{"it never arrived, tracking has been stuck for weeks", domain.CategoryNotArrived},
		{"changed my mind", domain.CategoryChangedMind},
		{"just don't want it anymore", domain.CategoryChangedMind},
	}
	vague := []struct {
		answer     string
		category   domain.Category
		confidence float64
	}{
		{"not great", domain.CategoryUnclear, 0.94},
		{"it wasn't what I wanted", domain.CategoryChangedMind, 0.96},
		{"meh, sending it back", domain.CategoryUnclear, 0.68},
	}
	lowestClear := 2.0
	for _, tc := range clear {
		class, err := client.Classify(ctx, tc.answer)
		if err != nil {
			t.Errorf("classify %q: %v", tc.answer, err)
			continue
		}
		if class.Category != tc.category {
			t.Errorf("classify %q category = %v, want %v", tc.answer, class.Category, tc.category)
		}
		if class.Confidence != 1 {
			t.Errorf("classify %q confidence = %v, want 1.00: the floor margin moved", tc.answer, class.Confidence)
		}
		if fakes.AskAgain(class) {
			t.Errorf("clear answer %q asks again", tc.answer)
		}
		if class.Confidence < lowestClear {
			lowestClear = class.Confidence
		}
	}
	highestVague := -1.0
	unclearCaught := 0
	for _, tc := range vague {
		class, err := client.Classify(ctx, tc.answer)
		if err != nil {
			t.Errorf("classify %q: %v", tc.answer, err)
			continue
		}
		if class.Category != tc.category || class.Confidence != tc.confidence {
			t.Errorf("classify %q = (%v, %v), recorded (%v, %v): drift",
				tc.answer, class.Category, class.Confidence, tc.category, tc.confidence)
		}
		if !fakes.AskAgain(class) {
			t.Errorf("vague answer %q does not ask again", tc.answer)
		}
		if class.Category == domain.CategoryUnclear {
			unclearCaught++
		}
		if class.Confidence > highestVague {
			highestVague = class.Confidence
		}
	}
	if margin := lowestClear - highestVague; margin < 0.039 || margin > 0.041 {
		t.Errorf("floor margin = %v, want 0.04 (clear 1.00 against vague 0.96)", margin)
	}
	if unclearCaught != 2 {
		t.Errorf("unclear category caught %d of 3 vague answers, want 2 (the floor catches the near-miss)", unclearCaught)
	}
}

// TestNeighbourContextOmitted is the neighbour-context decision, as a test
// rather than an opinion. The recorded requests carry the answer alone:
// no neighbour, example, or retrieved-context field exists in any of the
// eleven request bodies, yet the floor separates the set (see the floor
// test). Nearest labelled reasons therefore add nothing measurable here,
// and the client sends answer-only state. If a future recording needs
// neighbour context to separate, its request shape breaks this test and
// re-opens the decision.
func TestNeighbourContextOmitted(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join(fixtureRoot(t), "jev"))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(fixtureRoot(t), "jev", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Request map[string]json.RawMessage `json:"request"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		for key := range doc.Request {
			switch key {
			case "model", "state", "questions":
			default:
				t.Errorf("recording %s carries unexpected request field %q", entry.Name(), key)
			}
		}
		var state map[string]json.RawMessage
		if err := json.Unmarshal(doc.Request["state"], &state); err != nil {
			t.Fatal(err)
		}
		if len(state) != 1 {
			t.Errorf("recording %s state carries %d fields, want answer alone", entry.Name(), len(state))
		}
		if _, ok := state["answer"]; !ok {
			t.Errorf("recording %s state carries no answer", entry.Name())
		}
		count++
	}
	if count != 11 {
		t.Fatalf("checked %d recordings, want 11", count)
	}
}

// TestRequestShape pins the question the floor was measured on: the
// model, the instructions, and the six exclusive criteria.
func TestRequestShape(t *testing.T) {
	var captured []byte
	client := NewClient(DefaultModel, DefaultEndpoint, func(req *http.Request) (*http.Response, error) {
		var err error
		captured, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		for _, answer := range labelledAnswers(t) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(answer))}, nil
		}
		return nil, fakes.ErrNoRecording
	})
	if _, err := client.Classify(context.Background(), "arrived broken"); err != nil {
		t.Fatal(err)
	}
	var body struct {
		Model     string            `json:"model"`
		State     map[string]string `json:"state"`
		Questions map[string]struct {
			Type         string            `json:"type"`
			Instructions string            `json:"instructions"`
			Criteria     map[string]string `json:"criteria"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(captured, &body); err != nil {
		t.Fatal(err)
	}
	if body.Model != DefaultModel {
		t.Errorf("request model = %q, want %q", body.Model, DefaultModel)
	}
	if body.State["answer"] != "arrived broken" {
		t.Errorf("request state = %v, want the shopper's answer", body.State)
	}
	question, ok := body.Questions[QuestionName]
	if !ok {
		t.Fatalf("request carries no %q question", QuestionName)
	}
	if question.Type != "choice" || question.Instructions != Instructions {
		t.Errorf("request question = (%q, %q), want the measured wording", question.Type, question.Instructions)
	}
	if len(question.Criteria) != 6 {
		t.Errorf("request criteria cover %d categories, want 6", len(question.Criteria))
	}
	for _, category := range []domain.Category{
		domain.CategoryFitTooSmall, domain.CategoryFitTooLarge,
		domain.CategoryDamagedOrDefective, domain.CategoryNotArrived,
		domain.CategoryChangedMind, domain.CategoryUnclear,
	} {
		if _, ok := question.Criteria[string(category)]; !ok {
			t.Errorf("request criteria miss %q", category)
		}
	}
}

// TestRefusalAsksAgain proves a refusal answer stays a pending question
// instead of becoming a belief.
func TestRefusalAsksAgain(t *testing.T) {
	reply := `{"model": "typesafe/jev-1.14-20261014", "answers": {"reason": {"type": "refusal", "confidence": 0}}}`
	client := NewClient(DefaultModel, DefaultEndpoint, func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(reply))}, nil
	})
	class, err := client.Classify(context.Background(), "I will not say")
	if err != nil {
		t.Fatal(err)
	}
	if class.Category != domain.CategoryUnclear || class.Confidence != 0 {
		t.Errorf("refusal = (%v, %v), want (unclear, 0)", class.Category, class.Confidence)
	}
	if !fakes.AskAgain(class) {
		t.Error("refusal does not ask again")
	}
	if reason, askAgain := Settle("I will not say", class); reason != nil || !askAgain {
		t.Error("refusal settles to a reason instead of staying pending")
	}
}

// TestClientErrors proves loud failure: no silent category, no call
// without an answer, no masked transport or shape fault.
func TestClientErrors(t *testing.T) {
	ctx := context.Background()
	called := false
	failer := func(*http.Request) (*http.Response, error) {
		called = true
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	}
	if _, err := NewClient(DefaultModel, DefaultEndpoint, failer).Classify(ctx, ""); !errors.Is(err, ErrEmptyAnswer) {
		t.Errorf("empty answer err = %v, want ErrEmptyAnswer", err)
	}
	if called {
		t.Error("empty answer reached the transport")
	}
	serve := func(body string, status int) func(*http.Request) (*http.Response, error) {
		return func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
		}
	}
	good := `{"model": "m", "answers": {"reason": {"type": "choice", "choice": "changed_mind", "confidence": 1}}}`
	cases := []struct {
		name   string
		do     func(*http.Request) (*http.Response, error)
		answer string
		want   error
	}{
		{"unknown choice", serve(`{"model": "m", "answers": {"reason": {"type": "choice", "choice": "too_expensive", "confidence": 1}}}`, 200), "a", ErrUnknownCategory},
		{"none is not a choice", serve(`{"model": "m", "answers": {"reason": {"type": "choice", "choice": "none", "confidence": 1}}}`, 200), "a", ErrUnknownCategory},
		{"missing choice", serve(`{"model": "m", "answers": {"reason": {"type": "choice", "confidence": 1}}}`, 200), "a", ErrUnknownCategory},
		{"non-200", serve("busy", 503), "a", ErrBadResponse},
		{"truncated body", serve(`{"model":`, 200), "a", ErrBadResponse},
		{"missing answer", serve(`{"model": "m", "answers": {}}`, 200), "a", ErrBadResponse},
		{"confidence above one", serve(`{"model": "m", "answers": {"reason": {"type": "choice", "choice": "unclear", "confidence": 2}}}`, 200), "a", ErrBadResponse},
		{"transport failure", func(*http.Request) (*http.Response, error) { return nil, errors.New("dial") }, "a", ErrBadResponse},
		{"unrecorded answer", fixtureDo(t, nil), "too tight across the chest", fakes.ErrNoRecording},
		{"good reply", serve(good, 200), "a", nil},
	}
	for _, tc := range cases {
		_, err := NewClient(DefaultModel, DefaultEndpoint, tc.do).Classify(ctx, tc.answer)
		if tc.want == nil {
			if err != nil {
				t.Errorf("%s: unexpected err %v", tc.name, err)
			}
			continue
		}
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	if _, err := NewClient(DefaultModel, DefaultEndpoint, serve(good, 200)).Classify(cancelled, "a"); err == nil {
		t.Error("cancelled context did not fail")
	}
}
