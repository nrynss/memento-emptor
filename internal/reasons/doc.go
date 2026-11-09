// Package reasons turns the shopper's answer into a reason category with
// a confidence. One exclusive-choice question per answer goes to the
// classifier service; anything under the floor stays a pending question
// and the agent asks again. A refund the shopper never explains keeps a
// nil reason and counts toward nothing.
//
// The floor is 0.98, re-measured here on the eleven-phrase labelled set:
// every clear phrase classifies at 1.00 and every vague one falls below
// it, with no drift from the recorded measurements. The sharp
// probabilities make the unclear category do the separation work, so the
// client sends the answer alone: no retrieved neighbour context travels
// with the question. That omission is enforced by a test, not an
// opinion; re-measure both claims on real shopper answers before either
// gates a belief.
package reasons
