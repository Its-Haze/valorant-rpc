import { describe, expect, it } from "vitest";
import { createExternalStore } from "./createExternalStore";

describe("setInitial", () => {
  it("applies the fetched value when nothing has arrived yet", () => {
    const store = createExternalStore<string | null>(null, () => {});
    store.setInitial("fetched");
    expect(store.get()).toBe("fetched");
  });

  it("drops the fetched value when an event got there first", () => {
    const store = createExternalStore<string | null>(null, () => {});
    store.set("live event");
    store.setInitial("fetched, but stale");
    expect(store.get()).toBe("live event");
  });

  it("only guards the first write, so later events still land", () => {
    const store = createExternalStore<string | null>(null, () => {});
    store.setInitial("fetched");
    store.set("later event");
    expect(store.get()).toBe("later event");
  });
});
