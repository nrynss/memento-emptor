// Package harness runs a set of seeded shoppers through one comparison
// arm and records everything the run did.
//
// The three arms differ only in what the agent is given: no memory, the
// shopper's raw PayPal transactions and past messages handed over
// directly, or the Memento layer's beliefs, evidence and corrections.
// The harness drives arms against a fixture double (a scripted model
// standing in for the live model plus stub tool responses built from the
// seed histories); it never calls a live model, spends nothing, and never
// modifies agent/runtime.
//
// Every run enforces its caps in code: a run whose prompt already exceeds
// the per-run token or spend cap stops before any model call, so it never
// spends through. A run whose reply pushes it over the cap is marked
// capped so later stages (grader, pilot) can see the overrun.
//
// No code here reads the answer key. Arm
// inputs are assembled from the arm-facing histories only. The test in
// this package proves that statically and by checking prompts against
// the concealed answer text.
package harness
