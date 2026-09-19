// The presence contexts internal/presence/template knows about, display order.
// TestContextKeysAreStable in internal/discord pins the Go side of this list.
export const PRESENCE_CONTEXTS = [
  "in-client",
  "in-queue",
  "custom-game",
  "agent-select",
  "in-match",
] as const;

export type PresenceContext = (typeof PRESENCE_CONTEXTS)[number];

export const PRESENCE_CONTEXT_LABELS: Record<PresenceContext, string> = {
  "in-client": "In lobby",
  "in-queue": "In queue",
  "custom-game": "Custom game",
  "agent-select": "Agent select",
  "in-match": "In a match",
};

export function isPresenceContext(value: string): value is PresenceContext {
  return (PRESENCE_CONTEXTS as readonly string[]).includes(value);
}
