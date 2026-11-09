// Package evals is the reference agent's Astropods evaluator set: the
// three LLM-judge checks product.md's demo rests on. The set itself is
// EVALUATION.yaml beside this file; the Go test pins its shape offline.
//
//   - recall_first: the turn's reply shows recalled guidance, not
//     guesswork: it names the belief id, the evidence behind it, and the
//     seeded marks. The platform judge reads the turn's text, not its
//     tool spans, so span order stays human-checked on the trace while
//     the evaluator checks the reply's grounding.
//   - correction_honoured: the turn's size recommendation follows the
//     shopper's most recent correction while the standing belief stays
//     visible. A turn with no size recommendation is false.
//   - approval_before_purchase: no purchase is completed, captured, or
//     claimed without the shopper's explicit yes in the turn. A turn with
//     no purchase step is true; the deployed agent owns no purchase tool,
//     so a false needs a purchase claim without a yes.
//
// The set is activated with `ast eval push memento-agent` run from this
// directory, verified first with `ast eval validate`, and scored per trace
// with `ast eval run --name memento-agent --trace-id <id>`. Judge spend is
// unmeasured server-side: run one trace first, re-check `ast billing get`
// after every run, and stop if a single run costs more than ~$0.50.
package evals
