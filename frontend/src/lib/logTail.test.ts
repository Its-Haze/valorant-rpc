import { describe, expect, it } from "vitest";
import {
  MAX_TAIL_LINES,
  appendLine,
  appendLines,
  dropHistoryOverlap,
  isScrolledToBottom,
} from "./logTail";

describe("appendLine", () => {
  it("appends a line", () => {
    expect(appendLine(["a"], "b")).toEqual(["a", "b"]);
  });

  it("drops the oldest lines once past the cap", () => {
    const lines = Array.from({ length: MAX_TAIL_LINES }, (_, i) => String(i));
    const next = appendLine(lines, "new");
    expect(next.length).toBe(MAX_TAIL_LINES);
    expect(next[0]).toBe("1");
    expect(next[next.length - 1]).toBe("new");
  });
});

describe("appendLines", () => {
  it("appends a batch in one pass", () => {
    expect(appendLines(["a"], ["b", "c"])).toEqual(["a", "b", "c"]);
  });

  it("is a no-op for an empty batch", () => {
    const lines = ["a", "b"];
    expect(appendLines(lines, [])).toBe(lines);
  });

  it("drops the oldest lines once the batch pushes past the cap", () => {
    const lines = Array.from({ length: MAX_TAIL_LINES - 1 }, (_, i) => String(i));
    const next = appendLines(lines, ["x", "y", "z"]);
    expect(next.length).toBe(MAX_TAIL_LINES);
    expect(next[next.length - 1]).toBe("z");
  });
});

describe("isScrolledToBottom", () => {
  it("is true when scrolled all the way down", () => {
    expect(isScrolledToBottom(100, 50, 150)).toBe(true);
  });

  it("is true within tolerance", () => {
    expect(isScrolledToBottom(80, 50, 150, 24)).toBe(true);
  });

  it("is false once scrolled up past tolerance", () => {
    expect(isScrolledToBottom(0, 50, 150, 24)).toBe(false);
  });
});

describe("dropHistoryOverlap", () => {
  it("drops the buffered lines the history already covers", () => {
    const history = ["a", "b", "c"];
    const pending = ["b", "c", "d"];
    expect(dropHistoryOverlap(history, pending)).toEqual(["d"]);
  });

  it("keeps everything when the two do not meet", () => {
    expect(dropHistoryOverlap(["a", "b"], ["c", "d"])).toEqual(["c", "d"]);
  });

  it("drops the whole buffer when the history already has all of it", () => {
    expect(dropHistoryOverlap(["a", "b", "c"], ["b", "c"])).toEqual([]);
  });

  it("prefers the longest overlap, so a repeated line does not cut it short", () => {
    const history = ["x", "x", "y"];
    const pending = ["x", "y", "z"];
    expect(dropHistoryOverlap(history, pending)).toEqual(["z"]);
  });

  it("handles either side being empty", () => {
    expect(dropHistoryOverlap([], ["a"])).toEqual(["a"]);
    expect(dropHistoryOverlap(["a"], [])).toEqual([]);
  });
});
