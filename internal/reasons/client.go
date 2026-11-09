package reasons

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// DefaultModel is the pinned classifier model. A version change re-opens
// the floor measurement.
const DefaultModel = "typesafe/jev-1.14"

// DefaultEndpoint is the decisions endpoint the client posts one
// exclusive-choice question per shopper answer to.
const DefaultEndpoint = "https://openrouter.ai/api/alpha/decisions"

// QuestionName is the single question each request carries.
const QuestionName = "reason"

// Instructions asks for the one category that best matches the answer.
// It is the wording the floor was measured on; changing it re-opens the
// measurement.
const Instructions = "Why is the shopper returning the item? Pick the single category that best matches the shopper's answer."

// Criteria is the exclusive category set the question offers. Fit
// direction feeds the sizing belief, damage and non-arrival count only
// against the seller, changed mind counts toward nothing, and unclear is
// the outlet for vague answers so the agent asks again instead of
// guessing.
var Criteria = map[string]string{
	"fit_too_small":        "The item is too small or too tight: too tight in the shoulders, sleeves too short, cannot button or zip, runs small.",
	"fit_too_large":        "The item is too big or too loose: swims on the shopper, falls off, runs large.",
	"damaged_or_defective": "The item arrived broken, damaged, cracked, torn, or does not work.",
	"not_arrived":          "The item never arrived, was lost in shipping, or delivery failed.",
	"changed_mind":         "The shopper changed their mind, no longer wants the item, or gives no product fault.",
	"unclear":              "The answer gives no usable reason, or is vague, ambiguous, or off-topic, so the agent must ask again.",
}

// ErrEmptyAnswer is returned when there is no shopper answer to classify.
var ErrEmptyAnswer = errors.New("reasons: empty answer")

// ErrBadResponse is returned when the classifier reply is missing,
// truncated, or out of shape: transport failure, non-200 status,
// undecodable body, or a confidence outside 0 to 1.
var ErrBadResponse = errors.New("reasons: bad classifier response")

// ErrUnknownCategory is returned when the classifier picks a choice
// outside the exclusive set. No code path may turn an unknown choice
// into a belief.
var ErrUnknownCategory = errors.New("reasons: unknown reason category")

// Client classifies shopper answers behind the ReasonClassifier seam.
// The transport is injected so tests replay the recorded fixtures; the
// live service is reached only by the sibling live test, never in CI.
type Client struct {
	model    string
	endpoint string
	do       func(*http.Request) (*http.Response, error)
}

// Compile-time proof the client satisfies the seam the rest of the layer
// builds on.
var _ fakes.ReasonClassifier = (*Client)(nil)

// NewClient returns a Client posting to endpoint with model and sending
// through do. A nil do sends through the default transport.
func NewClient(model, endpoint string, do func(*http.Request) (*http.Response, error)) *Client {
	if do == nil {
		do = http.DefaultClient.Do
	}
	return &Client{model: model, endpoint: endpoint, do: do}
}

type choiceRequest struct {
	Model     string         `json:"model"`
	State     map[string]any `json:"state"`
	Questions map[string]any `json:"questions"`
}

type choiceReply struct {
	Model   string `json:"model"`
	Answers map[string]struct {
		Type       string  `json:"type"`
		Choice     string  `json:"choice"`
		Confidence float64 `json:"confidence"`
	} `json:"answers"`
}

// Classify returns the category and confidence for one shopper answer.
// A refusal carries no choice, so it maps to unclear at zero confidence
// and the agent asks again. Unknown answers and malformed replies fail
// loudly instead of inventing a category.
func (c *Client) Classify(ctx context.Context, answer string) (fakes.Classification, error) {
	if answer == "" {
		return fakes.Classification{}, fmt.Errorf("%w: no answer to classify", ErrEmptyAnswer)
	}
	body, err := json.Marshal(choiceRequest{
		Model: c.model,
		State: map[string]any{"answer": answer},
		Questions: map[string]any{QuestionName: map[string]any{
			"type":         "choice",
			"instructions": Instructions,
			"criteria":     Criteria,
		}},
	})
	if err != nil {
		return fakes.Classification{}, fmt.Errorf("%w: encode request: %v", ErrBadResponse, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fakes.Classification{}, fmt.Errorf("%w: build request: %v", ErrBadResponse, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if err := ctx.Err(); err != nil {
		return fakes.Classification{}, fmt.Errorf("%w: %w", ErrBadResponse, err)
	}
	resp, err := c.do(req)
	if err != nil {
		return fakes.Classification{}, fmt.Errorf("%w: %w", ErrBadResponse, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fakes.Classification{}, fmt.Errorf("%w: read body: %v", ErrBadResponse, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fakes.Classification{}, fmt.Errorf("%w: status %d", ErrBadResponse, resp.StatusCode)
	}
	var reply choiceReply
	if err := json.Unmarshal(raw, &reply); err != nil {
		return fakes.Classification{}, fmt.Errorf("%w: decode body: %v", ErrBadResponse, err)
	}
	got, ok := reply.Answers[QuestionName]
	if !ok {
		return fakes.Classification{}, fmt.Errorf("%w: no %q answer", ErrBadResponse, QuestionName)
	}
	if got.Type == "refusal" {
		return fakes.Classification{
			Category:   domain.CategoryUnclear,
			Confidence: 0,
			Provenance: reply.Model,
		}, nil
	}
	category := domain.Category(got.Choice)
	switch category {
	case domain.CategoryFitTooSmall,
		domain.CategoryFitTooLarge,
		domain.CategoryDamagedOrDefective,
		domain.CategoryNotArrived,
		domain.CategoryChangedMind,
		domain.CategoryUnclear:
	default:
		return fakes.Classification{}, fmt.Errorf("%w: %q", ErrUnknownCategory, got.Choice)
	}
	if got.Confidence < 0 || got.Confidence > 1 {
		return fakes.Classification{}, fmt.Errorf("%w: confidence %v", ErrBadResponse, got.Confidence)
	}
	return fakes.Classification{
		Category:   category,
		Confidence: got.Confidence,
		Provenance: reply.Model,
	}, nil
}
