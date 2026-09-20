import { describe, expect, it } from "vitest";
import type { StatusSnapshot } from "../../bindings/github.com/its-haze/valorant-rpc/internal/app/models";
import { summarizeConnection } from "./connectionStatus";

function snapshot(over: Partial<StatusSnapshot> = {}): StatusSnapshot {
  return {
    game_running: true,
    riot_connected: true,
    presence_stalled: false,
    discord_connected: true,
    paused: false,
    context: "in-client",
    presence: null,
    presence_cleared: false,
    ...over,
  };
}

describe("summarizeConnection", () => {
  it("reports connected when everything is up", () => {
    expect(summarizeConnection(snapshot())).toEqual({ label: "Connected", tone: "ok" });
  });

  it("shows a placeholder before the first snapshot lands", () => {
    expect(summarizeConnection(null).tone).toBe("idle");
  });

  it("puts pause ahead of every connection state", () => {
    const s = snapshot({ paused: true, game_running: false, discord_connected: false });
    expect(summarizeConnection(s).label).toBe("Paused");
  });

  it("treats a closed Valorant as idle, not a fault", () => {
    expect(summarizeConnection(snapshot({ game_running: false }))).toEqual({
      label: "Valorant closed",
      tone: "idle",
    });
  });

  it("distinguishes a pending Riot Client from a missing Discord", () => {
    expect(summarizeConnection(snapshot({ riot_connected: false })).label).toBe("Connecting");
    expect(summarizeConnection(snapshot({ discord_connected: false })).label).toBe("No Discord");
  });

  it("calls out a connection that never produced a presence", () => {
    const s = snapshot({ presence_stalled: true });
    expect(summarizeConnection(s)).toEqual({ label: "Can't reach Riot Client", tone: "warn" });
  });

  // PresenceStalled reads Connected() on the Go side, so the two can't
  // disagree; the still-connecting label has to win anyway.
  it("keeps saying Connecting while the client is not up yet", () => {
    const s = snapshot({ riot_connected: false, presence_stalled: true });
    expect(summarizeConnection(s).label).toBe("Connecting");
  });
});
