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

// KeyVariables lists every environment variable the real clients read.
// The replay asserts all of them unset or empty before it runs.
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
