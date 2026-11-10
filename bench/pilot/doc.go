// Package pilot runs the T5.3 baseline pilot: the first live measurement
// in the comparison, arms 1 and 2 only, never arm 3.
//
// The pilot drives one fresh conversation per run through the platform
// messaging proxy. The first two turns went through the deployed reference
// agent and proved it cannot implement the arms: its shopper default made
// it recall the demo shopper's memory on comparison prompts (kept as
// discarded evidence). The ten graded runs drive the neutral instrument
// pilot-baseline instead (same gateway Sonnet model, no tools, no memory
// beyond the user message); see README.md and agent/. The user message of each run is the harness prompt for that shopper
// and arm, verbatim: arms differ only in the memory block they carry, the
// same property the harness proves offline. The agent's own recall-first
// behaviour is constant across arms; for comparison shoppers it returns no
// memory, which is the honest no-layer state before any arm 3 run.
//
// The pilot records, per run: the prompt, the arm's memory fetch, the agent
// tool spans the trace shows, the reply, word-count tokens, the trace cost,
// and the per-measure scores from the frozen grader. Live BPE token counts
// are not exposed by the trace API, so tokens are harness word counts on a
// recorded basis and spend is platform cost; the two are never mixed.
//
// Blindness: the driver builds prompts through the harness only and never
// loads the hidden answers. Answers join the runs at grading time, in
// cmd/grade, after the replies are recorded. The grader package owns that
// separation; this package only carries the graded values through.
package pilot
