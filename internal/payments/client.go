package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// DefaultBaseURL is the payment source's sandbox REST root. Sandbox only:
// no live host or live credential belongs in this repository.
const DefaultBaseURL = "https://api-m.sandbox.paypal.com"

// VerifyPath is the signature endpoint one delivery is checked against.
const VerifyPath = "/v1/notifications/verify-webhook-signature"

// TokenPath mints the app access token the verify call travels with.
const TokenPath = "/v1/oauth2/token"

// WebhooksPath lists the app's webhook subscriptions, so the one a
// delivery was signed for can be named.
const WebhooksPath = "/v1/notifications/webhooks"

// publicWebhookPath is the PayPal-registered URL pattern the deployment
// serves, used to pick one subscription when PUBLIC_URL is set.
const publicWebhookPath = "/webhooks/paypal"

// tokenSkew renews the access token this long before it expires, so a
// call never races the source's expiry.
const tokenSkew = 60 * time.Second

// defaultTimeout bounds one token grant, discovery or verify call.
const defaultTimeout = 15 * time.Second

// Verifier is the gate a raw delivery passes before anything reads it.
// Implementations return nil only when the delivery's signature verified;
// every other answer is an error and the caller refuses the delivery.
type Verifier interface {
	// Verify checks one delivery's signature against the payment source.
	Verify(ctx context.Context, d Delivery) error
}

// Client calls the source's verify-webhook-signature endpoint. It is safe
// for concurrent use and caches the access token and the candidate webhook
// subscriptions between calls.
type Client struct {
	http      *http.Client
	baseURL   string
	clientID  string
	secret    string
	webhookID string
	catalog   Catalog

	tokenMu    sync.Mutex
	token      string
	tokenUntil time.Time

	hookMu sync.Mutex
	hooks  []string
}

// Compile-time proof the client satisfies the delivery gate.
var _ Verifier = (*Client)(nil)

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient sends calls through hc. Tests point it at a stand-in
// source; production leaves it unset for the default client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.http = hc
		}
	}
}

// WithBaseURL replaces the sandbox REST root. Tests use a stand-in server.
func WithBaseURL(base string) Option {
	return func(c *Client) {
		if base != "" {
			c.baseURL = strings.TrimRight(base, "/")
		}
	}
}

// WithCredentials sets the app credentials instead of reading them from
// the environment.
func WithCredentials(clientID, secret string) Option {
	return func(c *Client) {
		c.clientID = clientID
		c.secret = secret
	}
}

// WithWebhookID names the subscription a delivery is expected to belong
// to, instead of discovering it from the app's subscriptions.
func WithWebhookID(id string) Option {
	return func(c *Client) {
		c.webhookID = id
	}
}

// New returns a verifier over the sandbox app. Credentials come from
// PAYPAL_CLIENT_ID and PAYPAL_CLIENT_SECRET; when either is empty every
// verification is refused with ErrUnavailable. The subscription id is
// discovered on the first verification unless WithWebhookID names it.
func New(opts ...Option) *Client {
	c := &Client{
		http:     &http.Client{Timeout: defaultTimeout},
		baseURL:  DefaultBaseURL,
		clientID: os.Getenv("PAYPAL_CLIENT_ID"),
		secret:   os.Getenv("PAYPAL_CLIENT_SECRET"),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Verify checks one delivery's signature. It returns nil only when the
// source answers SUCCESS for the exact body bytes; every other answer is
// an error, and a verifier that cannot reach the source refuses too.
func (c *Client) Verify(ctx context.Context, d Delivery) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("payments: verify delivery: %w", err)
	}
	fields, err := d.signed()
	if err != nil {
		return err
	}
	if c.clientID == "" || c.secret == "" {
		return fmt.Errorf("%w: PAYPAL_CLIENT_ID and PAYPAL_CLIENT_SECRET are not set", ErrUnavailable)
	}
	subscriptions, err := c.subscriptions(ctx)
	if err != nil {
		return err
	}
	var refusal error
	for _, subscription := range subscriptions {
		payload := verificationPayload(fields, subscription, d.Body)
		status, err := c.check(ctx, payload)
		if err != nil && !errors.Is(err, ErrNotVerified) {
			return err
		}
		if err == nil && status == http.StatusUnauthorized {
			// A cached token the source no longer honours is renewed once;
			// a second refusal is the source's answer and refuses the
			// delivery.
			c.forgetToken()
			status, err = c.check(ctx, payload)
			if err != nil && !errors.Is(err, ErrNotVerified) {
				return err
			}
		}
		if err == nil && status == http.StatusOK {
			return nil
		}
		if errors.Is(err, ErrNotVerified) {
			refusal = err
			continue
		}
		return fmt.Errorf("%w: verify endpoint answered %d", ErrUnavailable, status)
	}
	if refusal == nil {
		refusal = fmt.Errorf("%w: no subscription answered", ErrNotVerified)
	}
	return refusal
}

// check posts one verification payload and reports the endpoint's status,
// refusing anything but SUCCESS. One call never retries: Verify renews a
// rejected token and asks again itself.
func (c *Client) check(ctx context.Context, payload []byte) (int, error) {
	token, err := c.bearer(ctx)
	if err != nil {
		return 0, err
	}
	status, body, err := c.post(ctx, VerifyPath, token, payload)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	if status == http.StatusOK {
		return status, verifyAnswer(body)
	}
	if status/100 == 2 {
		return status, fmt.Errorf("%w: verify endpoint answered %d", ErrUnavailable, status)
	}
	return status, nil
}

