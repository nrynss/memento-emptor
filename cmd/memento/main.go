// Command memento runs the memory layer: eleven frozen tools over MCP and
// plain HTTP, the call log, and payment webhook ingest. It boots on the
// recording fakes with no key and no network, so the demo story replays
// offline; the live swap replaces the fakes behind the same seams and
// touches no handler. With -seed it loads the frozen demo history first,
// so the ledger has rows before any live event arrives.
//
// With "key issue" it mints one caller key instead of serving: the token
// is printed once, on stdout, and the entry the deployment holds goes to
// stderr. The deployment keeps only the entry; the token is never stored.
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
	"github.com/nrynss/memento-emptor/internal/auth"
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
	if len(os.Args) > 1 && os.Args[1] == "key" {
		if err := runKey(os.Args[2:]); err != nil {
			log.Printf("memento: %v", err)
			os.Exit(1)
		}
		return
	}
	addr := flag.String("addr", "127.0.0.1:8080", "address to listen on")
	schemaDir := flag.String("schema-dir", "testdata/api", "directory holding the frozen request schemas")
	jevDir := flag.String("jev-dir", "testdata/jev", "directory holding the classifier recordings")
	seed := flag.Bool("seed", false, "load the frozen demo history before serving")
	shopper := flag.String("shopper", "shopper_01", "shopper the seed loads for")
	standingDir := flag.String("standing-dir", "", "directory holding the per-shopper standing journals (empty keeps them in memory)")
	flag.Parse()

	if err := run(*addr, *schemaDir, *jevDir, *seed, *shopper, *standingDir); err != nil {
		log.Printf("memento: %v", err)
		os.Exit(1)
	}
}

// runKey mints one caller key: "key issue" with a kind, an id, a seller
// and the shoppers it may act for. The token is printed once, on stdout;
// the entry the deployment holds is printed on stderr. The binding is
// validated by the same check the edge parses, so an issued entry always
// parses.
func runKey(args []string) error {
	if len(args) == 0 || args[0] != "issue" {
		return fmt.Errorf("usage: memento key issue --kind agent|shopper|merchant --id ID --seller SELLER [--shoppers A,B]")
	}
	flags := flag.NewFlagSet("key issue", flag.ContinueOnError)
	kind := flags.String("kind", "", "caller kind: agent, shopper or merchant")
	id := flags.String("id", "", "deployment id for the key")
	seller := flags.String("seller", "", "seller whose layer issues the key")
	shoppers := flags.String("shoppers", "", "comma-separated shoppers the key may act for")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if len(flags.Args()) != 0 {
		return fmt.Errorf("usage: memento key issue --kind agent|shopper|merchant --id ID --seller SELLER [--shoppers A,B]")
	}
	var caller auth.CallerKind
	switch *kind {
	case "agent":
		caller = auth.CallerAgent
	case "shopper":
		caller = auth.CallerShopper
	case "merchant":
		caller = auth.CallerMerchant
	default:
		return fmt.Errorf("memento: unknown key kind %q", *kind)
	}
	var bound []string
	if *shoppers != "" {
		for _, shopper := range splitList(*shoppers) {
			if shopper != "" {
				bound = append(bound, shopper)
			}
		}
	}
	token, entry, err := auth.IssueKey(caller, *id, *seller, bound)
	if err != nil {
		return fmt.Errorf("memento: issue key: %w", err)
	}
	fmt.Println(token)
	fmt.Fprintln(os.Stderr, entry)
	return nil
}

// splitList splits a comma-separated flag value.
func splitList(raw string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(raw); i++ {
		if i == len(raw) || raw[i] == ',' {
			out = append(out, raw[start:i])
			start = i + 1
		}
	}
	return out
}

func run(addr, schemaDir, jevDir string, seedFlag bool, shopper, standingDir string) error {
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
		// The demo leaves the pool unwired: no index, no salt and no
		// catalog adapter, so no pool fields and no fit rows. A deploy
		// wires them; the standing directory alone decides whether the
		// journals survive a restart.
		StandingDir: standingDir,
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
