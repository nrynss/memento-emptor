package evidence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// recordedLegs is the measured hybrid search from the research
// recordings: the two query texts and the lexical, vector and combined
// legs per query.
type recordedLegs struct {
	Queries []string `json:"queries"`
	Legs    map[string]struct {
		HTTP int      `json:"http"`
		Hits [][2]any `json:"hits"`
	} `json:"legs"`
}

// loadRecordings reads the committed hybrid-search recordings beside the
// repository. Tests replay them; production code never touches this path.
func loadRecordings(t *testing.T) (recordedLegs, map[string]string) {
	t.Helper()
	dir := filepath.Join("..", "..", "testdata", "elastic")
	resultsRaw, err := os.ReadFile(filepath.Join(dir, "hybrid-search-results.json"))
	if err != nil {
		t.Fatalf("read hybrid-search-results.json: %v", err)
	}
	var results struct {
		Queries []string `json:"queries"`
		Legs    map[string]struct {
			HTTP int      `json:"http"`
			Hits [][2]any `json:"hits"`
		} `json:"legs"`
		Combined struct {
			HTTP int      `json:"http"`
			Hits [][2]any `json:"hits"`
		} `json:"retriever_rrf:snug across the chest"`
	}
	if err := json.Unmarshal(resultsRaw, &results); err != nil {
		t.Fatalf("parse hybrid-search-results.json: %v", err)
	}
	legs := recordedLegs{Queries: results.Queries, Legs: results.Legs}
	if len(results.Combined.Hits) > 0 {
		legs.Legs["retriever_rrf:snug across the chest"] = results.Combined
	}
	docsRaw, err := os.ReadFile(filepath.Join(dir, "hybrid-documents.json"))
	if err != nil {
		t.Fatalf("read hybrid-documents.json: %v", err)
	}
	var docs struct {
		Documents []struct {
			DocID string `json:"doc_id"`
			Text  string `json:"text"`
		} `json:"documents"`
	}
	if err := json.Unmarshal(docsRaw, &docs); err != nil {
		t.Fatalf("parse hybrid-documents.json: %v", err)
	}
	texts := map[string]string{}
	for _, doc := range docs.Documents {
		texts[doc.DocID] = doc.Text
	}
	return legs, texts
}

// legIDs returns the recorded hit order for one leg.
func legIDs(t *testing.T, legs recordedLegs, name string) []string {
	t.Helper()
	leg, ok := legs.Legs[name]
	if !ok {
		t.Fatalf("recording has no leg %q", name)
	}
	if leg.HTTP != 200 {
		t.Fatalf("leg %q answers http %d, want 200", name, leg.HTTP)
	}
	var ids []string
	for _, hit := range leg.Hits {
		id, _ := hit[0].(string)
		ids = append(ids, id)
	}
	return ids
}

// aboveBefore reports whether every id in higher ranks above every id in
// lower within the given order.
func aboveBefore(order []string, higher, lower []string) bool {
	rank := map[string]int{}
	for i, id := range order {
		rank[id] = i
	}
	for _, h := range higher {
		for _, l := range lower {
			hi, okH := rank[h]
			li, okL := rank[l]
			if !okH || !okL || hi >= li {
				return false
			}
		}
	}
	return true
}

// TestReplayLexicalLeg replays the research's lexical finding: for a new
// SKU-shaped text, the returned shirts rank above both distractors.
func TestReplayLexicalLeg(t *testing.T) {
	legs, _ := loadRecordings(t)
	order := legIDs(t, legs, "bm25:Brand X oxford shirt")
	sameLine := []string{"p1", "p2", "p3", "p6"}
	distractors := []string{"p4", "p5"}
	if !aboveBefore(order, sameLine, distractors) {
		t.Errorf("lexical order %v puts a distractor above a returned shirt", order)
	}
}

// TestReplayVectorLeg replays the research's vector finding: for a fresh
// snug-chest query, the recorded reason phrases come first, ahead of
// every product text.
func TestReplayVectorLeg(t *testing.T) {
	legs, _ := loadRecordings(t)
	leg := legs.Legs["knn:snug across the chest, pulls at the buttons"]
	if leg.HTTP != 200 {
		t.Fatalf("vector leg answers http %d, want 200", leg.HTTP)
	}
	order := legIDs(t, legs, "knn:snug across the chest, pulls at the buttons")
	if len(order) < 2 || order[0] != "r2" || order[1] != "r1" {
		t.Fatalf("vector order %v, want r2 then r1 first", order)
	}
	score := func(i int) float64 {
		s, _ := leg.Hits[i][1].(float64)
		return s
	}
	if s := score(1); s < 0.84 || s >= 0.85 {
		t.Errorf("second vector hit scores %v, want the recorded 0.84", s)
	}
	products := []string{"p1", "p2", "p3", "p4", "p5", "p6"}
	if !aboveBefore(order, []string{"r2", "r1"}, products) {
		t.Errorf("vector order %v puts a product text above a reason phrase", order)
	}
}

