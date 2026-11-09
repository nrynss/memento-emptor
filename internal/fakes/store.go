package fakes

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ScoredDoc is one recorded search hit: the document, its score, and the
// text the score was measured on.
type ScoredDoc struct {
	// DocID identifies the recorded document.
	DocID string
	// Score is the recorded score on the kNN leg.
	Score float64
	// Text is the document text.
	Text string
}

// EvidenceStore is the seam the real evidence-store client implements in
// P2: raw evidence documents with their vectors, and the hybrid search
// that matches a new product to the items behind a belief. Indexing
// always succeeds in memory; searching an unrecorded query fails with
// ErrNoRecording.
type EvidenceStore interface {
	// IndexDoc stores one evidence text and returns its document id.
	// Lambo evidence concepts carry this id, so prose stays out of the
	// graph.
	IndexDoc(ctx context.Context, text string) (string, error)
	// Search replays the recorded kNN leg for one recorded query.
	Search(ctx context.Context, query string) ([]ScoredDoc, error)
}

// RecordingEvidenceStore is an EvidenceStore that replays the hybrid
// search recordings: the documents from hybrid-documents.json and the kNN
// rankings from hybrid-search-results.json. Unknown queries fail with
// ErrNoRecording.
type RecordingEvidenceStore struct {
	texts   map[string]string
	ranks   map[string][]ScoredDoc
	indexed []string
}

// NewRecordingEvidenceStore loads the evidence recordings under dir:
// hybrid-documents.json for the texts, hybrid-search-results.json for the
// rankings.
func NewRecordingEvidenceStore(dir string) (*RecordingEvidenceStore, error) {
	docsRaw, err := os.ReadFile(filepath.Join(dir, "hybrid-documents.json"))
	if err != nil {
		return nil, fmt.Errorf("fakes: read evidence documents: %w", err)
	}
	var docs struct {
		Documents []struct {
			DocID string `json:"doc_id"`
			Text  string `json:"text"`
		} `json:"documents"`
	}
	if err := json.Unmarshal(docsRaw, &docs); err != nil {
		return nil, fmt.Errorf("fakes: parse evidence documents: %w", err)
	}
	resultsRaw, err := os.ReadFile(filepath.Join(dir, "hybrid-search-results.json"))
	if err != nil {
		return nil, fmt.Errorf("fakes: read evidence rankings: %w", err)
	}
	var results struct {
		Queries []string `json:"queries"`
		Legs    map[string]struct {
			HTTP int      `json:"http"`
			Hits [][2]any `json:"hits"`
		} `json:"legs"`
	}
	if err := json.Unmarshal(resultsRaw, &results); err != nil {
		return nil, fmt.Errorf("fakes: parse evidence rankings: %w", err)
	}
	fake := &RecordingEvidenceStore{
		texts: map[string]string{},
		ranks: map[string][]ScoredDoc{},
	}
	for _, doc := range docs.Documents {
		fake.texts[doc.DocID] = doc.Text
	}
	for _, query := range results.Queries {
		leg, ok := results.Legs["knn:"+query]
		if !ok || leg.HTTP != 200 {
			continue
		}
		var hits []ScoredDoc
		for _, hit := range leg.Hits {
			id, _ := hit[0].(string)
			score, _ := hit[1].(float64)
			hits = append(hits, ScoredDoc{DocID: id, Score: score, Text: fake.texts[id]})
		}
		fake.ranks[query] = hits
	}
	if len(fake.ranks) == 0 {
		return nil, fmt.Errorf("fakes: no recorded search rankings in %s", dir)
	}
	return fake, nil
}

// IndexDoc stores one evidence text in memory and returns its document
// id. The id is story-scoped; the recorded search rankings only cover the
// recorded documents.
func (f *RecordingEvidenceStore) IndexDoc(ctx context.Context, text string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("fakes: index document: %w", err)
	}
	f.indexed = append(f.indexed, text)
	return fmt.Sprintf("doc-%04d", len(f.indexed)), nil
}

// Search replays the recorded kNN ranking for one recorded query, or
// ErrNoRecording when no recording covers the query.
func (f *RecordingEvidenceStore) Search(ctx context.Context, query string) ([]ScoredDoc, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("fakes: search evidence: %w", err)
	}
	hits, ok := f.ranks[query]
	if !ok {
		return nil, fmt.Errorf("%w: search query %q", ErrNoRecording, query)
	}
	out := make([]ScoredDoc, len(hits))
	copy(out, hits)
	return out, nil
}
