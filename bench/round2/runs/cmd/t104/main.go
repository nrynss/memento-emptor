// Command t104 drives the T10.4 comparison: every round-2 shopper through
// all four arms, two repeats, plus case 4's correction, next-session and
// other-agent turns, against the frozen round-2 inputs.
//
// The driver is run-record scaffolding only: the shoppers, answers,
// population, prompts, extraction, expected recall and grader are the frozen
// round-2 package's, and the model transport follows the frozen pilot and
// T5.5 drivers call for call. What this driver adds is the live layer for the
// Memento arms (one Lambo hub, one service per shopper, the live fit index
// behind the pool), the in-code cumulative and single-turn spend guards, the
// correction round, and the recorded comparison between the layer's live
// recall and the frozen expectation.
//
// Usage, from the repository root, after `source tools/env.sh`:
//
//	go run ./bench/round2/runs/cmd/t104 -mode one-cell -deployment-id <id>
//	go run ./bench/round2/runs/cmd/t104 -deployment-id <id>
//
// The one-cell mode runs one shopper through all four arms for one repeat,
// records its actual cost and prints the projection against the $5.00 cap.
// The full mode reuses every committed turn file and runs the rest.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nrynss/memento-emptor/bench/harness"
	"github.com/nrynss/memento-emptor/bench/pilot"
	"github.com/nrynss/memento-emptor/bench/round2"
	"github.com/nrynss/memento-emptor/seed"
)

// Caps and guards. cumulativeBudgetUSD is T10.4's binding ceiling across all
// turns; singleTurnGuardUSD aborts the batch when one trace costs more than
// this (pilot turns cost about $0.001-0.002; round-2 prompts are larger, so
// the guard is the backstop, not the expected price).
const (
	// cumulativeBudgetUSD is the hard ceiling across all T10.4 turns.
	cumulativeBudgetUSD = 5.00
	// singleTurnGuardUSD is the per-turn abort guard.
	singleTurnGuardUSD = 0.10
)

// reAsk is the uniform re-ask after a correction, the same words in every
// arm. It repeats the frozen task's cite-only instruction.
const reAsk = "Given that correction, what size do you recommend for the candidate? Cite only what you were given."

// options are the driver's command-line settings.
type options struct {
	outDir         string
	mode           string
	shoppers       []string
	arms           []int
	repeats        []int
	turns          []string
	deploymentID   string
	agentName      string
	lamboBin       string
	work           string
	fitIndex       string
	force          bool
	skipPopulation bool
	keepIndex      bool
	recoverRun     string
	recoverTrace   string
}

// driver is one invocation's shared state.
type driver struct {
	opts       options
	layer      *layer
	client     *proxyClient
	shoppers   map[string]seed.Round2Shopper
	population seed.Population
	spent      float64
}

// fail prints to stderr and exits nonzero.
func fail(err error) {
	fmt.Fprintf(os.Stderr, "t104: %v\n", err)
	os.Exit(1)
}

// main parses the flags and runs one invocation.
func main() {
	outDir := flag.String("out", "bench/round2/runs", "run file output directory")
	mode := flag.String("mode", "run", "run | one-cell | layer")
	shoppersFlag := flag.String("shoppers", "", "comma-separated shopper ids (default every frozen shopper)")
	armsFlag := flag.String("arms", "1,2,3,4", "comma-separated arms")
	repeatsFlag := flag.String("repeats", "1,2", "comma-separated repeat numbers")
	turnsFlag := flag.String("turns", "first,correction,next-session,other-agent", "comma-separated turn kinds")
	deploymentID := flag.String("deployment-id", "", "platform deployment id of the neutral instrument")
	agentName := flag.String("agent-name", "pilot-baseline", "deployed agent name for trace reads")
	lamboBin := flag.String("lambo-bin", "", "lambo binary (default ~/work/lambo/target/release/lambo)")
	work := flag.String("work", "", "layer working directory (default a fresh temp dir)")
	fitIndex := flag.String("fit-index", "t104-fit-rows", "scratch fit index (must start with t104)")
	force := flag.Bool("force", false, "re-run turns whose run file already exists")
	skipPopulation := flag.Bool("skip-population", false, "debug: reuse the scratch index instead of rewriting the population")
	keepIndex := flag.Bool("keep-index", false, "debug: leave the scratch index in place at the end")
	recoverRun := flag.String("recover-run", "", "recover mode: the run id whose trace arrived after the poll deadline")
	recoverTrace := flag.String("recover-trace", "", "recover mode: the full trace id to read")
	flag.Parse()

	arms, err := parseInts(*armsFlag)
	if err != nil {
		fail(err)
	}
	repeats, err := parseInts(*repeatsFlag)
	if err != nil {
		fail(err)
	}
	opts := options{
		outDir:         *outDir,
		mode:           *mode,
		shoppers:       parseList(*shoppersFlag),
		arms:           arms,
		repeats:        repeats,
		turns:          parseList(*turnsFlag),
		deploymentID:   *deploymentID,
		agentName:      *agentName,
		lamboBin:       *lamboBin,
		work:           *work,
		fitIndex:       *fitIndex,
		force:          *force,
		skipPopulation: *skipPopulation,
		keepIndex:      *keepIndex,
		recoverRun:     *recoverRun,
		recoverTrace:   *recoverTrace,
	}
	if opts.lamboBin == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fail(err)
		}
		opts.lamboBin = filepath.Join(home, "work/lambo/target/release/lambo")
	}
	if err := run(opts); err != nil {
		fail(err)
	}
}

