# Valorant RPC

Discord Rich Presence for Valorant, driven by the Riot Client's own local API.

This glossary is this project's alone. `league-rpc` is a sibling application with a separate
glossary, and a term defined there says nothing here even where the code was copied. A game
flow phase is not a session loop state, and a summoner is not a player.

## Language

**Presence blob**:
The base64-encoded `private` field on an entry from `/chat/v4/presences`, decoded and JSON-parsed. It is the source of everything v0.1 shows. Riot is mid-migration between a flat shape and a nested one where `sessionLoopState` and `matchMap` sit under `matchPresenceData` and the party fields under `partyPresenceData`. **Both shapes are normalized at the edge, in `internal/riotchat`.** If presence data is ever missing or wrong, look here first.
_Avoid_: payload, presence data (bare)

**Session loop state**:
Riot's own string for what the client is doing: `MENUS`, `PREGAME`, `INGAME`. A contract with Riot's client, not a value we invent. An unrecognized value degrades to the client context rather than erroring.
_Avoid_: game flow phase (that is league-rpc's term), game state, phase

**Party state**:
Riot's string subdividing `MENUS`: `DEFAULT`, `MATCHMAKING`, `CUSTOM_GAME_SETUP`. It keeps reporting `MATCHMAKING` all the way through a match, which is why session loop state is checked first.
_Avoid_: lobby state, queue state

**Context**:
One of the five situations a presence is built for: `in-client`, `in-queue`, `custom-game`, `agent-select`, `in-match`. Derived from session loop state and party state by `State.PhaseContext()`. Each maps to one builder and one user-editable template pair. The exact strings are config keys, pinned on the Go side by `TestContextKeysAreStable` and duplicated in `frontend/src/lib/presenceContexts.ts`.
_Avoid_: phase, screen, mode (mode means something else, below)

**Provisioning flow**:
Riot's string for how the current match was created. `ShootingRange` is the only one that matters: the range is rendered as a variant of `in-match`, not a sixth context, and its leftover round score is suppressed because the range has no rounds.
_Avoid_: match type

**Idle**:
Riot's `isIdle` flag, meaning the player is away. It cuts across every context, so it is a template token and a dimmed small icon, never a context of its own. Making it one would duplicate every other context's idle variant.

**Mode**:
The player-facing name for a queue, from the queue ID string (`competitive`, `swiftplay`, `hurm`). valorant-api.com returns `queueID: null` on every game mode entry, so there is no published mapping and it is hardcoded in `internal/content`. An unknown queue ID renders as a title-cased version of the raw string, never "Unknown", so a new mode degrades to something readable.
_Avoid_: queue (bare), playlist

**Catalogue**:
One resolved, immutable snapshot of agents, maps, competitive tiers, game modes and player cards from valorant-api.com, holding every language at once. Built by `internal/content` and swapped wholesale on refresh, so readers need no lock. A cold catalogue means names render empty, not wrong.
_Avoid_: cache (that is the thing that holds the catalogue), asset data

**Locale**:
The language names render in, always `types.DefaultLocale` (`en-US`). It is a lookup parameter on the catalogue, not a setting: the language picker was removed because the only thing it changed was the rank name. The catalogue still carries every language, so restoring a picker costs a config field and a dropdown, not a refetch.
_Avoid_: language, client locale

**Presence stalled**:
A Riot Client connection that is up but has never produced a presence. Not a timeout: `Watcher.Start` fetches the current snapshot the moment it connects, so once any presence arrives the connection is never stalled again, and a reconnect restarts the window. This is the condition that actually indicates something broken, as opposed to a slow start.
_Avoid_: disconnected, timed out

**Local tier**:
Everything the Riot Client publishes on the user's own machine, reached over the lockfile port. The whole of v0.1. See [ADR-0006](./docs/adr/0006-local-first-with-a-read-only-remote-tier.md).

**Remote tier**:
Read-only calls to Riot's production API, for the player's agent and nothing else. v0.2. Never writes, never reads another player's data. See [ADR-0006](./docs/adr/0006-local-first-with-a-read-only-remote-tier.md).
_Avoid_: the API (bare), online mode

**Entitlement**:
The short-lived credential set the remote tier needs, from the local `/entitlements/v1/token`: an `accessToken`, an entitlement JWT, and `subject`, the player's puuid. It expires an hour after generation and is refreshed on an HTTP 400. The app never sees a username or password.

**Region and shard**:
Two different things that are often equal and must not be assumed to be. The region comes from `-ares-deployment=` in the client's launch arguments; the glz hostname embeds `{region}-{number}` where the number is not always `1`. Both are read locally, never hardcoded.

**Updater**:
The single component responsible for every Discord presence send. It debounces rapid state changes into one send, then keeps resending on a heartbeat while that presence is current, with a faster burst right after each real change. Nothing else may touch the Discord connection. See [ADR-0001](./docs/adr/0001-updater-owns-the-heartbeat.md).
_Avoid_: RPCUpdater, presence manager

**App Update**:
The in-app self-update flow, which is a different thing from **Updater** above despite the name collision inherited from league-rpc. App Update downloads a signed release binary and swaps it; Updater sends Discord presence. When both appear in one sentence, say "App Update" and "the presence Updater".

**Placeholder presence**:
The "Launching VALORANT..." card shown while Valorant is running but no presence has been read yet, wearing a random player card as its art and a new one on every rotation. On by default, behind `Behavior.ShowPlaceholderPresence` for anyone who would rather show nothing until the game reports something.
