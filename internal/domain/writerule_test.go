package domain

import (
	"errors"
	"testing"
)

func TestWriteFor(t *testing.T) {
	cases := []struct {
		signal Signal
		want   WriteOp
	}{
		{SignalClassifiedOutcome, WriteRecordAction},
		{SignalFitFeedback, WriteRecordAction},
		{SignalCorrection, WriteRecordAction},
		{SignalGeneralStatement, WriteDerive},
		{SignalKeptPastWindow, WriteObserve},
	}
	for _, tc := range cases {
		got, err := WriteFor(tc.signal)
		if err != nil {
			t.Errorf("WriteFor(%q) returned error: %v", tc.signal, err)
			continue
		}
		if got != tc.want {
			t.Errorf("WriteFor(%q) = %q, want %q", tc.signal, got, tc.want)
		}
	}
}

func TestSignalsCoverWriteFor(t *testing.T) {
	for _, s := range Signals {
		if _, err := WriteFor(s); err != nil {
			t.Errorf("Signals entry %q has no write rule: %v", s, err)
		}
	}
}

func TestWriteForUnknown(t *testing.T) {
	if _, err := WriteFor("wishful_thinking"); !errors.Is(err, ErrUnknownSignal) {
		t.Fatalf("WriteFor(unknown) error = %v, want ErrUnknownSignal", err)
	}
}

func TestOnlyRecordActionPromotes(t *testing.T) {
	ops := []WriteOp{WriteRecordAction, WriteDerive, WriteObserve}
	for _, op := range ops {
		want := op == WriteRecordAction
		if got := op.CanPromote(); got != want {
			t.Errorf("WriteOp(%q).CanPromote() = %v, want %v", op, got, want)
		}
	}
}