// parseList splits a comma-separated value.
func parseList(raw string) []string {
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// parseInts splits and parses a comma-separated integer list.
func parseInts(raw string) ([]int, error) {
	out := []int{}
	for _, part := range parseList(raw) {
		var n int
		if _, err := fmt.Sscanf(part, "%d", &n); err != nil {
			return nil, fmt.Errorf("t104: bad integer %q", part)
		}
		out = append(out, n)
	}
	return out, nil
}

// run is one invocation: open the run directory, boot and seed the live
// layer, then either inspect it or drive the planned turns.
func run(opts options) error {
	ctx := context.Background()
	if os.Getenv("ELASTIC_URL") == "" || os.Getenv("LAMBO_ES_API_KEY") == "" {
		return errors.New("t104: ELASTIC_URL and LAMBO_ES_API_KEY are empty: source tools/env.sh first")
	}
	if os.Getenv("MEMENTO_IDENTITY_SALT") == "" {
		return errors.New("t104: MEMENTO_IDENTITY_SALT is empty: source tools/env.sh first")
	}
	if opts.mode != "layer" && opts.deploymentID == "" {
		return errors.New("t104: -deployment-id is required for a paid run (the pilot-baseline deployment)")
	}
	all, err := round2.LoadShoppers()
	if err != nil {
		return err
	}
	if opts.mode == "recover" {
		return recoverTurn(ctx, opts, all)
	}
	selected, err := selectShoppers(all, opts.shoppers)
	if err != nil {
		return err
	}
	population, err := round2.LoadPopulation()
	if err != nil {
		return err
	}
	marker, err := openRuns(opts.outDir, "T10.4: the round-2 comparison runs (t104 driver)")
	if err != nil {
		return err
	}
	fmt.Printf("runs: %s opened against freeze %s (manifest %s)\n", opts.outDir, marker.FreezeCommit, shortHash(marker.FrozenManifestSHA256))
	if err := runFreezeCheck(); err != nil {
		return err
	}
	if err := os.MkdirAll(opts.outDir, 0o755); err != nil {
		return fmt.Errorf("t104: mkdir %s: %w", opts.outDir, err)
	}
	if opts.mode == "finish" {
		d := &driver{opts: opts, population: population, shoppers: map[string]seed.Round2Shopper{}}
		for _, shopper := range all {
			d.shoppers[shopper.ShopperID] = shopper
		}
		return d.finish(ctx)
	}

	live, err := startLayer(ctx, opts, selected)
	if err != nil {
		return err
	}
	defer live.stop()

	d := &driver{
		opts:       opts,
		layer:      live,
		client:     &proxyClient{deploymentID: opts.deploymentID, http: &http.Client{Timeout: 60 * time.Second}},
		shoppers:   map[string]seed.Round2Shopper{},
		population: population,
	}
	for _, shopper := range all {
		d.shoppers[shopper.ShopperID] = shopper
	}
	if err := d.recordSeedLogs(opts.outDir); err != nil {
		return err
	}
	if err := d.preflight(ctx, selected); err != nil {
		return err
	}
	if opts.mode == "layer" {
		if err := d.checkCorrections(ctx, selected); err != nil {
			return err
		}
		fmt.Println("layer mode: seeds and live recalls only; no paid turn")
		closeIndexUnlessKept(ctx, live, opts)
		return nil
	}
	if opts.mode == "one-cell" {
		if len(selected) != 1 {
			return errors.New("t104: one-cell mode takes exactly one shopper")
		}
		if err := d.oneCell(ctx, selected[0]); err != nil {
			return err
		}
		closeIndexUnlessKept(ctx, live, opts)
		return nil
	}
	if err := d.batch(ctx, selected); err != nil {
		return err
	}
	if err := d.finish(ctx); err != nil {
		return err
	}
	return nil
}

// recoverTurn records one paid turn whose trace appeared after the driver's
// poll deadline. It sends nothing and spends nothing: it reads the existing
// trace and rebuilds the run file the turn would have written, with a note
// naming the delay. The turn is recorded, not retried.
func recoverTurn(ctx context.Context, opts options, shoppers []seed.Round2Shopper) error {
	if opts.recoverRun == "" || opts.recoverTrace == "" {
		return errors.New("t104: recover mode needs -recover-run and -recover-trace")
	}
	path := filepath.Join(opts.outDir, opts.recoverRun+".json")
	record, ok, err := readRunFile(path)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("t104: no run file at %s", path)
	}
	var shopper seed.Round2Shopper
	for _, candidate := range shoppers {
		if candidate.ShopperID == record.ShopperID {
			shopper = candidate
			break
		}
	}
	if shopper.ShopperID == "" {
		return fmt.Errorf("t104: no frozen shopper %q", record.ShopperID)
	}
	traced, err := readTrace(opts.agentName, opts.recoverTrace)
	if err != nil {
		return fmt.Errorf("t104: read recovered trace: %w", err)
	}
	trace := traced.native
	if strings.TrimSpace(trace.reply) == "" {
		return errors.New("t104: the recovered trace carries no reply")
	}
	prompt := record.Prompt
	memory := round2.MemoryFetch{Tool: record.MemoryTool, Response: []byte(record.MemoryResponse)}
	if strings.Contains(prompt, pilot.PlaceholderTrace) || strings.Contains(record.MemoryResponse, pilot.PlaceholderTrace) {
		// The old failure path scrubbed with an empty trace id and
		// strings.Replacer inserted the placeholder everywhere. Arms 1
		// and 2 rebuild byte for byte from the frozen inputs; an arm 3 or
		// 4 prompt carried live memory this tool cannot reconstruct.
		if record.Arm != int(round2.ArmNoMemory) && record.Arm != int(round2.ArmPayPalHistory) {
			return errors.New("t104: the failed record's prompt was mangled and arm 3/4 memory cannot be rebuilt; record the failure in the note instead")
		}
		if record.Turn != round2.TurnFirst {
			return errors.New("t104: only a first turn's prompt can be rebuilt")
		}
		fetch, err := round2.FetchMemory(shopper, round2.Arm(record.Arm), seed.Population{}, round2.TurnPhase(record.Turn))
		if err != nil {
			return err
		}
		block, err := round2.MemorySection(round2.Arm(record.Arm), fetch)
		if err != nil {
			return err
		}
		prompt = round2.BaseSection(shopper) + "\n" + block
		memory = fetch
	}
	rebuilt, err := round2.NewRunFile(record.RunID, shopper, round2.Arm(record.Arm), record.Repeat,
		record.Turn, record.Agent, prompt, memory, trace.reply, trace.cost)
	if err != nil {
		return err
	}
	rebuilt.Extraction.Notes = fmt.Sprintf("trace %s arrived after the driver's trace-poll deadline; its reply and cost are recorded here from the platform trace, not re-sent",
		shortHash(trace.id))
	scrub := scrubber(opts.deploymentID, traced.conversation, trace)
	rebuilt.Prompt = scrub(rebuilt.Prompt)
	rebuilt.MemoryResponse = scrub(rebuilt.MemoryResponse)
	rebuilt.Reply = scrub(rebuilt.Reply)
	if err := writeJSON(path, rebuilt); err != nil {
		return err
	}
	fmt.Printf("recovered %s: reply %d words, cost $%.6f (trace %s)\n", rebuilt.RunID, rebuilt.ReplyWords, rebuilt.TraceCostUSD, shortHash(trace.id))
	return nil
}

