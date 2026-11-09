package fakes

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/nrynss/memento-emptor/internal/domain"
)

// ConfidenceFloor is the ask-again floor from the classifier research: a
// vague answer, or a classification under this confidence, stays pending
// and the agent asks again. On the labelled set every clear phrase
// returned 1.00 and every vague one fell below the floor.
const ConfidenceFloor = 0.98

// Classification is one recorded answer: the exclusive category plus the
// classifier's confidence, with the recording it replays.
type Classification struct {
	// Category is the classified reason for the shopper's answer.
	Category domain.Category
	// Confidence is the classifier's confidence, from 0 to 1.
	Confidence float64
	// Provenance names the P0 recording this answer replays.
	Provenance string
}

// AskAgain reports whether the answer stays a pending question: the
// choice is unclear, or the confidence is under the floor.
func AskAgain(class Classification) bool {
	return class.Category == domain.CategoryUnclear || class.Confidence < ConfidenceFloor
}

// ReasonClassifier is the seam the real classifier client implements in
// P2: one exclusive-choice question per shopper answer. Unknown answers
// fail with ErrNoRecording; the fake never invents a category.
type ReasonClassifier interface {
	// Classify returns the recorded choice and confidence for one
	// shopper answer, verbatim.
	Classify(ctx context.Context, answer string) (Classification, error)
}

// RecordingReasonClassifier is a ReasonClassifier that replays the
// labelled Jev recordings. The table is built from the recordings
// themselves at construction, keyed on the exact recorded answer.
type RecordingReasonClassifier struct {
	answers map[string]Classification
}

// NewRecordingReasonClassifier loads every labelled recording under dir
// and builds the answer table from the recorded request and reply.
func NewRecordingReasonClassifier(dir string) (*RecordingReasonClassifier, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("fakes: read classifier recordings: %w", err)
	}
	fake := &RecordingReasonClassifier{answers: map[string]Classification{}}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("fakes: read classifier recording: %w", err)
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
			return nil, fmt.Errorf("fakes: parse classifier recording %s: %w", entry.Name(), err)
		}
		if doc.Request.State.Answer == "" || doc.Response.Answers.Reason.Choice == "" {
			return nil, fmt.Errorf("fakes: classifier recording %s has no answer or choice", entry.Name())
		}
		fake.answers[doc.Request.State.Answer] = Classification{
			Category:   domain.Category(doc.Response.Answers.Reason.Choice),
			Confidence: doc.Response.Answers.Reason.Confidence,
			Provenance: "testdata/jev/" + entry.Name(),
		}
	}
	if len(fake.answers) == 0 {
		return nil, fmt.Errorf("fakes: no classifier recordings in %s", dir)
	}
	return fake, nil
}

// Classify returns the recorded choice and confidence for one shopper
// answer, or ErrNoRecording when no recording covers the answer.
func (f *RecordingReasonClassifier) Classify(ctx context.Context, answer string) (Classification, error) {
	if err := ctx.Err(); err != nil {
		return Classification{}, fmt.Errorf("fakes: classify answer: %w", err)
	}
	class, ok := f.answers[answer]
	if !ok {
		return Classification{}, fmt.Errorf("%w: answer %q", ErrNoRecording, answer)
	}
	return class, nil
}

// RecordedAnswers lists every answer the fake covers, sorted.
func (f *RecordingReasonClassifier) RecordedAnswers() []string {
	answers := make([]string, 0, len(f.answers))
	for answer := range f.answers {
		answers = append(answers, answer)
	}
	sort.Strings(answers)
	return answers
}
