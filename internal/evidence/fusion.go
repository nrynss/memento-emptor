package evidence

import "sort"

// rankFusionBias is the reciprocal-rank-fusion bias: a hit at rank r in
// one leg scores 1/(bias+r). The standard value keeps one leg's top hit
// from outscoring another leg's agreement further down.
const rankFusionBias = 60

// RankedDoc is one fused hit: the document and its combined score.
type RankedDoc struct {
	// DocID identifies the ranked document.
	DocID string
	// Score is the fused score. Higher ranks first.
	Score float64
}

// FuseRRF combines lexical and vector rank lists with reciprocal rank
// fusion: each document scores the sum over the lists of 1/(bias+rank),
// with ranks starting at one. A document missing from a leg scores
// nothing there. Ties break by document id so the fusion is
// deterministic. The combined ranking answers whether a new product
// matches the items behind a belief when the legs agree; either leg
// alone is the lexical or the vector answer.
func FuseRRF(lists ...[]string) []RankedDoc {
	scores := map[string]float64{}
	for _, list := range lists {
		for rank, docID := range list {
			if docID == "" {
				continue
			}
			scores[docID] += 1.0 / float64(rankFusionBias+rank+1)
		}
	}
	fused := make([]RankedDoc, 0, len(scores))
	for docID, score := range scores {
		fused = append(fused, RankedDoc{DocID: docID, Score: score})
	}
	sort.Slice(fused, func(i, j int) bool {
		if fused[i].Score != fused[j].Score {
			return fused[i].Score > fused[j].Score
		}
		return fused[i].DocID < fused[j].DocID
	})
	return fused
}