// selectShoppers filters the frozen shoppers by the flag.
func selectShoppers(all []seed.Round2Shopper, wanted []string) ([]seed.Round2Shopper, error) {
	if len(wanted) == 0 {
		return all, nil
	}
	byID := map[string]seed.Round2Shopper{}
	for _, shopper := range all {
		byID[shopper.ShopperID] = shopper
	}
	out := make([]seed.Round2Shopper, 0, len(wanted))
	for _, id := range wanted {
		shopper, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("t104: unknown shopper %q", id)
		}
		out = append(out, shopper)
	}
	return out, nil
}

// openRuns opens the run directory against the freeze in this tree, or
// reuses the marker already there when it still names this freeze. The
// marker binds the directory to one freeze: a resume of the same run keeps
// the original marker, and a freeze edited mid-run fails here instead of
// being silently re-bound by a rewritten marker. A directory with no marker
// is opened with BeginRuns.
func openRuns(outDir, note string) (round2.RunsMarker, error) {
	freeze, err := freezeCommit()
	if err != nil {
		return round2.RunsMarker{}, err
	}
	path := filepath.Join(outDir, round2.RunsMarkerFile)
	if _, statErr := os.Stat(path); statErr != nil {
		if !os.IsNotExist(statErr) {
			return round2.RunsMarker{}, fmt.Errorf("t104: inspect runs marker: %w", statErr)
		}
		return round2.BeginRuns(outDir, freeze, note)
	}
	marker, err := round2.ReadRunsMarker(outDir)
	if err != nil {
		return round2.RunsMarker{}, fmt.Errorf("t104: the run directory is already open; move it aside to start a new run: %w", err)
	}
	return marker, nil
}

// freezeCommit finds the commit that added bench/round2/FROZEN.md, the
// freeze's landing commit. It uses --diff-filter=A so a later prose
// amendment does not re-label the freeze the run directory was opened
// against; the manifest hash in the marker is the binding that holds on
// every clone (FROZEN.md section 8).
func freezeCommit() (string, error) {
	out, err := exec.Command("git", "log", "--diff-filter=A", "--format=%H", "--", "bench/round2/FROZEN.md").Output()
	if err != nil {
		return "", fmt.Errorf("t104: find freeze commit: %w", err)
	}
	lines := strings.Fields(strings.TrimSpace(string(out)))
	if len(lines) == 0 {
		return "", errors.New("t104: no commit added bench/round2/FROZEN.md")
	}
	return lines[len(lines)-1], nil
}

// runFreezeCheck runs the frozen check with the run directory open.
func runFreezeCheck() error {
	cmd := exec.Command("bench/round2/freeze_check.sh", "--allow-runs")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("t104: freeze check failed: %w", err)
	}
	return nil
}

// shortHash shortens a hash for a log line.
func shortHash(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12]
}

