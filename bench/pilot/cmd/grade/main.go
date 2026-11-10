// Command grade joins the pilot run files with the hidden answers and
// scores each run's first recommendation with the frozen grader. It runs
// after the pilot author has reviewed every reply and set the human
// extraction fields where the deterministic step needs judgement.
//
// The blindness check runs first: no run prompt may carry a hidden Why
// string. (RightSize values are single letters that occur naturally in
// prompts, so the check covers the rationale strings, mirroring the
// harness's own hidden-answer test.)
//
// Usage:
//
//	go run ./bench/pilot/cmd/grade
//
// Run from bench/pilot so the default paths resolve. It rewrites each run
// file with its score and writes scores.json beside them.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nrynss/memento-emptor/bench/grader"
	"github.com/nrynss/memento-emptor/bench/harness"
	"github.com/nrynss/memento-emptor/bench/pilot"
	"github.com/nrynss/memento-emptor/seed"
)

// ErrBlindness is returned when a run prompt carries hidden answer material.
var ErrBlindness = errors.New("grade: run prompt carries hidden answer material")

func main() {
	runsDir := flag.String("runs", "runs", "pilot run files directory")
	historiesDir := flag.String("histories", "../../testdata/seed/histories", "arm-facing histories directory")
	answersPath := flag.String("answers", "../../testdata/seed/hidden/answers.json", "hidden answers file")
	flag.Parse()
	if err := run(*runsDir, *historiesDir, *answersPath); err != nil {
		fmt.Fprintf(os.Stderr, "grade: %v\n", err)
		os.Exit(1)
	}
}

// ScoredRun is one graded pilot run in scores.json.
type ScoredRun struct {
	RunID              string  `json:"run_id"`
	ShopperID          string  `json:"shopper_id"`
	Arm                int     `json:"arm"`
	RightSize          string  `json:"answer_right_size"`
	Recommended        string  `json:"recommended_size"`
	RightSizeFirst     bool    `json:"right_size_first"`
	WrongCause         int     `json:"wrong_cause"`
	CorrectionHonoured *bool   `json:"correction_honoured"`
	Supported          int     `json:"supported_cites"`
	Cited              int     `json:"cited"`
	TokensFirst        int     `json:"tokens_first"`
	OverCap            bool    `json:"over_cap"`
	Contaminated       bool    `json:"contaminated"`
	TraceCostUSD       float64 `json:"trace_cost_usd"`
}

// Scores is the committed grading summary beside the run files.
type Scores struct {
	Runs []ScoredRun `json:"runs"`
}

func run(runsDir, historiesDir, answersPath string) error {
	histories, err := harness.LoadHistories(historiesDir)
	if err != nil {
		return err
	}
	byShopper := make(map[string]seed.History, len(histories))
	for _, h := range histories {
		byShopper[h.ShopperID] = h
	}
	answers, err := grader.LoadAnswers(answersPath)
	if err != nil {
		return err
	}
	paths, err := filepath.Glob(filepath.Join(runsDir, "*.json"))
	if err != nil {
		return fmt.Errorf("grade: list runs: %w", err)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return errors.New("grade: no run files to score")
	}
	scores := Scores{Runs: []ScoredRun{}}
	for _, path := range paths {
		if strings.HasSuffix(path, "scores.json") {
			continue
		}
		scored, skipped, err := gradeOne(path, byShopper, answers)
		if err != nil {
			return err
		}
		if skipped {
			fmt.Printf("skip contaminated %s (kept as evidence, ungraded)\n", path)
			continue
		}
		scores.Runs = append(scores.Runs, scored)
	}
	raw, err := json.MarshalIndent(scores, "", "  ")
	if err != nil {
		return fmt.Errorf("grade: encode scores: %w", err)
	}
	out := filepath.Join(runsDir, "scores.json")
	if err := os.WriteFile(out, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("grade: write %s: %w", out, err)
	}
	fmt.Printf("graded %d runs -> %s\n", len(scores.Runs), out)
	return nil
}

func gradeOne(path string, byShopper map[string]seed.History, answers map[string]seed.Answer) (ScoredRun, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ScoredRun{}, false, fmt.Errorf("grade: read %s: %w", path, err)
	}
	var record pilot.RunFile
	if err := json.Unmarshal(raw, &record); err != nil {
		return ScoredRun{}, false, fmt.Errorf("grade: decode %s: %w", path, err)
	}
	if record.Contaminated {
		// Contaminated runs stay on disk as evidence of the wrong
		// memory being read. They are never graded: they measured
		// another shopper's memory, not the arm.
		return ScoredRun{}, true, nil
	}
	history, ok := byShopper[record.ShopperID]
	if !ok {
		return ScoredRun{}, false, fmt.Errorf("grade: %s: unknown shopper %q", path, record.ShopperID)
	}
	answer, ok := answers[record.ShopperID]
	if !ok {
		return ScoredRun{}, false, fmt.Errorf("grade: %s: no hidden answer for %q", path, record.ShopperID)
	}
	for _, other := range answers {
		if other.Why != "" && strings.Contains(record.Prompt, other.Why) {
			return ScoredRun{}, false, fmt.Errorf("%w: %s carries the rationale for %s", ErrBlindness, path, other.ShopperID)
		}
	}
	size := pilot.EffectiveSize(record.Extraction.Size, record.Extraction.HumanSize)
	cites := record.Extraction.EffectiveCites()
	score, err := pilot.GradeFirst(history, answer, record.Prompt, record.Reply, size, cites)
	if err != nil {
		return ScoredRun{}, false, fmt.Errorf("grade: %s: %w", path, err)
	}
	record.Score = &score
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return ScoredRun{}, false, fmt.Errorf("grade: encode %s: %w", path, err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		return ScoredRun{}, false, fmt.Errorf("grade: write %s: %w", path, err)
	}
	return ScoredRun{
		RunID:              record.RunID,
		ShopperID:          record.ShopperID,
		Arm:                record.Arm,
		RightSize:          answer.RightSize,
		Recommended:        size,
		RightSizeFirst:     score.RightSize,
		WrongCause:         score.WrongCause,
		CorrectionHonoured: score.CorrectionHonoured,
		Supported:          score.Supported,
		Cited:              score.Cited,
		TokensFirst:        score.TokensFirst,
		OverCap:            score.OverCap,
		Contaminated:       record.Contaminated,
		TraceCostUSD:       record.TraceCostUSD,
	}, false, nil
}
