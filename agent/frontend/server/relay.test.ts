/**
 * Guards for the shopper page's two relay rules (T9.12):
 *  - the request shape validateMessages accepts, and
 *  - the anonymous turn budget: 30 model turns per container lifetime,
 *    then an honest refusal, with validation failures costing nothing.
 *
 * Runs on node alone (`node --test server/relay.test.ts`), so the gate can
 * run it without bun, a gateway credential or a model call; relay.ts has no
 * imports for the same reason.
 */

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  createTurnBudget,
  decideChatTurn,
  MAX_MESSAGE_CHARS,
  MAX_REQUEST_MESSAGES,
  MAX_TURNS,
  TURNS_EXHAUSTED_ERROR,
  validateMessages,
} from "./relay.ts";

function userTurn(text: string) {
  return { role: "user", content: text };
}

function assistantTurn(text: string) {
  return { role: "assistant", content: text };
}

function refusal(body: unknown): string {
  const result = validateMessages(body);
  assert.equal(result.ok, false);
  return result.ok ? "" : result.error;
}

describe("validateMessages", () => {
  it("refuses a body that is not {messages: [...]}", () => {
    assert.match(refusal(null), /messages/);
    assert.match(refusal("hi"), /messages/);
    assert.match(refusal({}), /messages/);
    assert.match(refusal({ messages: null }), /non-empty array/);
    assert.match(refusal({ messages: "hi" }), /non-empty array/);
  });

  it("refuses an empty request and one over the message cap", () => {
    assert.match(refusal({ messages: [] }), /at most 20 turns/);

    const atCap = Array.from({ length: MAX_REQUEST_MESSAGES }, (_, i) =>
      i % 2 === 0 ? userTurn(`turn ${i}`) : assistantTurn(`reply ${i}`),
    );
    // The last message of an even-length list is an assistant turn, so the
    // acceptance of exactly MAX_REQUEST_MESSAGES is proved separately below.
    assert.equal(atCap.length, 20);
    assert.match(refusal({ messages: [...atCap, userTurn("one more")] }), /at most 20 turns/);
  });

  it("refuses a message that is not {role, content}", () => {
    assert.match(refusal({ messages: [null] }), /each message/);
    assert.match(refusal({ messages: [42] }), /each message/);
    assert.match(refusal({ messages: [{}] }), /role must be user\|assistant/);
  });

  it("refuses an unknown role or a non-string content", () => {
    assert.match(refusal({ messages: [{ role: "system", content: "x" }] }), /role must be user\|assistant/);
    assert.match(refusal({ messages: [{ role: "user", content: 7 }] }), /role must be user\|assistant/);
    assert.match(
      refusal({ messages: [{ role: "assistant", content: [{ type: "text", text: "x" }] }] }),
      /content a string/,
    );
  });

  it("refuses empty, whitespace-only and over-long text", () => {
    assert.match(refusal({ messages: [userTurn("")] }), /1\.\.1000 characters/);
    assert.match(refusal({ messages: [userTurn("   \n\t ")] }), /1\.\.1000 characters/);
    assert.match(refusal({ messages: [userTurn("x".repeat(MAX_MESSAGE_CHARS + 1))] }), /1\.\.1000 characters/);

    const atLimit = validateMessages({ messages: [userTurn("x".repeat(MAX_MESSAGE_CHARS))] });
    assert.equal(atLimit.ok, true);
  });

  it("refuses a conversation that does not end with the shopper", () => {
    assert.match(
      refusal({ messages: [assistantTurn("welcome")] }),
      /last message must be from the user/,
    );
    assert.match(
      refusal({ messages: [userTurn("I need a shirt"), assistantTurn("Which size?")] }),
      /last message must be from the user/,
    );
  });

  it("accepts one short user turn and trims it", () => {
    const result = validateMessages({ messages: [userTurn("  I need a Brand X oxford shirt \n")] });
    assert.deepEqual(result, { ok: true, messages: [userTurn("I need a Brand X oxford shirt")] });
  });

  it("accepts the full 20-message history when it ends with the shopper", () => {
    const messages = Array.from({ length: MAX_REQUEST_MESSAGES }, (_, i) =>
      i % 2 === 0 ? userTurn(`turn ${i}`) : assistantTurn(`reply ${i}`),
    );
    messages.push(userTurn("  yes  "));
    // 21 messages is one over; drop the oldest to land exactly on the cap.
    const atCap = messages.slice(1);
    const result = validateMessages({ messages: atCap });
    assert.equal(result.ok, true);
    if (!result.ok) return;
    assert.equal(result.messages.length, MAX_REQUEST_MESSAGES);
    assert.deepEqual(result.messages.at(-1), userTurn("yes"));
    assert.deepEqual(result.messages.at(-2), {
      role: "assistant",
      content: [{ type: "text", text: "reply 19" }],
    });
  });

  it("wraps assistant turns as text parts and keeps user turns as text", () => {
    const result = validateMessages({
      messages: [userTurn("ask"), assistantTurn("reply"), userTurn("ask again")],
    });
    assert.deepEqual(result, {
      ok: true,
      messages: [
        { role: "user", content: "ask" },
        { role: "assistant", content: [{ type: "text", text: "reply" }] },
        { role: "user", content: "ask again" },
      ],
    });
  });
});