// preflight proves the layer answers for every shopper before the first paid
// turn: one live recall each, decoded and counted. It spends no model money.
func (d *driver) preflight(ctx context.Context, shoppers []seed.Round2Shopper) error {
	for _, shopper := range shoppers {
		raw, err := d.layer.recall(ctx, shopper)
		if err != nil {
			return fmt.Errorf("t104: preflight recall for %s: %w", shopper.ShopperID, err)
		}
		var response struct {
			ShopperID string `json:"shopper_id"`
			Beliefs   []struct {
				Value      string `json:"value"`
				Status     string `json:"status"`
				Standing   string `json:"standing"`
				Evidence   []any  `json:"evidence"`
				Precedence *struct {
					Step string `json:"step"`
				} `json:"precedence"`
			} `json:"beliefs"`
			Pool *struct {
				Status    string `json:"status"`
				Direction string `json:"direction"`
			} `json:"pool"`
			PoolLookup  string `json:"pool_lookup"`
			Pending     int    `json:"pending_count"`
			Corrections any    `json:"overriding_correction"`
		}
		if err := json.Unmarshal(raw, &response); err != nil {
			return fmt.Errorf("t104: preflight recall %s: %w", shopper.ShopperID, err)
		}
		if response.ShopperID != shopper.ShopperID {
			return fmt.Errorf("t104: preflight recall answered %q for %q", response.ShopperID, shopper.ShopperID)
		}
		beliefs := make([]string, 0, len(response.Beliefs))
		for _, belief := range response.Beliefs {
			step := ""
			if belief.Precedence != nil {
				step = belief.Precedence.Step
			}
			beliefs = append(beliefs, fmt.Sprintf("%q %s/%s %d %s", belief.Value, belief.Status, belief.Standing, len(belief.Evidence), step))
		}
		pool := "none"
		if response.Pool != nil {
			pool = response.Pool.Status + " " + response.Pool.Direction
		}
		fmt.Printf("preflight %s: pool=%s (%s) pending=%d beliefs=[%s]\n",
			shopper.ShopperID, pool, response.PoolLookup, response.Pending, strings.Join(beliefs, "; "))
		expected, err := round2.ExpectedRecall(shopper, d.population, round2.PhaseBeforeCorrection)
		if err != nil {
			return fmt.Errorf("t104: expected recall for %s: %w", shopper.ShopperID, err)
		}
		differences, err := diffRecall(expected, raw)
		if err != nil {
			return fmt.Errorf("t104: compare preflight recall %s: %w", shopper.ShopperID, err)
		}
		fmt.Printf("preflight %s: %d differences from the frozen expectation\n", shopper.ShopperID, len(differences))
		for i, difference := range differences {
			if i >= 6 {
				fmt.Printf("  … and %d more\n", len(differences)-i)
				break
			}
			fmt.Printf("  %s\n", difference)
		}
	}
	return nil
}

// checkCorrections proves the case-4 path before any paid turn: record the
// shopper's correction through the layer, then read the post-correction
// recall and compare it with the frozen after-phase expectation. It is
// layer-mode only, because recording a correction before the batch would
// answer the first turn after the shopper spoke.
func (d *driver) checkCorrections(ctx context.Context, shoppers []seed.Round2Shopper) error {
	for _, shopper := range shoppers {
		if shopper.Correction == nil {
			continue
		}
		if err := d.recordCorrection(ctx, shopper); err != nil {
			return err
		}
		raw, err := d.layer.recall(ctx, shopper)
		if err != nil {
			return fmt.Errorf("t104: post-correction recall for %s: %w", shopper.ShopperID, err)
		}
		expected, err := round2.ExpectedRecall(shopper, d.population, round2.PhaseAfterCorrection)
		if err != nil {
			return err
		}
		differences, err := diffRecall(expected, raw)
		if err != nil {
			return err
		}
		fmt.Printf("correction-check %s: %d differences after the correction\n", shopper.ShopperID, len(differences))
		for i, difference := range differences {
			if i >= 8 {
				fmt.Printf("  ... and %d more\n", len(differences)-i)
				break
			}
			fmt.Printf("  %s\n", difference)
		}
	}
	return nil
}

// recordSeedLogs commits the per-shopper seed summary.
func (d *driver) recordSeedLogs(outDir string) error {
	if len(seedLogs) == 0 {
		return nil
	}
	return writeJSON(filepath.Join(outDir, "r2-seed-log.json"), map[string]any{
		"note":  "What each frozen history filed in the live layer: LoadSeed replays the seed through the layer's real write path.",
		"seeds": seedLogs,
	})
}

// oneCell runs one shopper through the four arms for repeat 1 and prints the
// projection against the $5.00 cap. It writes the same run files the batch
// reads, so the batch reuses them.
func (d *driver) oneCell(ctx context.Context, shopper seed.Round2Shopper) error {
	billingBefore, err := refreshLogin(ctx)
	if err != nil {
		return err
	}
	spentBefore, err := committedSpend(d.opts.outDir)
	if err != nil {
		return err
	}
	d.spent = spentBefore
	fmt.Printf("one cell: %s through arms %v, repeat 1, first turn only\n", shopper.ShopperID, d.opts.arms)
	costs := map[int]float64{}
	for _, armNum := range d.opts.arms {
		record, cost, err := d.driveTurn(ctx, shopper, round2.Arm(armNum), 1, round2.TurnFirst)
		if err != nil {
			return err
		}
		costs[armNum] += cost
		fmt.Printf("one cell: arm %d %s cost $%.6f (%d prompt words, %d reply words)\n",
			armNum, record.RunID, cost, record.PromptWords, record.ReplyWords)
	}
	billingAfter, err := refreshLogin(ctx)
	if err != nil {
		return err
	}
	projection := 0.0
	worst := 0.0
	for _, cost := range costs {
		mean := cost
		projection += mean * 28
		if cost > worst {
			worst = cost
		}
	}
	fmt.Printf("one cell spend: $%.6f (committed before $%.6f; billing usage before $%.4f, after $%.4f, delta $%.4f)\n",
		sum(costs), spentBefore, billingBefore.usage, billingAfter.usage, billingAfter.usage-billingBefore.usage)
	fmt.Printf("projection at 112 turns: $%.4f (mean of the four arm turns; conservative max-turn projection $%.4f)\n",
		projection, worst*112)
	fmt.Printf("cumulative cap: $%.2f; committed before this cell $%.6f; account usage $%.2f",
		cumulativeBudgetUSD, spentBefore, billingAfter.usage)
	if billingAfter.hasCr {
		fmt.Printf("; credit left $%.2f", billingAfter.credit)
	}
	fmt.Println()
	if projection > cumulativeBudgetUSD || worst*112 > cumulativeBudgetUSD {
		return fmt.Errorf("t104: measured projection $%.4f (conservative $%.4f) does not fit the $%.2f cap: stopping before the batch",
			projection, worst*112, cumulativeBudgetUSD)
	}
	if billingAfter.hasCr && projection+0.5 > billingAfter.credit {
		return fmt.Errorf("t104: measured projection $%.4f leaves under $0.50 of the account's $%.2f credit: stopping before the batch",
			projection, billingAfter.credit)
	}
	return nil
}

