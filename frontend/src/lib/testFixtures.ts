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
      default: { show_rank: true, show_stats: true, show_kills: false, match_image: "agent" },
      locale: "auto",
    },
    presence: {
      show_in_client: true,
      templates: {
        "in-client": { details: "{mode}", state: "In lobby · {party} · {idle}" },
        "in-queue": { details: "{mode}", state: "In queue · {party} · {idle}" },
        "custom-game": { details: "{mode}", state: "In lobby · {party} · {idle}" },
        "agent-select": { details: "{mode} · {map}", state: "Agent select · {party}" },
        "in-match": { details: "{mode} · {map}", state: "In a match · {score}" },
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