// TestReplayCombinedShape replays the research's combined finding:
// top-level server-side fusion is refused on this deployment, while the
// retriever shape answers with both reason phrases and all four
// same-line shirts above the distractors.
func TestReplayCombinedShape(t *testing.T) {
	legs, _ := loadRecordings(t)
	for _, name := range []string{
		"rrf:Brand X oxford shirt",
		"rrf:snug across the chest, pulls at the buttons",
	} {
		leg, ok := legs.Legs[name]
		if !ok {
			t.Fatalf("recording has no leg %q", name)
		}
		if leg.HTTP != 400 {
			t.Errorf("leg %q answers http %d, want the recorded 400", name, leg.HTTP)
			continue
		}
		body, _ := leg.Hits[0][1].(string)
		if !strings.Contains(body, "parsing_exception") {
			t.Errorf("leg %q 400 body misses the recorded parsing_exception", name)
		}
	}
	combined, ok := legs.Legs["retriever_rrf:snug across the chest"]
	if !ok {
		t.Fatal("recording has no retriever_rrf:snug across the chest leg")
	}
	if combined.HTTP != 200 {
		t.Fatalf("combined leg answers http %d, want 200", combined.HTTP)
	}
	var order []string
	for _, hit := range combined.Hits {
		id, _ := hit[0].(string)
		order = append(order, id)
	}
	if len(order) < 2 || order[0] != "r2" || order[1] != "r1" {
		t.Errorf("combined order %v, want the reason phrases first", order)
	}
	sameLine := []string{"p1", "p2", "p3", "p6"}
	if !aboveBefore(order, append([]string{"r2", "r1"}, sameLine...), []string{"p4", "p5"}) {
		t.Errorf("combined order %v puts a distractor above evidence", order)
	}
}

// TestFuseRecordedLegs replays the queries through this package's own
// fusion: reciprocal rank fusion over the recorded lexical and vector
// legs keeps both reason phrases first and the distractors last.
func TestFuseRecordedLegs(t *testing.T) {
	legs, _ := loadRecordings(t)
	lexical := legIDs(t, legs, "bm25:snug across the chest, pulls at the buttons")
	vector := legIDs(t, legs, "knn:snug across the chest, pulls at the buttons")
	fused := FuseRRF(lexical, vector)
	var order []string
	for _, hit := range fused {
		order = append(order, hit.DocID)
	}
	if len(order) < 2 || order[0] != "r2" || order[1] != "r1" {
		t.Errorf("fused order %v, want the reason phrases first", order)
	}
	if !aboveBefore(order, []string{"r2", "r1", "p1", "p2", "p3", "p6"}, []string{"p4", "p5"}) {
		t.Errorf("fused order %v puts a distractor above evidence", order)
	}
	if len(fused) != 8 {
		t.Errorf("fusion covers %d documents, want all 8 recorded ones once", len(fused))
	}
}

// TestStoreReplaysRecordedTexts proves the store's own search over the
// recorded eight documents: the new-SKU text finds the same-line shirts
// above the distractors, and the fresh reason query finds its recorded
// phrase first.
func TestStoreReplaysRecordedTexts(t *testing.T) {
	_, texts := loadRecordings(t)
	store, err := NewStore(stubContract{kind: "test-kind"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	ids := map[string]string{}
	for _, docID := range []string{"p1", "p2", "p3", "p4", "p5", "r1", "r2", "p6"} {
		id, err := store.IndexDoc(ctx, texts[docID])
		if err != nil {
			t.Fatal(err)
		}
		ids[docID] = id
	}

	hits, err := store.Search(ctx, "Brand X oxford shirt")
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, hit := range hits {
		order = append(order, hit.DocID)
	}
	sameLine := []string{ids["p1"], ids["p2"], ids["p3"], ids["p6"]}
	if !aboveBefore(order, sameLine, []string{ids["p4"], ids["p5"]}) {
		t.Errorf("candidate search %v puts a distractor above a same-line shirt", order)
	}

	hits, err = store.Search(ctx, "snug across the chest, pulls at the buttons")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].DocID != ids["r2"] {
		t.Errorf("reason search ranks %+v first, want the recorded snug-chest phrase", hits)
	}
}
