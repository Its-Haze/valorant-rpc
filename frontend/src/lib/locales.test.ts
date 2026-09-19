import { describe, expect, it } from "vitest";
import { LOCALE_AUTO, autoLocaleLabel } from "./locales";

const LOCALES = [
  { tag: "en-US", name: "English" },
  { tag: "sv-SE", name: "Svenska" },
];

describe("autoLocaleLabel", () => {
  it("names the language the client actually resolved to", () => {
    expect(autoLocaleLabel("sv-SE", LOCALES)).toBe("Automatic (Svenska)");
  });

  it("stays a bare label before the first status snapshot lands", () => {
    expect(autoLocaleLabel("", LOCALES)).toBe("Automatic");
  });

  it("does not invent a name for a tag the list does not carry", () => {
    expect(autoLocaleLabel("xx-XX", LOCALES)).toBe("Automatic");
  });
});

describe("LOCALE_AUTO", () => {
  it("matches the sentinel the Go config validates against", () => {
    expect(LOCALE_AUTO).toBe("auto");
  });
});
