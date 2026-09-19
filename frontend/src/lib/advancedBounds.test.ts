import { describe, expect, it } from "vitest";
import { UPDATE_INTERVAL_BOUNDS, clampToBounds, formatIntervalSeconds } from "./advancedBounds";

describe("clampToBounds", () => {
  it("passes values already in range through unchanged", () => {
    expect(clampToBounds(1500, UPDATE_INTERVAL_BOUNDS)).toBe(1500);
  });

  it("clamps below the minimum", () => {
    expect(clampToBounds(10, UPDATE_INTERVAL_BOUNDS)).toBe(500);
  });

  it("clamps above the maximum", () => {
    expect(clampToBounds(999999, UPDATE_INTERVAL_BOUNDS)).toBe(10000);
  });

  it("falls back to the minimum for NaN", () => {
    expect(clampToBounds(NaN, UPDATE_INTERVAL_BOUNDS)).toBe(500);
  });
});

describe("formatIntervalSeconds", () => {
  it("formats a whole-second value with no decimal", () => {
    expect(formatIntervalSeconds(10000)).toBe("10s");
  });

  it("formats a fractional value to one decimal place", () => {
    expect(formatIntervalSeconds(1500)).toBe("1.5s");
  });

  it("rounds to the nearest tenth of a second", () => {
    expect(formatIntervalSeconds(1540)).toBe("1.5s");
    expect(formatIntervalSeconds(1560)).toBe("1.6s");
  });
});
