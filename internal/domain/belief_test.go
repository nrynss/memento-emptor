package domain

import (
	"encoding/json"
	"errors"
	"testing"
)

// TestSubjectSerializesAsTheContract pins the subject's wire keys. Every
// belief schema declares kind, id and name and refuses any other key, so
// the domain type carries the tags rather than a caller renaming the
// marshaled output afterwards.
func TestSubjectSerializesAsTheContract(t *testing.T) {
	raw, err := json.Marshal(Subject{Kind: SubjectBrand, ID: "brand-x-001", Name: "Brand X"})
	if err != nil {
		t.Fatalf("marshal subject: %v", err)
	}
	var keys map[string]any
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatalf("parse subject: %v", err)
	}
	want := map[string]any{"kind": "brand", "id": "brand-x-001", "name": "Brand X"}
	if len(keys) != len(want) {
		t.Fatalf("subject serialises %s, want exactly %d keys", raw, len(want))
	}
	for key, value := range want {
		if keys[key] != value {
			t.Errorf("subject %q = %v, want %v", key, keys[key], value)
		}
	}
}

func TestStatusGraphRoundTrip(t *testing.T) {
	cases := []struct {
		status BeliefStatus
		state  GraphState
	}{
		{StatusClaimed, GraphNone},
		{StatusBuilding, GraphCandidate},
		{StatusEarned, GraphVenerable},
		{StatusOutOfTrue, GraphContradicted},
	}
	for _, tc := range cases {
		state, err := GraphStateFor(tc.status)
		if err != nil {
			t.Errorf("GraphStateFor(%q) returned error: %v", tc.status, err)
			continue
		}
		if state != tc.state {
			t.Errorf("GraphStateFor(%q) = %q, want %q", tc.status, state, tc.state)
		}
		status, err := StatusForGraph(tc.state)
		if err != nil {
			t.Errorf("StatusForGraph(%q) returned error: %v", tc.state, err)
			continue
		}
		if status != tc.status {
			t.Errorf("StatusForGraph(%q) = %q, want %q", tc.state, status, tc.status)
		}
	}
}

func TestBeliefStatusesCoverGraphStates(t *testing.T) {
	if len(BeliefStatuses) != len(GraphStates) {
		t.Fatalf("statuses %d != states %d", len(BeliefStatuses), len(GraphStates))
	}
	for _, status := range BeliefStatuses {
		state, err := GraphStateFor(status)
		if err != nil {
			t.Errorf("status %q has no graph state: %v", status, err)
			continue
		}
		back, err := StatusForGraph(state)
		if err != nil {
			t.Errorf("state %q has no status: %v", state, err)
			continue
		}
		if back != status {
			t.Errorf("round trip %q -> %q -> %q", status, state, back)
		}
	}
}

func TestUnknownStatusAndState(t *testing.T) {
	if _, err := GraphStateFor("Settled"); !errors.Is(err, ErrUnknownBeliefStatus) {
		t.Errorf("GraphStateFor(unknown) error = %v, want ErrUnknownBeliefStatus", err)
	}
	if _, err := StatusForGraph("Canonical"); !errors.Is(err, ErrUnknownGraphState) {
		t.Errorf("StatusForGraph(unknown) error = %v, want ErrUnknownGraphState", err)
	}
}
