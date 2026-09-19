import { describe, expect, it } from "vitest";
import { DefaultConfig } from "./testFixtures";
import { withLocale, withShowInClient, withShowRank, withShowStats } from "./displayPatch";

describe("withShowRank", () => {
  it("sets show_rank and keeps sibling display fields", () => {
    const cfg = DefaultConfig();
    const patch = withShowRank(cfg, false);
    expect(patch.display).toEqual({
      ...cfg.display,
      default: { ...cfg.display.default, show_rank: false },
    });
  });
});

describe("withShowStats", () => {
  it("sets show_stats and keeps sibling display fields", () => {
    const cfg = DefaultConfig();
    const patch = withShowStats(cfg, false);
    expect(patch.display).toEqual({
      ...cfg.display,
      default: { ...cfg.display.default, show_stats: false },
    });
  });
});

describe("withLocale", () => {
  it("sets the locale and keeps sibling display fields", () => {
    const cfg = DefaultConfig();
    const patch = withLocale(cfg, "sv-SE");
    expect(patch.display).toEqual({ ...cfg.display, locale: "sv-SE" });
  });
});

describe("withShowInClient", () => {
  it("sets show_in_client and keeps sibling presence fields", () => {
    const cfg = DefaultConfig();
    const patch = withShowInClient(cfg, false);
    expect(patch.presence).toEqual({ ...cfg.presence, show_in_client: false });
  });
});
