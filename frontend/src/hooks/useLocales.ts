import { GetLocales } from "../../bindings/github.com/its-haze/valorant-rpc/cmd/valorant-rpc-gui/guiservice";
import type { Locale } from "../../bindings/github.com/its-haze/valorant-rpc/internal/app/models";
import { createExternalStore } from "./createExternalStore";

// The language list is compiled into the binary, so it is fetched once and
// shared rather than re-fetched on every Display screen remount.
const store = createExternalStore<Locale[]>([], () => {
  GetLocales()
    .then((l) => store.set(l ?? []))
    .catch(() => {});
});

export function useLocales(): Locale[] {
  return store.useValue();
}