// sum totals one map's values.
func sum(values map[int]float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total
}

// batch runs every planned turn that is not already committed. Each
// shopper's first turns run before any correction is recorded, so a first
// turn can never be answered from a post-correction memory.
func (d *driver) batch(ctx context.Context, shoppers []seed.Round2Shopper) error {
	spent, err := committedSpend(d.opts.outDir)
	if err != nil {
		return err
	}
	d.spent = spent
	fmt.Printf("batch: committed spend in %s $%.6f of $%.2f\n", d.opts.outDir, spent, cumulativeBudgetUSD)
	for _, shopper := range shoppers {
		if err := d.runShopper(ctx, shopper); err != nil {
			return err
		}
	}
	return nil
}

// runShopper drives one shopper's cells in three phases: every first turn,
// then the correction (case 4 only), then every later turn.
func (d *driver) runShopper(ctx context.Context, shopper seed.Round2Shopper) error {
	type cell struct {
		arm    round2.Arm
		repeat int
	}
	cells := []cell{}
	for _, armNum := range d.opts.arms {
		for _, repeat := range d.opts.repeats {
			cells = append(cells, cell{arm: round2.Arm(armNum), repeat: repeat})
		}
	}
	// Phase A: first turns, all cells, before any correction exists.
	for _, c := range cells {
		if !d.wantsTurn(round2.TurnFirst) {
			continue
		}
		if _, _, err := d.driveTurn(ctx, shopper, c.arm, c.repeat, round2.TurnFirst); err != nil {
			return err
		}
	}
	// Phase B: the shopper's correction, recorded once through the layer.
	if shopper.Correction != nil && d.hasMementoArm() {
		if err := d.recordCorrection(ctx, shopper); err != nil {
			return err
		}
	}
	// Phase C: the later turns.
	for _, c := range []round2.TurnKind{round2.TurnCorrection, round2.TurnNextSession, round2.TurnOtherAgent} {
		if !d.wantsTurn(c) || shopper.Correction == nil {
			continue
		}
		for _, cell := range cells {
			if _, _, err := d.driveTurn(ctx, shopper, cell.arm, cell.repeat, c); err != nil {
				return err
			}
		}
	}
	return nil
}

// wantsTurn reports whether the flag plans one turn kind.
func (d *driver) wantsTurn(turn round2.TurnKind) bool {
	for _, want := range d.opts.turns {
		if want == string(turn) {
			return true
		}
	}
	return false
}

// hasMementoArm reports whether any selected arm carries the live layer.
func (d *driver) hasMementoArm() bool {
	for _, armNum := range d.opts.arms {
		if armNum == int(round2.ArmMementoShopperOnly) || armNum == int(round2.ArmMementoFull) {
			return true
		}
	}
	return false
}

