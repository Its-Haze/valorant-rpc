import type { Config } from "../../bindings/github.com/its-haze/valorant-rpc/internal/config/models";

// Pure patch builders for the global display defaults, shared by the Display
// screen and the onboarding walkthrough's "display" step.

export function withShowRank(cfg: Config, enabled: boolean): Partial<Config> {
  return { display: { ...cfg.display, default: { ...cfg.display.default, show_rank: enabled } } };
}

export function withShowStats(cfg: Config, enabled: boolean): Partial<Config> {
  return { display: { ...cfg.display, default: { ...cfg.display.default, show_stats: enabled } } };
}

export function withMatchImage(cfg: Config, choice: string): Partial<Config> {
  return { display: { ...cfg.display, default: { ...cfg.display.default, match_image: choice } } };
}

export function withLocale(cfg: Config, locale: string): Partial<Config> {
  return { display: { ...cfg.display, locale } };
}

export function withShowInClient(cfg: Config, enabled: boolean): Partial<Config> {
  return { presence: { ...cfg.presence, show_in_client: enabled } };
}
