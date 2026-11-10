package payments

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Delivery is one raw webhook delivery exactly as the transport received
// it: the request headers and the body bytes the sender posted. The header
// names and the body are the source's, so the verifier extracts the signed
// fields and never a caller.
type Delivery struct {
	// Header carries the request headers, signature fields included.
	Header http.Header
	// Body is the exact body the sender posted. The signature covers
	// these bytes, so a re-encoded or reformatted body does not verify.
	Body []byte
}

// Header names the signature fields travel in. They are the source's
// transport contract, not a layer concept, so they stay in this package.
const (
	headerTransmissionID   = "Paypal-Transmission-Id"
	headerTransmissionTime = "Paypal-Transmission-Time"
	headerCertURL          = "Paypal-Cert-Url"
	headerAuthAlgo         = "Paypal-Auth-Algo"
	headerTransmissionSig  = "Paypal-Transmission-Sig"
)

// signedFields are the transmission values one delivery signs with.
type signedFields struct {
	transmissionID   string
	transmissionTime string
	certURL          string
	authAlgo         string
	signature        string
}

// signed reads the signature fields off the delivery. A missing field or a
// body that is not JSON is ErrMalformedDelivery: the verifier refuses
// without asking the source about a delivery it cannot describe.
func (d Delivery) signed() (signedFields, error) {
	fields := signedFields{
		transmissionID:   strings.TrimSpace(d.Header.Get(headerTransmissionID)),
		transmissionTime: strings.TrimSpace(d.Header.Get(headerTransmissionTime)),
		certURL:          strings.TrimSpace(d.Header.Get(headerCertURL)),
		authAlgo:         strings.TrimSpace(d.Header.Get(headerAuthAlgo)),
		signature:        strings.TrimSpace(d.Header.Get(headerTransmissionSig)),
	}
	if fields.transmissionID == "" || fields.transmissionTime == "" || fields.certURL == "" ||
		fields.authAlgo == "" || fields.signature == "" {
		return signedFields{}, fmt.Errorf("%w: missing transmission header", ErrMalformedDelivery)
	}
	if len(d.Body) == 0 {
		return signedFields{}, fmt.Errorf("%w: empty body", ErrMalformedDelivery)
	}
	if !json.Valid(d.Body) {
		return signedFields{}, fmt.Errorf("%w: body is not JSON", ErrMalformedDelivery)
	}
	return fields, nil
}
