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
// fail fast so caller and contract cannot drift. Each request is one
// object: trailing data after it fails the call.
func (s *Service) dispatch(ctx context.Context, tool string, raw json.RawMessage) (json.RawMessage, error) {
	decode := func(target any) error {
		return decodeSingle(raw, target, tool)
	}
	encode := func(value any) (json.RawMessage, error) {
		out, err := json.Marshal(value)
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
	case contract.ToolProductFit:
		var req contract.ProductFitRequest
		if err := decode(&req); err != nil {
			return nil, err
		}
		res, err := s.ProductFit(ctx, req)
		if err != nil {
			return nil, err
		}
		return encode(res)
	case contract.ToolSetPooling:
		var req contract.SetPoolingRequest
		if err := decode(&req); err != nil {
			return nil, err
		}
		res, err := s.SetPooling(ctx, req)
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

// decodeSingle decodes one request object with unknown fields rejected.
// A second value after the object fails the call: batches are not a
// shape the guard can scope, so the layer never accepts one.
func decodeSingle(raw json.RawMessage, target any, what string) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("bad %s request: %w", what, err)
	}
	if decoder.More() {
		return fmt.Errorf("bad %s request: trailing data", what)
	}
	return nil
}
