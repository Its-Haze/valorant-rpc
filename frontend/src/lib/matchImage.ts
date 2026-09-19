// The large-image choices during a match. These strings are the config
// values and are pinned on the Go side by config.MatchImageAgent.
export const MATCH_IMAGE_AGENT = "agent";
export const MATCH_IMAGE_CARD = "card";

export const MATCH_IMAGE_OPTIONS = [
  { value: MATCH_IMAGE_AGENT, label: "Agent you're playing" },
  { value: MATCH_IMAGE_CARD, label: "Your player card" },
];
