// Package loops is the reference agent's asynchronous loop: the four
// shopper round-trips that product.md's demo steps 3 to 5 rest on.
//
//   - Why did it go back? The agent polls pending_questions and asks one
//     short question for the first pending refund; the answer goes to
//     record_reason tied to that refund's transaction.
//   - Approval before payment. The agent proposes, asks, and only an
//     explicit yes reaches orders.Service.Approve, mirrored into the
//     transport so both sides agree before capture. Capture without that
//     record fails closed in the orders package; this package is what
//     earns the approval first.
//   - Correction at decision time. The shopper's words go to
//     record_correction at once, and the very next recommendation follows
//     the correction while the belief stays.
//   - Fit feedback after delivery, tied to the purchase's custom id.
//
// The policy here is transport-free: it builds the next step and guards
// the approval write. The deployed agent carries the same steps as tool
// calls; the replay test pins the shapes to testdata/replay/p3-turns.json.
package loops
