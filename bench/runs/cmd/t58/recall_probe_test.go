// Offline test for the T5.8b settle-note instrument. No key, no network,
// no spend: it proves the pre/post recall reading the re-run writes carries
// a served belief's status, standing and evidence count, so the record can
// tell a withheld belief from an absent one. The two multi-brand seeds are
// the cells the P5 review found unexplained, and the asserted readings are
// the current layer's answer for them.
package main

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/nrynss/memento-emptor/internal/api/contract"
	"github.com/nrynss/memento-emptor/internal/api/server"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/seed"
)

// beliefGraph reads one recording graph through the probe's seam.
type beliefGraph struct {
	inner *fakes.RecordingEvidenceGraph
}

// Inspect reports one belief's promotion reading.
func (g beliefGraph) Inspect(ctx context.Context, content string) (fakes.BeliefReport, error) {
	return g.inner.Inspect(ctx, content)
}

// probeService seeds one history over the recording graph and returns the
// live service the readings come from, plus the graph and the seed.
func probeService(t *testing.T, history seed.History) (*server.Service, beliefGraph, server.Seed) {
	t.Helper()
	s, err := buildSeed(history)
	if err != nil {
		t.Fatalf("seed %s: %v", history.ShopperID, err)
	}
	graph := fakes.NewRecordingEvidenceGraph(server.SeedGraphContents(s), nil)
	store, err := evidence.NewStore(t58Contract{kind: "t58-probe"})
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	svc, err := server.NewService(server.Deps{
		Payments:   closedPayments{},
		Classifier: closedClassifier{},
		Store:      store,
		Graph:      graph,
		SchemaDir:  "../../../../testdata/api",
	})
	if err != nil {
		t.Fatalf("service: %v", err)
	}
	if err := svc.LoadSeed(context.Background(), s); err != nil {
		t.Fatalf("load seed %s: %v", history.ShopperID, err)
	}
	return svc, beliefGraph{inner: graph}, s
}

