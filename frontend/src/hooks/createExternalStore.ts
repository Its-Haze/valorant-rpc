import { useSyncExternalStore } from "react";

export interface ExternalStore<T> {
  useValue(): T;
  get(): T;
  set(next: T): void;
  /** Seeds the store from its initial fetch, unless an event beat it here.
   * Without the guard a slow fetch overwrites the newer value. */
  setInitial(next: T): void;
}

// A minimal useSyncExternalStore-backed store, shared module-wide so every
export function createExternalStore<T>(initialValue: T, init: () => void): ExternalStore<T> {
  let value = initialValue;
  const listeners = new Set<() => void>();
  let initialized = false;
  let written = false;

  function get(): T {
    return value;
  }

  function set(next: T): void {
    written = true;
    value = next;
    listeners.forEach((l) => l());
  }

  function setInitial(next: T): void {
    if (written) return;
    set(next);
  }

  function subscribe(listener: () => void): () => void {
    if (!initialized) {
      initialized = true;
      init();
    }
    listeners.add(listener);
    return () => listeners.delete(listener);
  }

  function useValue(): T {
    return useSyncExternalStore(subscribe, get);
  }

  return { useValue, get, set, setInitial };
}
