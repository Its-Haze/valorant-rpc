# The menu screen comes from Valorant's log, because the presence payload cannot tell

**Status**: accepted

Valorant puts every player in a party with a queue already selected, from login onward. Sitting on the home screen and sitting in a Play lobby are both `sessionLoopState: MENUS` with `partyState: DEFAULT`, and a capture of each differs only in `queueId`, which is set either way because the client preselects one. So a player who launches the game and touches nothing published a presence that read "In lobby · Unrated". They had opened no lobby and chosen no mode.

Every field of the private presence blob was checked against live captures. `/chat/v4/presences` carries party, match and player data and nothing about the client's UI. The remote glz party endpoints do not carry it either, and reaching for more of Riot's API to answer a cosmetic question is the wrong trade.

Valorant's own log answers it. Two lines, both at a level that is on by default:

```
LogMenuStackManager: Opening HomeScreen_PC_C
LogUINavigationModel: Warning: Current Url: main/lobby
```

`gamelog.Reader.MenuScreen` scans the tail for the last line that moved the UI and reports `ScreenLobby` for a route under `main/lobby`, `ScreenClient` for the home screen and for every other section, and `ScreenUnknown` for a log that has not drawn a menu. Empty routes and `main/settingsingame` are passed over, so opening settings from a lobby does not read as leaving it. `State.PhaseContext` then splits `MENUS` + `DEFAULT` into `in-client` and `in-lobby`.

A custom lobby defers to the screen too. It outlives the page it sits on, so backing out to the home screen with one open reads as `in-client`; only `ScreenClient` overrides it, so an unread screen keeps the lobby rather than denying one the player did open. Matchmaking does not defer: queueing runs whatever page is open and ends by pulling the player into a match, so browsing the store while queued is still `in-queue`.

`in-client` shows availability and nothing else. The mode and the competitive tier both track `queueId`, which Valorant preselects, so both were stating a choice the player had not made; `isRanked()` returns false there and the rank token is not offered. Both come back in `in-lobby`, where the player picked the queue.

**Consequences**: this is log scraping, with the same fragility ADR-0006 and the agent lookup already carry. A Riot rename stops it matching silently. The failure is one-directional on purpose: an unknown screen reads as `in-client`, so a broken signal understates rather than announcing a lobby nobody opened, which is the bug this fixes.

The read is gated on the menus. In a match the log grows fast and the screen could not change the context anyway, so it is skipped rather than paid for on every poll.

No new endpoint, no network call, no write. It is the same file `internal/gamelog` already opens for the agent, which is why the two share a `Reader`.
