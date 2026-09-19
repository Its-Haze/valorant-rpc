import type { StatusSnapshot } from "../../bindings/github.com/its-haze/valorant-rpc/internal/app/models";

export type ConnectionTone = "ok" | "warn" | "idle";

export interface ConnectionSummary {
  label: string;
  tone: ConnectionTone;
}

// Collapses the status snapshot into one sidebar line. Order is precedence:
// an explicit pause outranks everything, and no Valorant means nothing else runs.
export function summarizeConnection(status: StatusSnapshot | null): ConnectionSummary {
  if (!status) return { label: "Starting up", tone: "idle" };
  if (status.paused) return { label: "Paused", tone: "warn" };
  if (!status.game_running) return { label: "Valorant closed", tone: "idle" };
  if (!status.riot_connected) return { label: "Connecting", tone: "warn" };
  // A connection that has never produced a presence is the one condition that
  // means something is actually broken rather than merely slow to start.
  if (status.presence_stalled) return { label: "Can't reach Riot Client", tone: "warn" };
  if (!status.discord_connected) return { label: "No Discord", tone: "warn" };
  return { label: "Connected", tone: "ok" };
}
