import type { Config } from "../../bindings/github.com/its-haze/valorant-rpc/internal/config/models";

// A representative Config tree for pure-logic tests; not the source of truth
// for defaults (internal/config.DefaultConfig() is), just a fixture shape.
export function DefaultConfig(): Config {
  return {
    schema_version: 1,
    discord_app_id: "1194034071588851783",
    theme: "system",
    onboarding_complete: false,
    display: {
      default: { show_rank: true, show_stats: true },
      locale: "auto",
    },
    presence: {
      show_in_client: true,
      templates: {
        "in-client": { details: "In the client", state: "{rank} · {idle}" },
        "in-queue": { details: "{mode}", state: "In queue · {party} · {idle}" },
        "custom-game": { details: "{map}", state: "Custom game · {party} · {idle}" },
        "agent-select": { details: "{mode} · {map}", state: "Agent select · {agent} · {party}" },
        "in-match": { details: "{mode} · {map}", state: "In a match · {score} · {agent}" },
      },
    },
    behavior: {
      launch_at_startup: false,
      close_action: "ask",
      notify_updates: true,
      show_placeholder_presence: false,
    },
    advanced: {
      update_interval: 1500,
      debug_mode: false,
    },
  };
}
