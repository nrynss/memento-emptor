package main

import "testing"

// TestWebhookShopperResolvesDisputeCustom proves the director routes a
// dispute by the custom its disputed transaction carries, while a capture
// and a refund keep joining on the resource's top-level custom id.
func TestWebhookShopperResolvesDisputeCustom(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "capture top-level",
			body: `{"event_type":"PAYMENT.CAPTURE.COMPLETED","resource":{"custom_id":"capture_shopper"}}`,
			want: "capture_shopper",
		},
		{
			name: "refund top-level",
			body: `{"event_type":"PAYMENT.CAPTURE.REFUNDED","resource":{"custom_id":"refund_shopper"}}`,
			want: "refund_shopper",
		},
		{
			name: "dispute per transaction",
			body: `{"event_type":"CUSTOMER.DISPUTE.CREATED","resource":{"disputed_transactions":[{"custom":""},{"custom":"dispute_shopper"}]}}`,
			want: "dispute_shopper",
		},
		{
			name: "dispute top-level first",
			body: `{"event_type":"CUSTOMER.DISPUTE.CREATED","resource":{"custom_id":"top_level","disputed_transactions":[{"custom":"per_transaction"}]}}`,
			want: "top_level",
		},
		{
			name: "dispute without custom",
			body: `{"event_type":"CUSTOMER.DISPUTE.CREATED","resource":{"disputed_transactions":[{"custom":""}]}}`,
			want: "",
		},
		{
			name: "capture without custom",
			body: `{"event_type":"PAYMENT.CAPTURE.COMPLETED","resource":{}}`,
			want: "",
		},
		{
			name: "not JSON",
			body: `not json`,
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := webhookShopper([]byte(tc.body)); got != tc.want {
				t.Errorf("webhookShopper = %q, want %q", got, tc.want)
			}
		})
	}
}
