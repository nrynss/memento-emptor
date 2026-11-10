// Package precedence resolves the pivot's five-step order as a pure
// function: the shopper's correction, the shopper's own fit evidence, the
// pooled product fit adjusted from the usual size, the shopper's general
// statement, and nothing. It names the step that decided and marks every
// weaker present step overridden, never hidden: when steps disagree, recall
// returns both sides and the agent sees which rung won.
package precedence
