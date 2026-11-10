// Package grader scores one comparison run on the five measures from
// the product spec, as counts, never percentages: right size first time
// against the hidden answer; wrong-cause errors; corrections honoured on
// the very next recommendation; citations that really support the
// recommendation; tokens per recommendation.
//
// The grader runs fully offline on hand-built runs: no key, no network,
// no spend. It consumes harness records (prompts, replies, token counts)
// plus the arm-facing history and the hidden answer. Any model-judgement
// call happens in later tasks with their own budgets, not here: the
// score's Opinions field stays empty offline, and every measure below
// states its mechanical-vs-opinion line.
//
// Mechanical vs opinion, per measure:
//
//  1. Right size: mechanical. Normalized string equality between the
//     recommended size and the hidden RightSize. No judgement.
//  2. Wrong cause: mechanical. A cited row whose evidence counts toward
//     seller reliability or toward nothing, cited in support of a size
//     that moves off the shopper's baseline kept size. Citing the same
//     row while holding the size is a legitimate exclusion, not an error.
//  3. Correction honoured: mechanical. Normalized string equality between
//     the correction's wanted size and the size of the very next
//     recommendation. Turn order is the caller's contract: Second must be
//     the immediate next recommendation turn.
//  4. Citations: mechanical. A citation supports a size recommendation
//     when it resolves to exactly one history row and that row counts
//     toward fit: a classified fit reason, or explicit fit feedback tied
//     to a purchase (which the layer counts toward fit by verdict, not by
//     reason category). Whether the reply's prose reads well is a model's
//     opinion, never a measurement, and is not scored here.
//  5. Tokens: mechanical. harness.CountTokens over prompt plus reply per
//     recommendation turn, with the basis recorded on every score so no
//     later stage can silently mix tokenizer bases.
package grader
