// Fallback bounds, used only until GetConfigBounds() resolves (useConfigBounds).
export const UPDATE_INTERVAL_BOUNDS = { min: 500, max: 10000 };

export interface Bounds {
  min: number;
  max: number;
}

// Clamps value into bounds. A non-numeric input (NaN, from an empty or
// partially-typed field) falls back to the nearer bound's min.
export function clampToBounds(value: number, bounds: Bounds): number {
  if (Number.isNaN(value)) return bounds.min;
  return Math.min(bounds.max, Math.max(bounds.min, value));
}

// Formats a millisecond duration as a plain seconds label for non-technical
// users, e.g. 1500 -> "1.5s", 10000 -> "10s". At most one decimal place.
export function formatIntervalSeconds(ms: number): string {
  const seconds = Math.round(ms / 100) / 10;
  return `${seconds}s`;
}
