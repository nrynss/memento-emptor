package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/nrynss/memento-emptor/internal/api/contract"
)

// dispatch runs one frozen tool by name over its request JSON and returns
// the response JSON. Unknown tools fail; unknown fields in the request
// fail fast so caller and contract cannot drift.
func (s *Service) dispatch(ctx context.Context, tool string, raw json.RawMessage) (json.RawMessage, error) {
	decode := func(target any) error {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(target); err != nil {
			return fmt.Errorf("bad %s request: %w", tool, err)
		}
		return nil
	}
	encode := func(value any) (json.RawMessage, error) {
		out, err := marshalResponse(value)
		if err != nil {
			return nil, fmt.Errorf("encode %s response: %w", tool, err)
		}
		return out, nil
	}
	switch contract.ToolName(tool) {
	case contract.ToolRecallForPurchase:
		var req contract.RecallRequest
		if err := decode(&req); err != nil {
			return nil, err
		}
		res, err := s.Recall(ctx, req)
		if err != nil {
			return nil, err
		}
		return encode(res)
	case contract.ToolRecordCorrection:
		var req contract.RecordCorrectionRequest
		if err := decode(&req); err != nil {
			return nil, err
		}
		res, err := s.RecordCorrection(ctx, req)
		if err != nil {
			return nil, err
		}
		return encode(res)
	case contract.ToolPendingQuestions:
		var req contract.PendingQuestionsRequest
		if err := decode(&req); err != nil {
			return nil, err
		}
		res, err := s.PendingQuestions(ctx, req)
		if err != nil {
			return nil, err
		}
		return encode(res)
	case contract.ToolRecordReason:
		var req contract.RecordReasonRequest
		if err := decode(&req); err != nil {
			return nil, err
		}
		res, err := s.RecordReason(ctx, req)
		if err != nil {
			return nil, err
		}
		return encode(res)
	case contract.ToolRecordFeedback:
		var req contract.RecordFeedbackRequest
		if err := decode(&req); err != nil {
			return nil, err
		}
		res, err := s.RecordFeedback(ctx, req)
		if err != nil {
			return nil, err
		}
		return encode(res)
	case contract.ToolExplainBelief:
		var req contract.ExplainBeliefRequest
		if err := decode(&req); err != nil {
			return nil, err
		}
		res, err := s.ExplainBelief(ctx, req)
		if err != nil {
			return nil, err
		}
		return encode(res)
	case contract.ToolLedgerBeliefs:
		var req contract.LedgerBeliefsRequest
		if err := decode(&req); err != nil {
			return nil, err
		}
		res, err := s.LedgerBeliefs(ctx, req)
		if err != nil {
			return nil, err
		}
		return encode(res)
	case contract.ToolLedgerBelief:
		var req contract.LedgerBeliefRequest
		if err := decode(&req); err != nil {
			return nil, err
		}
		res, err := s.LedgerBelief(ctx, req)
		if err != nil {
			return nil, err
		}
		return encode(res)
	case contract.ToolLedgerEvidence:
		var req contract.LedgerEvidenceRequest
		if err := decode(&req); err != nil {
			return nil, err
		}
		res, err := s.LedgerEvidence(ctx, req)
		if err != nil {
			return nil, err
		}
		return encode(res)
	default:
		return nil, fmt.Errorf("unknown tool %q", tool)
	}
}

// marshalResponse encodes one tool response. It then normalizes the
// tagless subject shape: the frozen domain.Subject carries no JSON tags,
// so the encoder emits Go field names while the contract schemas pin
// lowercase kind, id and name with no additional properties. The walk
// renames those three keys inside every subject object, and nothing
// else, so the service serves exactly the frozen shape.
func marshalResponse(value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}
	fixSubjectKeys(decoded)
	return json.Marshal(decoded)
}

// fixSubjectKeys renames the tagless subject keys in place.
func fixSubjectKeys(node any) {
	switch current := node.(type) {
	case map[string]any:
		for key, value := range current {
			if key == "subject" {
				if obj, ok := value.(map[string]any); ok {
					renameKey(obj, "Kind", "kind")
					renameKey(obj, "ID", "id")
					renameKey(obj, "Name", "name")
				}
			}
			fixSubjectKeys(value)
		}
	case []any:
		for _, value := range current {
			fixSubjectKeys(value)
		}
	}
}

// renameKey moves one key's value when present.
func renameKey(obj map[string]any, from, to string) {
	if value, ok := obj[from]; ok {
		delete(obj, from)
		obj[to] = value
	}
}
