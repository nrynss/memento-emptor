// Command merchant-edge is the public front door of the merchant web
// service: it serves the merchant single-page app and proxies the layer's
// product_fit read on the same origin. The merchant key lives in the
// service's environment (a Render secret, never a file) and is injected
// server-side; the browser never sees it. No other layer tool is reachable
// through this service: any /tools/{tool} that is not product_fit is
// refused here, before any backend is touched.
package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

// productFitPath is the only backend route this service proxies. The
// merchant app reaches the layer's product_fit through it and nothing else.
const productFitPath = "/tools/product_fit"

// maxProductFitBody bounds the proxied read. A product_fit request names
// one line and size; anything larger is not a product_fit read.
const maxProductFitBody = 64 << 10

// defaultLayerURL is the layer service's public base URL. T9.6 lands the
// pivoted layer on the existing service, so the URL is stable while the
// code behind it changes; the environment overrides it.
const defaultLayerURL = "https://memento-emptor.onrender.com"

func main() {
	addr := flag.String("addr", envOr("MERCHANT_ADDR", ":"+envOr("PORT", "8080")), "address to listen on")
	staticDir := flag.String("static", envOr("MERCHANT_STATIC", "/app/merchant"), "directory holding the merchant build")
	layerURL := flag.String("layer", envOr("MEMENTO_LAYER_URL", defaultLayerURL), "layer base URL serving product_fit")
	flag.Parse()

	merchantKey := os.Getenv("MEMENTO_MERCHANT_KEY")
	target, err := url.Parse(*layerURL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		log.Fatalf("merchant-edge: bad layer URL %q", *layerURL)
	}
	log.Printf("merchant-edge: listening on %s (static %s, layer %s://%s, merchant key configured: %v)",
		*addr, *staticDir, target.Scheme, target.Host, merchantKey != "")

	mux := newMux(*staticDir, target, merchantKey)
	if err := http.ListenAndServe(*addr, withLog(mux)); err != nil {
		log.Fatalf("merchant-edge: %v", err)
	}
}

// newMux builds the merchant routing. Exactly one backend route exists:
// POST /tools/product_fit, proxied with the merchant key. Every other
// /tools/ path is refused here; anything else is a static asset or the app
// shell.
func newMux(staticDir string, layer *url.URL, merchantKey string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealth)
	mux.HandleFunc("POST "+productFitPath, proxyProductFit(layer, merchantKey))
	mux.HandleFunc("/tools/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "unknown_tool", "the merchant app reaches product_fit only")
	})
	files := http.FileServer(http.Dir(staticDir))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean(r.URL.Path)
		if directFile(staticDir, clean) {
			files.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, staticDir+"/200.html")
	})
	return mux
}

// handleHealth answers the deployment check with no state.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

// proxyProductFit forwards one product_fit read to the layer with the
// merchant key. The client-sent authorization, if any, is dropped: the only
// credential the layer sees is the service's own key. With no key
// configured the service still serves the app and says plainly that the
// live read is not wired yet.
func proxyProductFit(layer *url.URL, merchantKey string) http.HandlerFunc {
	client := &http.Client{Timeout: 15 * time.Second}
	return func(w http.ResponseWriter, r *http.Request) {
		if merchantKey == "" {
			writeError(w, http.StatusServiceUnavailable, "key_not_configured",
				"the merchant key is not set on this service yet; the app below reads its bundled snapshot")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxProductFitBody))
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad-request", "the product_fit read is too large")
			return
		}
		out, err := http.NewRequestWithContext(r.Context(), http.MethodPost, layer.String()+productFitPath, strings.NewReader(string(body)))
		if err != nil {
			writeError(w, http.StatusBadGateway, "bad-layer-url", "the layer URL is not usable")
			return
		}
		out.Header.Set("Content-Type", "application/json")
		out.Header.Set("Authorization", "Bearer "+merchantKey)
		resp, err := client.Do(out)
		if err != nil {
			writeError(w, http.StatusBadGateway, "layer-unreachable", "the layer did not answer the read")
			return
		}
		defer resp.Body.Close()
		for _, header := range []string{"Content-Type", "Content-Length"} {
			if value := resp.Header.Get(header); value != "" {
				w.Header().Set(header, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, io.LimitReader(resp.Body, 1<<20))
	}
}

// writeError answers with one small JSON error. Bodies can carry shopper
// words elsewhere in the system; this service never logs or echoes one.
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, `{"error":{"code":"`+code+`","message":"`+message+`"}}`)
}

// directFile reports whether a cleaned request path names a real file under
// the static dir. Directories resolve through the app shell, like the
// static host the demo replaces.
func directFile(staticDir, clean string) bool {
	if clean == "/" {
		return true
	}
	info, err := os.Stat(staticDir + clean)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// withLog records one line per request: method, path and status. Paths
// only: bodies are never logged, and the merchant key never leaves the
// Authorization header it is set on.
func withLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("merchant-edge: %s %s -> %d", r.Method, r.URL.Path, rec.status)
	})
}

// statusRecorder keeps the status the handler wrote.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader records the status before writing it.
func (rec *statusRecorder) WriteHeader(status int) {
	rec.status = status
	rec.ResponseWriter.WriteHeader(status)
}

// envOr reads one environment variable or returns the fallback.
func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
