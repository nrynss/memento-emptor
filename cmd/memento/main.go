// Command memento runs the memory layer: nine frozen tools over MCP and
// plain HTTP, the call log, and payment webhook ingest. It boots on the
// recording fakes with no key and no network, so the demo story replays
// offline; the live swap replaces the fakes behind the same seams and
// touches no handler. With -seed it loads the frozen demo history first,
// so the ledger has rows before any live event arrives.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/nrynss/memento-emptor/internal/api/server"
	"github.com/nrynss/memento-emptor/internal/domain"
	"github.com/nrynss/memento-emptor/internal/evidence"
	"github.com/nrynss/memento-emptor/internal/fakes"
)

// staticContract names the embedding contract the demo store is built
// under. It carries a kind and nothing else: every session built under
// it matches or re-embeds.
type staticContract struct {
	kind string
}

// EmbeddingKind names the contract family.
func (c staticContract) EmbeddingKind() string { return c.kind }

// Validate reports whether the contract is usable.
func (c staticContract) Validate() error {
	if c.kind == "" {
		return fmt.Errorf("%w: demo contract names no kind", domain.ErrEmptyEmbedderKind)
	}
	return nil
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "address to listen on")
	schemaDir := flag.String("schema-dir", "testdata/api", "directory holding the frozen request schemas")
	jevDir := flag.String("jev-dir", "testdata/jev", "directory holding the classifier recordings")
	seed := flag.Bool("seed", false, "load the frozen demo history before serving")
	shopper := flag.String("shopper", "shopper_01", "shopper the seed loads for")
	flag.Parse()

	if err := run(*addr, *schemaDir, *jevDir, *seed, *shopper); err != nil {
		log.Printf("memento: %v", err)
		os.Exit(1)
	}
}

func run(addr, schemaDir, jevDir string, seedFlag bool, shopper string) error {
	classifier, err := fakes.NewRecordingReasonClassifier(jevDir)
	if err != nil {
		return err
	}
	store, err := evidence.NewStore(staticContract{kind: "offline-demo"})
	if err != nil {
		return err
	}
	var seed server.Seed
	if seedFlag {
		loaded, err := server.SeedFromExamples(schemaDir)
		if err != nil {
			return err
		}
		loaded.ShopperID = shopper
		seed = loaded
	}
	graphContents := fakes.GraphContents()
	if seedFlag {
		graphContents = append(graphContents, server.SeedGraphContents(seed)...)
	}
	svc, err := server.NewService(server.Deps{
		Payments:   fakes.NewRecordingPaymentSource(fakes.DemoStoryInputs()),
		Classifier: classifier,
		Store:      store,
		Graph:      fakes.NewRecordingEvidenceGraph(graphContents, fakes.StoryRecalls()),
		SchemaDir:  schemaDir,
		Clock:      func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return err
	}
	if seedFlag {
		if err := svc.LoadSeed(context.Background(), seed); err != nil {
			return err
		}
	}
	httpServer := &http.Server{Addr: addr, Handler: svc.Handler(), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("memento: listening on %s (seed=%v)", addr, seed)
	return httpServer.ListenAndServe()
}
