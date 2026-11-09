package evidence

import (
	"fmt"
	"strings"
)

// stubPrefix and stubSuffix bracket the document id inside a graph stub.
const (
	stubPrefix = "evidence["
	stubSuffix = "]"
)

// stubMaxLen bounds a graph stub. Stubs are short and keyed; prose lives
// in the store behind the id.
const stubMaxLen = 128

// GraphStub builds the graph-side content for one stored document: the
// document id plus a short key, and nothing else. The graph holds this
// stub; the prose stays in the store. The key must be short — a label such
// as which return this is — never a sentence from the shopper.
func GraphStub(docID, key string) (string, error) {
	docID = strings.TrimSpace(docID)
	key = strings.TrimSpace(key)
	if docID == "" || key == "" {
		return "", ErrEmptyText
	}
	stub := stubPrefix + docID + stubSuffix + " " + key
	if len(stub) > stubMaxLen {
		return "", fmt.Errorf("%w: graph stub for %q is %d characters, at most %d",
			ErrStubTooLong, docID, len(stub), stubMaxLen)
	}
	return stub, nil
}

// StubDocID returns the document id a graph stub carries, or ErrNoDocID
// when the content is not a stub this package built.
func StubDocID(stub string) (string, error) {
	if !strings.HasPrefix(stub, stubPrefix) {
		return "", fmt.Errorf("%w: %q", ErrNoDocID, stub)
	}
	rest := stub[len(stubPrefix):]
	end := strings.Index(rest, stubSuffix)
	if end <= 0 {
		return "", fmt.Errorf("%w: %q", ErrNoDocID, stub)
	}
	return rest[:end], nil
}
