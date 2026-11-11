package main

import (
	"testing"

	"github.com/nrynss/memento-emptor/internal/fakes"
	"github.com/nrynss/memento-emptor/internal/payments"
)

// TestPaymentSeamsOfflineUseRecording proves the no-credential path keeps
// the recording demo source and leaves the default gate in place. It is
// the path every offline demo and every test run takes.
func TestPaymentSeamsOfflineUseRecording(t *testing.T) {
	t.Setenv("PAYPAL_CLIENT_ID", "")
	t.Setenv("PAYPAL_CLIENT_SECRET", "")
	source, gate := paymentSeams()
	if _, ok := source.(*fakes.RecordingPaymentSource); !ok {
		t.Errorf("offline source = %T, want the recording demo source", source)
	}
	if gate != nil {
		t.Errorf("offline gate = %T, want the source's default", gate)
	}
}

// TestPaymentSeamsLiveUseREST proves the sandbox app's credentials switch
// both the reads and the delivery gate to the real client, sharing one
// access token. Constructing the client touches no network and spends
// nothing.
func TestPaymentSeamsLiveUseREST(t *testing.T) {
	t.Setenv("PAYPAL_CLIENT_ID", "client-id")
	t.Setenv("PAYPAL_CLIENT_SECRET", "client-secret")
	source, gate := paymentSeams()
	client, ok := source.(*payments.Client)
	if !ok {
		t.Fatalf("live source = %T, want the REST client", source)
	}
	if gate != client {
		t.Errorf("live gate = %p, want the same client %p", gate, client)
	}
}

// TestPaymentSeamsHalfConfiguredStayLive proves one missing credential does
// not silently fall back to the recording. The client refuses when used, so
// a half-configured deployment fails loudly instead of replaying fixtures.
func TestPaymentSeamsHalfConfiguredStayLive(t *testing.T) {
	t.Setenv("PAYPAL_CLIENT_ID", "client-id")
	t.Setenv("PAYPAL_CLIENT_SECRET", "")
	source, gate := paymentSeams()
	if _, ok := source.(*payments.Client); !ok {
		t.Errorf("half-configured source = %T, want the REST client that refuses", source)
	}
	if gate == nil {
		t.Error("half-configured gate is nil; want the client that refuses")
	}
}
