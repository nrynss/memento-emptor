// The gate for the merchant edge: exactly one backend route exists, the
// merchant key is injected server-side and never read from the caller, and
// the app shell serves without any key configured.
package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testStatic builds a static dir holding an index, the SPA fallback and
// one asset, like the merchant build the image ships.
func testStatic(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"index.html": "<h1>merchant</h1>",
		"200.html":   "<h1>shell</h1>",
		"app.js":     "console.log(1)",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// testLayer starts a fake layer that records what the edge sends it and
// answers one canned product_fit response.
func testLayer(t *testing.T, status int, body string) (*httptest.Server, *http.Request, *[]byte) {
	t.Helper()
	var seenReq *http.Request
	var seenBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenReq = r
		raw, _ := io.ReadAll(r.Body)
		seenBody = raw
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	return server, seenReq, &seenBody
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestHealthzNeedsNoKey(t *testing.T) {
	mux := newMux(testStatic(t), mustURL(t, "http://layer.invalid"), "")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"ok":true`) {
		t.Fatalf("healthz body = %q, want the ok marker", recorder.Body.String())
	}
}

func TestProductFitInjectsTheServiceKey(t *testing.T) {
	var seenAuth, seenPath string
	var seenContentType string
	wrapped := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth, seenPath = r.Header.Get("Authorization"), r.URL.Path
		seenContentType = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"pool":{}}`)
	}))
	t.Cleanup(wrapped.Close)

	mux := newMux(testStatic(t), mustURL(t, wrapped.URL), "service-key")
	request := httptest.NewRequest(http.MethodPost, "/tools/product_fit",
		strings.NewReader(`{"brand_id":"brand-x-001","category":"shirts","fit_handle":"regular","size":"M"}`))
	request.Header.Set("Authorization", "Bearer caller-key-touch-nothing")
	request.Header.Set("X-Memento-Shopper", "shopper_01")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("product_fit status = %d, want 200", recorder.Code)
	}
	if seenPath != "/tools/product_fit" {
		t.Fatalf("layer path = %q, want /tools/product_fit", seenPath)
	}
	if seenAuth != "Bearer service-key" {
		t.Fatalf("layer authorization = %q, want exactly the service key", seenAuth)
	}
	if seenContentType != "application/json" {
		t.Fatalf("layer content type = %q, want application/json", seenContentType)
	}
}

func TestOtherToolsNeverReachTheLayer(t *testing.T) {
	layer, _, _ := testLayer(t, http.StatusOK, `{}`)
	_ = layer
	touched := false
	guarded := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		touched = true
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(guarded.Close)

	mux := newMux(testStatic(t), mustURL(t, guarded.URL), "service-key")
	for _, target := range []string{
		"/tools/recall_for_purchase",
		"/tools/ledger_beliefs",
		"/tools/set_pooling",
		"/tools/not_a_tool",
		"/tools/",
	} {
		request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(`{}`))
		request.Header.Set("Authorization", "Bearer service-key")
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", target, recorder.Code)
		}
	}
	if touched {
		t.Fatal("the backend was touched by a non-product_fit read")
	}
}

func TestProductFitRefusesTheWrongMethod(t *testing.T) {
	touched := false
	guarded := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		touched = true
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(guarded.Close)

	mux := newMux(testStatic(t), mustURL(t, guarded.URL), "service-key")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/tools/product_fit", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("GET product_fit status = %d, want 404", recorder.Code)
	}
	if touched {
		t.Fatal("the backend was touched by a GET")
	}
}

func TestMissingKeyServesTheAppButNotTheRead(t *testing.T) {
	touched := false
	guarded := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		touched = true
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(guarded.Close)

	mux := newMux(testStatic(t), mustURL(t, guarded.URL), "")
	request := httptest.NewRequest(http.MethodPost, "/tools/product_fit", strings.NewReader(`{}`))
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("keyless product_fit status = %d, want 503", recorder.Code)
	}
	if touched {
		t.Fatal("the backend was touched with no key configured")
	}

	landing := httptest.NewRecorder()
	mux.ServeHTTP(landing, httptest.NewRequest(http.MethodGet, "/", nil))
	if landing.Code != http.StatusOK {
		t.Fatalf("landing status = %d, want 200 with no key configured", landing.Code)
	}
}

func TestLayerAnswerPassesThrough(t *testing.T) {
	layer, _, _ := testLayer(t, http.StatusForbidden, `{"error":{"code":"forbidden","message":"no"}}`)
	mux := newMux(testStatic(t), mustURL(t, layer.URL), "service-key")
	request := httptest.NewRequest(http.MethodPost, "/tools/product_fit", strings.NewReader(`{}`))
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("refused read status = %d, want the layer's 403", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "forbidden") {
		t.Fatalf("refused read body = %q, want the layer's answer", recorder.Body.String())
	}
}

func TestUnknownPathsServeTheShell(t *testing.T) {
	mux := newMux(testStatic(t), mustURL(t, "http://layer.invalid"), "")
	for target, want := range map[string]string{
		"/app.js":  "console.log(1)",
		"/belief":  "<h1>shell</h1>",
		"/missing": "<h1>shell</h1>",
	} {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", target, recorder.Code)
		}
		if !strings.Contains(recorder.Body.String(), want) {
			t.Fatalf("%s body = %q, want %q", target, recorder.Body.String(), want)
		}
	}
}