// TestRecallProbeRecordsCandidateBelief pins what the re-run records for
// the two multi-brand cells. The split shopper's Brand R belief is
// Candidate standing (Building), two evidence rows; the steady shopper's
// Brand Q belief is no standing (Claimed), one row. Both are served, and
// both readings carry them, so the record states the belief rather than
// an empty response.
func TestRecallProbeRecordsCandidateBelief(t *testing.T) {
	cases := []struct {
		shopper     string
		brand       string
		beliefID    string
		value       string
		status      domain.BeliefStatus
		standing    domain.GraphState
		evidence    int
		graphPrefix string
	}{
		{
			shopper: "cmp-brands-split", brand: "BRR", beliefID: "blf_000001",
			value: "runs large, take size S", status: domain.StatusBuilding,
			standing: domain.GraphCandidate, evidence: 2,
		},
		{
			shopper: "cmp-brands-steady", brand: "BRQ", beliefID: "blf_000002",
			value: "size M fits this shopper now", status: domain.StatusClaimed,
			standing: domain.GraphNone, evidence: 1,
		},
	}
	for _, tc := range cases {
		history := mustHistory(t, tc.shopper)
		if brand, _, err := brandForRecall(history); err != nil || brand != tc.brand {
			t.Fatalf("%s recall brand %q, %v; want %s", tc.shopper, brand, err, tc.brand)
		}
		svc, reader, s := probeService(t, history)
		probe, err := probeReading(context.Background(), svc, history, reader,
			beliefContents(s), "t58b-a3-"+strings.TrimPrefix(tc.shopper, "cmp-")+"-r1-pre", "pre-settle")
		if err != nil {
			t.Fatalf("%s pre-settle reading: %v", tc.shopper, err)
		}
		if len(probe.recall.Beliefs) != 1 {
			t.Fatalf("%s: %d beliefs served, want 1", tc.shopper, len(probe.recall.Beliefs))
		}
		belief := probe.recall.Beliefs[0]
		if belief.BeliefID != tc.beliefID {
			t.Errorf("%s: served belief %q, want %q", tc.shopper, belief.BeliefID, tc.beliefID)
		}
		if belief.Value != tc.value {
			t.Errorf("%s: served value %q, want %q", tc.shopper, belief.Value, tc.value)
		}
		if belief.Status != tc.status {
			t.Errorf("%s: served status %q, want %q", tc.shopper, belief.Status, tc.status)
		}
		if belief.Standing != tc.standing {
			t.Errorf("%s: served standing %q, want %q", tc.shopper, belief.Standing, tc.standing)
		}
		if len(belief.Evidence) != tc.evidence {
			t.Errorf("%s: served evidence %d, want %d", tc.shopper, len(belief.Evidence), tc.evidence)
		}

		// The record states the status, standing, evidence count and that
		// recall served it, at both points.
		mem := &arm3Memory{
			settleNote:  "7 polls over 60s: " + tc.brand + "=Candidate/2/0.714",
			recallResp:  probe.recall,
			pendingResp: probe.pending,
			probe:       probe,
		}
		note := settleNotes(mem)
		if note == nil {
			t.Fatalf("%s: settle note absent", tc.shopper)
		}
		for _, want := range []string{
			"live layer seed, settled before recall: 7 polls over 60s",
			"pre-settle recall:",
			"post-settle recall:",
			"served " + tc.beliefID,
			"status " + string(tc.status),
			"standing " + string(tc.standing),
			"evidence " + strconv.Itoa(tc.evidence),
		} {
			if !strings.Contains(*note, want) {
				t.Errorf("%s: settle note missing %q:\n%s", tc.shopper, want, *note)
			}
		}

		// The arm response carries both full responses, labelled, and the
		// served one is the prompt's reading.
		response := armResponse(mem)
		preLabel := "response (pre-settle):"
		postLabel := "response (post-settle, served in the prompt):"
		for _, want := range []string{preLabel, postLabel} {
			if !strings.Contains(response, want) {
				t.Errorf("%s: arm response missing %q", tc.shopper, want)
			}
		}
		parts := strings.SplitN(response, postLabel, 2)
		if len(parts) != 2 {
			t.Fatalf("%s: arm response carries no post-settle block", tc.shopper)
		}
		for label, block := range map[string]string{"pre-settle": parts[0], "post-settle": parts[1]} {
			if !strings.Contains(block, `"belief_id":"`+tc.beliefID+`"`) {
				t.Errorf("%s: %s block does not carry belief %s", tc.shopper, label, tc.beliefID)
			}
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(response[strings.LastIndex(response, "{"):]), &decoded); err != nil {
			t.Errorf("%s: post-settle response is not JSON: %v", tc.shopper, err)
		}
	}
}

// TestArmResponseWithoutProbeKeepsT58Shape pins the default recording: a
// run without the probe writes exactly the T5.8 arm-memory text, so the
// instrument is additive and the older shape stays reproducible.
func TestArmResponseWithoutProbeKeepsT58Shape(t *testing.T) {
	trace := "t58-a3-brands-split-r1"
	mem := &arm3Memory{
		recallResp:  contract.RecallResponse{ShopperID: "cmp-brands-split", TraceID: &trace},
		pendingResp: contract.PendingQuestionsResponse{ShopperID: "cmp-brands-split"},
	}
	got := armResponse(mem)
	if !strings.HasPrefix(got, "recall_for_purchase response:\n") {
		t.Errorf("default arm response does not open with the T5.8 label:\n%s", got)
	}
	if strings.Contains(got, "pre-settle") || strings.Contains(got, "post-settle") {
		t.Errorf("default arm response carries probe labels:\n%s", got)
	}
	if !strings.Contains(got, "\npending_questions response:\n") {
		t.Errorf("default arm response misses the pending-questions block:\n%s", got)
	}
}