// recordCorrection writes the shopper's correction through the layer once,
// after every first turn and before any later turn. It names the strongest
// own-evidence belief when one is served, and records a standing correction
// when none is.
func (d *driver) recordCorrection(ctx context.Context, shopper seed.Round2Shopper) error {
	raw, err := d.layer.recall(ctx, shopper)
	if err != nil {
		return fmt.Errorf("t104: pre-correction recall for %s: %w", shopper.ShopperID, err)
	}
	var response struct {
		Beliefs []struct {
			BeliefID string `json:"belief_id"`
			Evidence []any  `json:"evidence"`
		} `json:"beliefs"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return fmt.Errorf("t104: decode pre-correction recall %s: %w", shopper.ShopperID, err)
	}
	against := ""
	for _, belief := range response.Beliefs {
		if len(belief.Evidence) > 0 {
			against = belief.BeliefID
			break
		}
	}
	req := map[string]string{
		"shopper_id": shopper.ShopperID,
		"text":       shopper.Correction.Text,
	}
	if against != "" {
		req["contradicts_belief_id"] = against
	}
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("t104: encode correction: %w", err)
	}
	respBody, err := d.layer.tool(ctx, "record_correction", body)
	if err != nil {
		return fmt.Errorf("t104: record correction for %s: %w", shopper.ShopperID, err)
	}
	fmt.Printf("correction %s: recorded against %q\n", shopper.ShopperID, against)
	corrections = append(corrections, correctionRecord{
		ShopperID: shopper.ShopperID,
		Against:   against,
		Request:   json.RawMessage(body),
		Response:  json.RawMessage(respBody),
	})
	return nil
}

// correctionRecord is one committed correction call.
type correctionRecord struct {
	ShopperID string          `json:"shopper_id"`
	Against   string          `json:"contradicts_belief_id,omitempty"`
	Request   json.RawMessage `json:"request"`
	Response  json.RawMessage `json:"response"`
}

// corrections collects the correction calls the run commits.
var corrections []correctionRecord

// driveTurn builds one turn's prompt, sends it, records the run file and
// returns the record with its trace cost. A turn stopped by the caps before
// any model call is committed as capped. A turn that may have been paid for
// and then failed is committed with the failure noted, never silently
// retried.
func (d *driver) driveTurn(ctx context.Context, shopper seed.Round2Shopper, arm round2.Arm, repeat int, turn round2.TurnKind) (round2.RunFile, float64, error) {
	runID := runIDFor(shopper, arm, repeat, turn)
	path := filepath.Join(d.opts.outDir, runID+".json")
	if !d.opts.force {
		if record, ok, err := readRunFile(path); err != nil {
			return round2.RunFile{}, 0, err
		} else if ok {
			// A turn committed without a reply is a recorded failure that
			// may have been paid for, not a finished turn: reuse would
			// silently drop it from the batch, so refuse until its trace
			// is recovered or the file is removed.
			if !record.Capped && strings.TrimSpace(record.Reply) == "" {
				return round2.RunFile{}, 0, fmt.Errorf("t104: %s is committed with no reply (a failed turn); recover its trace with -mode recover or delete the file before re-running", runID)
			}
			fmt.Printf("turn %s: already committed, reused\n", runID)
			return record, 0, nil
		}
	}
	phase := round2.TurnPhase(turn)
	fetch, err := d.memoryFetch(ctx, shopper, arm, phase)
	if err != nil {
		return round2.RunFile{}, 0, err
	}
	memory, err := round2.MemorySection(arm, fetch)
	if err != nil {
		return round2.RunFile{}, 0, err
	}
	prompt := round2.BaseSection(shopper) + "\n" + memory
	agent := "A"
	switch turn {
	case round2.TurnCorrection:
		firstReply, err := d.firstReply(shopper, arm, repeat)
		if err != nil {
			return round2.RunFile{}, 0, err
		}
		prompt += "\n\nYour recommendation in reply to the above was:\n" + firstReply +
			"\n\nShopper correction: " + shopper.Correction.Text + "\n" + reAsk
	case round2.TurnNextSession:
		// A new session carries no conversation: the prompt is the arm's
		// memory alone, in the turn's phase.
	case round2.TurnOtherAgent:
		// Another agent asks with no shared conversation; the label says
		// agent B even though the neutral instrument is one deployment.
		agent = "B"
	}
	record, err := round2.NewRunFile(runID, shopper, arm, repeat, turn, agent, prompt, fetch, "", 0)
	if err != nil {
		return round2.RunFile{}, 0, err
	}
	caps := harness.DefaultCaps()
	if !caps.Allows(record.PromptWords) {
		record.Capped = true
		record.Extraction.Notes = "capped before any model call: the prompt alone exceeds the frozen caps"
		if err := writeJSON(path, record); err != nil {
			return round2.RunFile{}, 0, err
		}
		fmt.Printf("turn %s: capped before any model call, committed as marked\n", runID)
		return record, 0, nil
	}
	if err := d.refuseOverBudget(runID); err != nil {
		return round2.RunFile{}, 0, err
	}
	billingBefore, err := refreshLogin(ctx)
	if err != nil {
		return round2.RunFile{}, 0, err
	}
	fmt.Printf("turn %s: prompt %d words; billing usage before $%.4f\n", runID, record.PromptWords, billingBefore.usage)
	conversationID, err := d.client.createConversation(ctx)
	if err != nil {
		return round2.RunFile{}, 0, err
	}
	sentAt := time.Now().UTC()
	if _, err := d.client.send(ctx, conversationID, prompt); err != nil {
		return round2.RunFile{}, 0, err
	}
	trace, err := waitForTrace(ctx, d.opts.agentName, conversationID, sentAt)
	if err != nil {
		record.Extraction.Notes = "trace missing after the message was sent: the turn may have been paid for; it is recorded, not retried"
		if writeErr := writeJSON(path, d.scrubbed(record, conversationID, liveTrace{})); writeErr != nil {
			return round2.RunFile{}, 0, writeErr
		}
		return round2.RunFile{}, 0, fmt.Errorf("t104: turn %s: %w", runID, err)
	}
	billingAfter, err := refreshLogin(ctx)
	if err != nil {
		return round2.RunFile{}, 0, err
	}
	final, err := d.scrubbedWithReply(record, shopper, conversationID, trace)
	if err != nil {
		return round2.RunFile{}, 0, err
	}
	if trace.cost > singleTurnGuardUSD {
		final.Capped = true
		final.Extraction.Notes = fmt.Sprintf("single-turn guard: trace cost $%.6f exceeds $%.2f; the batch aborts", trace.cost, singleTurnGuardUSD)
		if err := writeJSON(path, final); err != nil {
			return round2.RunFile{}, 0, err
		}
		return round2.RunFile{}, trace.cost, fmt.Errorf("t104: turn %s cost $%.6f over the $%.2f single-turn guard: committed as marked, aborting",
			runID, trace.cost, singleTurnGuardUSD)
	}
	if err := writeJSON(path, final); err != nil {
		return round2.RunFile{}, 0, err
	}
	d.spent += trace.cost
	fmt.Printf("turn %s: reply %d words, cost $%.6f, cumulative $%.6f (billing delta %.4f)\n",
		runID, final.ReplyWords, trace.cost, d.spent, billingAfter.usage-billingBefore.usage)
	if d.spent > cumulativeBudgetUSD {
		return round2.RunFile{}, trace.cost, fmt.Errorf("t104: cumulative $%.6f exceeds the $%.2f budget after %s: stopping",
			d.spent, cumulativeBudgetUSD, runID)
	}
	return final, trace.cost, nil
}

// refuseOverBudget refuses to start a turn that could exceed the cap even at
// the single-turn guard's worst case.
func (d *driver) refuseOverBudget(runID string) error {
	if d.spent+singleTurnGuardUSD > cumulativeBudgetUSD {
		return fmt.Errorf("t104: refusing to start %s: $%.6f spent plus the $%.2f single-turn guard would exceed the $%.2f cap",
			runID, d.spent, singleTurnGuardUSD, cumulativeBudgetUSD)
	}
	return nil
}

// memoryFetch obtains one arm's memory: the frozen none/extraction for arms 1
// and 2, and the layer's live recall_for_purchase for arms 3 and 4.
func (d *driver) memoryFetch(ctx context.Context, shopper seed.Round2Shopper, arm round2.Arm, phase round2.RecallPhase) (round2.MemoryFetch, error) {
	switch arm {
	case round2.ArmNoMemory, round2.ArmPayPalHistory:
		return round2.FetchMemory(shopper, arm, d.population, phase)
	case round2.ArmMementoShopperOnly, round2.ArmMementoFull:
		raw, err := d.layer.recall(ctx, shopper)
		if err != nil {
			return round2.MemoryFetch{}, err
		}
		return round2.MemoryFetch{Tool: "recall_for_purchase", Response: raw}, nil
	default:
		return round2.MemoryFetch{}, fmt.Errorf("t104: unknown arm %d", int(arm))
	}
}

// firstReply reads the committed first turn's reply for one cell.
func (d *driver) firstReply(shopper seed.Round2Shopper, arm round2.Arm, repeat int) (string, error) {
	path := filepath.Join(d.opts.outDir, runIDFor(shopper, arm, repeat, round2.TurnFirst)+".json")
	record, ok, err := readRunFile(path)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("t104: %s has no committed first turn to correct", shopper.ShopperID)
	}
	return record.Reply, nil
}

// scrubbed renders a record's prompt and reply through the placeholder
// scrubber before it is written.
func (d *driver) scrubbed(record round2.RunFile, conversationID string, trace liveTrace) round2.RunFile {
	scrub := scrubber(d.opts.deploymentID, conversationID, trace)
	record.Prompt = scrub(record.Prompt)
	record.MemoryResponse = scrub(record.MemoryResponse)
	record.Reply = scrub(record.Reply)
	return record
}

// scrubbedWithReply fills the reply, extraction and cost from the trace and
// scrubs the record.
func (d *driver) scrubbedWithReply(record round2.RunFile, shopper seed.Round2Shopper, conversationID string, trace liveTrace) (round2.RunFile, error) {
	final, err := round2.NewRunFile(record.RunID, shopper, round2.Arm(record.Arm), record.Repeat,
		record.Turn, record.Agent, record.Prompt, round2.MemoryFetch{Tool: record.MemoryTool, Response: []byte(record.MemoryResponse)},
		trace.reply, trace.cost)
	if err != nil {
		return round2.RunFile{}, err
	}
	return d.scrubbed(final, conversationID, trace), nil
}

// runIDFor names one turn.
func runIDFor(shopper seed.Round2Shopper, arm round2.Arm, repeat int, turn round2.TurnKind) string {
	return fmt.Sprintf("r2-a%d-%s-r%d-%s", int(arm), strings.TrimPrefix(shopper.ShopperID, "r2-"), repeat, turn)
}

// readRunFile reads one committed run file, reporting whether it exists.
func readRunFile(path string) (round2.RunFile, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return round2.RunFile{}, false, nil
		}
		return round2.RunFile{}, false, fmt.Errorf("t104: read %s: %w", path, err)
	}
	var record round2.RunFile
	if err := json.Unmarshal(raw, &record); err != nil {
		return round2.RunFile{}, false, fmt.Errorf("t104: decode %s: %w", path, err)
	}
	return record, true, nil
}

// writeJSON writes one indented JSON file with a trailing newline.
func writeJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("t104: encode %s: %w", path, err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("t104: write %s: %w", path, err)
	}
	return nil
}

// committedSpend sums the trace costs of the committed run files.
func committedSpend(outDir string) (float64, error) {
	paths, err := filepath.Glob(filepath.Join(outDir, "r2-a*.json"))
	if err != nil {
		return 0, err
	}
	total := 0.0
	for _, path := range paths {
		record, ok, err := readRunFile(path)
		if err != nil {
			return 0, err
		}
		if !ok {
			continue
		}
		total += record.TraceCostUSD
	}
	return total, nil
}

// finish grades every committed turn, writes the scores and the recall
// comparison, deletes the scratch index and prints the spend by arm.
func (d *driver) finish(ctx context.Context) error {
	if err := d.writeCorrections(); err != nil {
		return err
	}
	scores, err := d.gradeAll()
	if err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(d.opts.outDir, "r2-scores.json"), map[string]any{"runs": scores}); err != nil {
		return err
	}
	report, err := d.compareRecall()
	if err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(d.opts.outDir, "r2-recall-compare.json"), report); err != nil {
		return err
	}
	summary, err := d.summary(scores, report)
	if err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(d.opts.outDir, "r2-summary.json"), summary); err != nil {
		return err
	}
	if d.layer != nil {
		closeIndexUnlessKept(ctx, d.layer, d.opts)
	}
	return d.printSummary(summary)
}

// closeIndexUnlessKept removes the scratch index unless a debug run keeps it.
func closeIndexUnlessKept(ctx context.Context, live *layer, opts options) {
	if opts.keepIndex {
		fmt.Println("index: kept for debugging (delete it before the batch)")
		return
	}
	live.closeIndex(ctx)
}

// writeCorrections commits the correction calls when there were any.
func (d *driver) writeCorrections() error {
	if len(corrections) == 0 {
		return nil
	}
	return writeJSON(filepath.Join(d.opts.outDir, "r2-corrections.json"), map[string]any{
		"note":        "The shopper's correction, recorded through the layer's record_correction after every first turn and before any later turn. This file holds the final driver invocation's calls: the layer is reseeded per invocation, so a correction turn committed earlier keeps that instance's recorded_at in its own memory_response.",
		"corrections": corrections,
	})
}

// runFilePaths lists the committed run files in name order.
func (d *driver) runFilePaths() ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(d.opts.outDir, "r2-a*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

// gradeAll grades every committed run file with the frozen grader and writes
// the score back into the record, so every run file carries its own grade as
// well as the collected scores file.
func (d *driver) gradeAll() ([]round2.Score, error) {
	answers, err := round2.LoadAnswers()
	if err != nil {
		return nil, err
	}
	paths, err := d.runFilePaths()
	if err != nil {
		return nil, err
	}
	out := make([]round2.Score, 0, len(paths))
	for _, path := range paths {
		record, ok, err := readRunFile(path)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		shopper, ok := d.shoppers[record.ShopperID]
		if !ok {
			return nil, fmt.Errorf("t104: run %s names an unselected shopper %q", record.RunID, record.ShopperID)
		}
		answer, ok := answers[record.ShopperID]
		if !ok {
			return nil, fmt.Errorf("t104: no hidden answer for %s", record.ShopperID)
		}
		score, err := round2.GradeRunFile(record, shopper, answer)
		if err != nil {
			return nil, fmt.Errorf("t104: grade %s: %w", record.RunID, err)
		}
		record.Score = &score
		if err := writeJSON(path, record); err != nil {
			return nil, err
		}
		out = append(out, score)
	}
	return out, nil
}

// summary carries the committed spend and counts.
type runSummary struct {
	// FreezeCommit is the commit the marker names.
	FreezeCommit string `json:"freeze_commit"`
	// Deployment and Model name the instrument the turns ran on.
	Deployment string `json:"deployment"`
	Model      string `json:"model"`
	// Turns, CostUSD, CostByArm and CostByTurn record the spend.
	Turns      int                `json:"turns"`
	CostUSD    float64            `json:"cost_usd"`
	CostByArm  map[string]float64 `json:"cost_by_arm"`
	CostByTurn map[string]float64 `json:"cost_by_turn"`
	// CapUSD is the frozen cap.
	CapUSD float64 `json:"cap_usd"`
	// Contaminated and Capped count the marked turns.
	Contaminated int `json:"contaminated"`
	Capped       int `json:"capped"`
	// RecallTurnsCompared and RecallTurnsDiffering count the Memento turns
	// against the frozen expectation; RecallDifferingRuns names the turns
	// that differ.
	RecallTurnsCompared  int      `json:"recall_turns_compared"`
	RecallTurnsDiffering int      `json:"recall_turns_differing"`
	RecallDifferingRuns  []string `json:"recall_differing_runs,omitempty"`
}

// summary derives the committed summary from the run files and comparison.
func (d *driver) summary(scores []round2.Score, report *recallReport) (runSummary, error) {
	marker, err := round2.ReadRunsMarker(d.opts.outDir)
	if err != nil {
		return runSummary{}, err
	}
	out := runSummary{
		FreezeCommit: marker.FreezeCommit,
		Deployment:   "pilot-baseline",
		Model:        round2.Model,
		CostByArm:    map[string]float64{},
		CostByTurn:   map[string]float64{},
		CapUSD:       cumulativeBudgetUSD,
		Turns:        len(scores),
	}
	paths, err := d.runFilePaths()
	if err != nil {
		return runSummary{}, err
	}
	for _, path := range paths {
		record, ok, err := readRunFile(path)
		if err != nil {
			return runSummary{}, err
		}
		if !ok {
			continue
		}
		out.CostByArm[fmt.Sprintf("arm %d", record.Arm)] += record.TraceCostUSD
		out.CostByTurn[string(record.Turn)] += record.TraceCostUSD
		out.CostUSD += record.TraceCostUSD
		if record.Contaminated {
			out.Contaminated++
		}
		if record.Capped {
			out.Capped++
		}
	}
	if report != nil {
		out.RecallTurnsCompared = len(report.Runs)
		for _, run := range report.Runs {
			if len(run.Differences) > 0 {
				out.RecallTurnsDiffering++
				out.RecallDifferingRuns = append(out.RecallDifferingRuns, run.RunID)
			}
		}
	}
	return out, nil
}

// printSummary prints the committed figures.
func (d *driver) printSummary(summary runSummary) error {
	fmt.Printf("done: %d turns, $%.6f of $%.2f\n", summary.Turns, summary.CostUSD, summary.CapUSD)
	for _, arm := range []string{"arm 1", "arm 2", "arm 3", "arm 4"} {
		fmt.Printf("  %s: $%.6f\n", arm, summary.CostByArm[arm])
	}
	if summary.RecallTurnsCompared > 0 {
		fmt.Printf("recall compared on %d turns; %d differ from the frozen expectation\n",
			summary.RecallTurnsCompared, summary.RecallTurnsDiffering)
	}
	return nil
}

// caps mirrors the frozen caps: the record builder applies the same rule.
