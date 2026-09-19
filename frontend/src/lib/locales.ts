import type { Locale } from "../../bindings/github.com/its-haze/valorant-rpc/internal/app/models";

// The display.locale value that follows the Riot Client. Mirrors
// types.LocaleAuto, which is the only non-tag the setting accepts.
export const LOCALE_AUTO = "auto";

// Labels the automatic option with the language it actually resolved to, so
export function autoLocaleLabel(resolvedTag: string, locales: Locale[]): string {
  const match = locales.find((l) => l.tag === resolvedTag);
  return match ? `Automatic (${match.name})` : "Automatic";
}
