package fakes

import (
	"errors"
	"fmt"
	"os"
)

// ErrNoRecording is returned when a fake is asked for something no
// recording and no committed story covers. The caller asked outside the
// replayed world; the fake fails loudly instead of inventing an answer.
// Match it with errors.Is.
var ErrNoRecording = errors.New("fakes: no recording covers this request")

// ErrKeysPresent is returned when key variables are set where the replay
// requires them absent. The replay runs offline; a set key means the
// environment, not the failure, is wrong.
var ErrKeysPresent = errors.New("fakes: key variables are set")

// KeyVariables lists the key variables the replay asserts absent before
// it runs: the ones the demo's live counterparts read for the payment
// source, the classifier, the evidence store, the catalog and the host.
// Later-phase keys live outside the replay's world and are not listed:
// the identity salt (T7.2) and the graph hub's MCP token (T8.1), neither
// of which the four fakes read.
func KeyVariables() []string {
	return []string{
		"PAYPAL_CLIENT_ID",
		"PAYPAL_CLIENT_SECRET",
		"CHANNEL3_API_KEY",
		"OPENROUTER_API_KEY",
		"LAMBO_ES_API_KEY",
		"ELASTIC_URL",
		"RENDER_API_KEY",
	}
}

// AssertKeysAbsent reports ErrKeysPresent naming the variables that are
// set and non-empty, or nil when the replay may proceed offline.
func AssertKeysAbsent() error {
	var set []string
	for _, name := range KeyVariables() {
		if os.Getenv(name) != "" {
			set = append(set, name)
		}
	}
	if len(set) > 0 {
		return fmt.Errorf("%w: %v", ErrKeysPresent, set)
	}
	return nil
}
