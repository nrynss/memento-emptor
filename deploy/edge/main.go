// Command edge is the public front door of the single Render web
// service: it serves the ledger single-page app, proxies the memory
// layer's API on the same origin, and maps the PayPal webhook path to
// the layer's frozen ingest route. It carries no product logic: every
// API answer comes from the service behind -backend, and the webhook
// shim only rewrites the path because the frozen route predates the
// public URL pattern.
package main

import (
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

// proxyRoutes are the layer's own routes, served through the backend.
// Anything else is a static asset or a client-side route.
var proxyRoutes = []string{"/mcp", "/tools/", "/calls", "/webhooks/payment"}

// webhookPublicPath is the PayPal-registered URL pattern from the
// hosting brief. The frozen service answers it one segment over, so
// the edge rewrites the path and nothing else.
const webhookPublicPath = "/webhooks/paypal"

func main() {
	addr := flag.String("addr", envOr("EDGE_ADDR", ":"+envOr("PORT", "8080")), "address to listen on")
	staticDir := flag.String("static", envOr("EDGE_STATIC", "/app/web"), "directory holding the ledger build")
	backend := flag.String("backend", envOr("MEMENTO_BACKEND", "http://127.0.0.1:18080"), "memory layer base URL")
	flag.Parse()

	target, err := url.Parse(*backend)
	if err != nil {
		log.Fatalf("edge: bad backend %q: %v", *backend, err)
	}
	proxy := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			if r.URL.Path == webhookPublicPath {
				r.URL.Path = "/webhooks/payment"
			}
			r.URL.Scheme = target.Scheme
			r.URL.Host = target.Host
			r.Host = target.Host
		},
		Transport: &http.Transport{
			ResponseHeaderTimeout: 10 * time.Second,
			IdleConnTimeout:       60 * time.Second,
		},
		ErrorLog: log.New(os.Stderr, "edge: proxy: ", log.LstdFlags),
	}

	files := http.FileServer(http.Dir(*staticDir))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("POST "+webhookPublicPath, proxy.ServeHTTP)
	for _, route := range proxyRoutes {
		pattern := route
		if strings.HasSuffix(pattern, "/") {
			mux.HandleFunc("POST "+pattern+"{rest...}", proxy.ServeHTTP)
			continue
		}
		mux.Handle(pattern, proxy)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && routeIsAPI(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		clean := path.Clean(r.URL.Path)
		if directFile(*staticDir, clean) {
			files.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, *staticDir+"/200.html")
	})

	log.Printf("edge: listening on %s (static %s, backend %s)", *addr, *staticDir, *backend)
	if err := http.ListenAndServe(*addr, withLog(mux)); err != nil {
		log.Fatalf("edge: %v", err)
	}
}

// routeIsAPI reports whether a path belongs to the backend surface, so
// unknown API paths 404 instead of serving the app shell.
func routeIsAPI(p string) bool {
	if p == "/mcp" || p == "/tools" || p == "/calls" || p == webhookPublicPath {
		return true
	}
	for _, route := range proxyRoutes {
		if strings.HasPrefix(p, route) {
			return true
		}
	}
	return false
}

// directFile reports whether a cleaned request path names a real file
// under the static dir. Directories resolve to their index, like the
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
// only: bodies can carry shopper words and are never logged.
func withLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("edge: %s %s -> %d", r.Method, r.URL.Path, rec.status)
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
