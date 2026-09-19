import { describe, expect, it } from "vitest";
import { PRESENCE_CONTEXTS, PRESENCE_CONTEXT_LABELS, isPresenceContext } from "./presenceContexts";

describe("PRESENCE_CONTEXTS", () => {
  // The same literal list TestContextKeysAreStable pins in internal/discord.
  it("matches the keys the Go builders are registered under", () => {
    expect([...PRESENCE_CONTEXTS]).toEqual([
      "in-client",
      "in-queue",
      "custom-game",
      "agent-select",
      "in-match",
    ]);
  });

  it("labels every context", () => {
    for (const ctx of PRESENCE_CONTEXTS) {
      expect(PRESENCE_CONTEXT_LABELS[ctx]).toBeTruthy();
    }
  });
});

describe("isPresenceContext", () => {
  it("accepts every known context", () => {
    for (const ctx of PRESENCE_CONTEXTS) {
      expect(isPresenceContext(ctx)).toBe(true);
    }
  });

  it("rejects unknown values", () => {
    expect(isPresenceContext("bogus")).toBe(false);
    expect(isPresenceContext("")).toBe(false);
  });
});