// verifyAnswer reads the endpoint's verdict. Anything but SUCCESS is a
// refusal, including a reply that names no status at all.
func verifyAnswer(body []byte) error {
	var answer struct {
		Status string `json:"verification_status"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return fmt.Errorf("%w: unreadable verify reply", ErrUnavailable)
	}
	if answer.Status != "SUCCESS" {
		return fmt.Errorf("%w: verification status %q", ErrNotVerified, answer.Status)
	}
	return nil
}

// verificationPayload renders the verify request around the delivery's
// exact body bytes. The signature covers those bytes, so the body is
// embedded as delivered, never decoded and re-encoded.
func verificationPayload(fields signedFields, webhookID string, body []byte) []byte {
	quoted := func(value string) []byte {
		raw, _ := json.Marshal(value)
		return raw
	}
	var payload []byte
	payload = append(payload, `{"transmission_id":`...)
	payload = append(payload, quoted(fields.transmissionID)...)
	payload = append(payload, `,"transmission_time":`...)
	payload = append(payload, quoted(fields.transmissionTime)...)
	payload = append(payload, `,"cert_url":`...)
	payload = append(payload, quoted(fields.certURL)...)
	payload = append(payload, `,"auth_algo":`...)
	payload = append(payload, quoted(fields.authAlgo)...)
	payload = append(payload, `,"transmission_sig":`...)
	payload = append(payload, quoted(fields.signature)...)
	payload = append(payload, `,"webhook_id":`...)
	payload = append(payload, quoted(webhookID)...)
	payload = append(payload, `,"webhook_event":`...)
	payload = append(payload, body...)
	payload = append(payload, '}')
	return payload
}

// bearer returns a cached access token or mints a new one.
func (c *Client) bearer(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenUntil) {
		return c.token, nil
	}
	return c.grant(ctx)
}

// grant mints an app access token. The caller holds tokenMu.
func (c *Client) grant(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+TokenPath, strings.NewReader("grant_type=client_credentials"))
	if err != nil {
		return "", fmt.Errorf("%w: build token request", ErrUnavailable)
	}
	req.SetBasicAuth(c.clientID, c.secret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: token grant: %w", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("%w: read token grant: %w", ErrUnavailable, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: token grant answered %d", ErrUnavailable, resp.StatusCode)
	}
	var parsed struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || parsed.AccessToken == "" {
		return "", fmt.Errorf("%w: token grant carried no token", ErrUnavailable)
	}
	c.token = parsed.AccessToken
	if parsed.ExpiresIn > 0 {
		c.tokenUntil = time.Now().Add(time.Duration(parsed.ExpiresIn)*time.Second - tokenSkew)
	} else {
		c.tokenUntil = time.Time{}
	}
	return c.token, nil
}

// forgetToken drops the cached token so the next call mints a new one.
func (c *Client) forgetToken() {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	c.token = ""
	c.tokenUntil = time.Time{}
}

// subscriptions returns the webhook subscriptions a delivery may have
// been signed for: the named one, the one serving the configured public
// URL, or every subscription the app has. Verify tries them in order and
// stops at the first SUCCESS, because the source answers SUCCESS only for
// the subscription that signed the delivery.
func (c *Client) subscriptions(ctx context.Context) ([]string, error) {
	c.hookMu.Lock()
	defer c.hookMu.Unlock()
	if len(c.hooks) > 0 {
		return c.hooks, nil
	}
	if c.webhookID != "" {
		c.hooks = []string{c.webhookID}
		return c.hooks, nil
	}
	token, err := c.bearer(ctx)
	if err != nil {
		return nil, err
	}
	status, raw, err := c.get(ctx, WebhooksPath, token)
	if err != nil {
		return nil, fmt.Errorf("%w: list webhooks: %w", ErrUnavailable, err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("%w: list webhooks answered %d", ErrUnavailable, status)
	}
	var listed struct {
		Webhooks []webhook `json:"webhooks"`
	}
	if err := json.Unmarshal(raw, &listed); err != nil {
		return nil, fmt.Errorf("%w: unreadable webhook list", ErrUnavailable)
	}
	ids, err := subscriptionIDs(listed.Webhooks, os.Getenv("PUBLIC_URL"))
	if err != nil {
		return nil, err
	}
	c.hooks = ids
	return c.hooks, nil
}

// webhook is one subscription in the app's webhook list.
type webhook struct {
	// ID is the subscription id the verify call names.
	ID string `json:"id"`
	// URL is the endpoint the subscription delivers to.
	URL string `json:"url"`
}

// subscriptionIDs lists the subscriptions whose id a delivery may be
// checked against. A configured public URL names its own subscription and
// nothing else, so a deployment refuses deliveries signed for a stale
// subscription. Without one, every subscription the app has is a
// candidate, because all of them belong to the same app and the source
// answers SUCCESS for exactly the one that signed the delivery. No
// candidate at all is a refusal: the adapter never guesses an id.
func subscriptionIDs(hooks []webhook, publicURL string) ([]string, error) {
	if publicURL != "" {
		want := strings.TrimRight(publicURL, "/") + publicWebhookPath
		for _, hook := range hooks {
			if hook.URL == want && hook.ID != "" {
				return []string{hook.ID}, nil
			}
		}
		return nil, fmt.Errorf("%w: no subscription serves %s", ErrNoWebhook, want)
	}
	var ids []string
	for _, hook := range hooks {
		if hook.ID != "" {
			ids = append(ids, hook.ID)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: the app has no webhook subscription", ErrNoWebhook)
	}
	return ids, nil
}

// post sends an authenticated JSON body and returns the status and reply.
func (c *Client) post(ctx context.Context, path, token string, body []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return c.reply(req)
}

// get sends an authenticated read and returns the status and reply.
func (c *Client) get(ctx context.Context, path, token string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return c.reply(req)
}

// reply performs one request and reads a bounded reply.
func (c *Client) reply(req *http.Request) (int, []byte, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, raw, nil
}
