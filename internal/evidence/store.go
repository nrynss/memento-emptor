package evidence

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// Store is the raw-evidence store: shopper messages, classified reasons
// and product texts with their vectors, behind one embedding contract.
// It implements the fakes.EvidenceStore seam as-is, so the replay that
// runs on the fake runs unchanged on this store.
type Store struct {
	contractKind string
	docs         []Document
}

// Verify at compile time that the store adopts the seam: the replay runs
// on either.
var _ fakes.EvidenceStore = (*Store)(nil)

// NewStore opens an empty evidence store under the given embedding
// contract. The contract carries no model name, dimensions, endpoint or
// key — only its kind, so every session built under it matches or
// re-embeds. A nil contract, a failed validation or an empty kind fails
// with ErrInvalidContract.
func NewStore(contract domain.EmbedderContract) (*Store, error) {
	if err := checkContract(contract); err != nil {
		return nil, err
	}
	return &Store{contractKind: contract.EmbeddingKind()}, nil
}

// checkContract reports whether the contract is usable.
func checkContract(contract domain.EmbedderContract) error {
	if contract == nil {
		return fmt.Errorf("%w: no contract", ErrInvalidContract)
	}
	if err := contract.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidContract, err)
	}
	if strings.TrimSpace(contract.EmbeddingKind()) == "" {
		return fmt.Errorf("%w: contract names no kind", ErrInvalidContract)
	}
	return nil
}

// ContractKind names the embedding contract the store was built under.
func (s *Store) ContractKind() string {
	return s.contractKind
}

// CheckContract refuses a contract the store was not built under with
// ErrContractMismatch. Vectors from one embedding space do not answer in
// another; the caller re-embeds instead of mixing them.
func (s *Store) CheckContract(contract domain.EmbedderContract) error {
	if err := checkContract(contract); err != nil {
		return err
	}
	if contract.EmbeddingKind() != s.contractKind {
		return fmt.Errorf("%w: store holds %q, caller presents %q",
			ErrContractMismatch, s.contractKind, contract.EmbeddingKind())
	}
	return nil
}

// ReEmbed re-keys the store onto a new embedding contract, atomically: an
// unusable contract fails and the store keeps its old kind. The live
// deployment re-computes every stored vector under the new contract at
// this point; this store holds the texts the vectors are computed from,
// so re-keying is the offline half of that operation.
func (s *Store) ReEmbed(contract domain.EmbedderContract) error {
	if err := checkContract(contract); err != nil {
		return err
	}
	s.contractKind = contract.EmbeddingKind()
	return nil
}

// IndexDocument stores one built document and returns its document id.
// Graph stubs carry this id so prose stays out of the graph. The id is
// sequential and deterministic offline.
func (s *Store) IndexDocument(ctx context.Context, doc Document) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("evidence: index document: %w", err)
	}
	if err := doc.Validate(); err != nil {
		return "", err
	}
	doc.DocID = fmt.Sprintf("doc-%04d", len(s.docs)+1)
	s.docs = append(s.docs, doc)
	return doc.DocID, nil
}

// IndexDoc stores one evidence text and returns its document id. It is
// the seam method: a bare text with no line, for callers with no
// structured document to build.
func (s *Store) IndexDoc(ctx context.Context, text string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("evidence: index document: %w", err)
	}
	return s.IndexDocument(ctx, Document{Kind: KindMessage, Text: text})
}

// Search answers one query from the stored documents with a lexical
// overlap score: the share of the query's tokens the document text holds.
// Documents holding nothing score nothing and are omitted; ties break by
// document id so the ranking is deterministic. An empty query fails with
// ErrEmptyQuery.
func (s *Store) Search(ctx context.Context, query string) ([]fakes.ScoredDoc, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("evidence: search evidence: %w", err)
	}
	tokens := tokensOf(query)
	if len(tokens) == 0 {
		return nil, ErrEmptyQuery
	}
	var hits []fakes.ScoredDoc
	for _, doc := range s.docs {
		held := tokensOf(doc.Text)
		match := 0
		for token := range tokens {
			if held[token] {
				match++
			}
		}
		if match == 0 {
			continue
		}
		hits = append(hits, fakes.ScoredDoc{
			DocID: doc.DocID,
			Score: float64(match) / float64(len(tokens)),
			Text:  doc.Text,
		})
	}
	sortScored(hits)
	return hits, nil
}

// LedgerSearch returns the stored documents whose text holds the query,
// in index order. The ledger reads through this: a shopper searching
// their evidence sees the raw rows behind the beliefs. An empty query
// fails with ErrEmptyQuery.
func (s *Store) LedgerSearch(query string) ([]Document, error) {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return nil, ErrEmptyQuery
	}
	var found []Document
	for _, doc := range s.docs {
		if strings.Contains(strings.ToLower(doc.Text), needle) {
			found = append(found, doc)
		}
	}
	return found, nil
}

// Documents returns the stored documents in index order.
func (s *Store) Documents() []Document {
	out := make([]Document, len(s.docs))
	copy(out, s.docs)
	return out
}

// tokensOf splits text into a set of lowercase alphanumeric tokens.
// Sizes such as 32 survive as tokens; punctuation does not.
func tokensOf(text string) map[string]bool {
	tokens := map[string]bool{}
	for _, token := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}) {
		tokens[token] = true
	}
	return tokens
}

// sortScored orders hits by score, highest first, breaking ties by
// document id so the ranking is deterministic.
func sortScored(hits []fakes.ScoredDoc) {
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].DocID < hits[j].DocID
	})
}