describe("the 30-turn cap", () => {
  it("allows exactly MAX_TURNS turns and refuses the next one", () => {
    assert.equal(MAX_TURNS, 30);
    const budget = createTurnBudget();
    for (let i = 1; i <= MAX_TURNS; i += 1) {
      assert.equal(budget.take(), true, `turn ${i} must be allowed`);
      assert.equal(budget.used(), i);
    }
    assert.equal(budget.remaining(), 0);
    assert.equal(budget.take(), false, "turn 31 must be refused");
    assert.equal(budget.used(), MAX_TURNS, "a refused turn consumes nothing");
  });

  it("counts one turn per accepted chat request", () => {
    const budget = createTurnBudget();
    const decision = decideChatTurn({ messages: [userTurn("I need a shirt")] }, budget);
    assert.equal(decision.kind, "turn");
    assert.equal(budget.used(), 1);
    assert.equal(budget.remaining(), MAX_TURNS - 1);
  });

  it("spends the whole budget, then refuses; later turns stay refused", () => {
    const budget = createTurnBudget();
    for (let i = 0; i < MAX_TURNS; i += 1) {
      const decision = decideChatTurn({ messages: [userTurn(`turn ${i}`)] }, budget);
      assert.equal(decision.kind, "turn");
    }

    const overCap = decideChatTurn({ messages: [userTurn("one more")] }, budget);
    assert.deepEqual(overCap, { kind: "exhausted", error: TURNS_EXHAUSTED_ERROR });
    assert.match(TURNS_EXHAUSTED_ERROR, /turn budget exhausted/);
    assert.equal(budget.used(), MAX_TURNS);

    const again = decideChatTurn({ messages: [userTurn("still there?")] }, budget);
    assert.equal(again.kind, "exhausted");
    assert.equal(budget.used(), MAX_TURNS, "refusals never increment the counter");
  });

  it("charges nothing for a refused request shape", () => {
    const budget = createTurnBudget();
    const badBodies: unknown[] = [
      null,
      {},
      { messages: [] },
      { messages: [userTurn("")] },
      { messages: [assistantTurn("reply")] },
      { messages: [userTurn("x".repeat(MAX_MESSAGE_CHARS + 1))] },
    ];
    for (const body of badBodies) {
      assert.equal(decideChatTurn(body, budget).kind, "invalid");
    }
    assert.equal(budget.used(), 0, "validation failures cost no turns");

    for (let i = 0; i < MAX_TURNS; i += 1) {
      assert.equal(decideChatTurn({ messages: [userTurn(`turn ${i}`)] }, budget).kind, "turn");
    }
    assert.equal(decideChatTurn({ messages: [userTurn("last")] }, budget).kind, "exhausted");
  });

  it("keeps budgets independent, so a fresh container starts full", () => {
    const first = createTurnBudget();
    for (let i = 0; i < MAX_TURNS; i += 1) first.take();
    assert.equal(first.take(), false);

    const second = createTurnBudget();
    assert.equal(second.remaining(), MAX_TURNS);
    assert.equal(second.take(), true);
  });
});
