# Local first, with a read-only remote tier, and never the game process

**Status**: accepted

Valorant runs Vanguard, a kernel-level anti-cheat. Three tiers of data access were considered.

**Reading or hooking the game process is out, permanently.** Not "out for now", not "behind a setting". There is no presence detail worth putting a user's account at risk for, and nothing in this app will ever read game memory, inject, or touch game files.

**The local tier is everything the Riot Client already publishes on the user's own machine.** `/chat/v4/presences` on the lockfile port carries match state, map, mode, round score, coarse rank, party size, account level and idle. This is the whole of v0.1, and it cannot get anyone banned because it is the same data the client hands its own UI.

**The remote tier is read-only calls to Riot's production API, for one thing only: which agent the player is on.** The local presence blob does not carry it. Getting it means an entitlement token from the local `/entitlements/v1/token` endpoint and two `glz` reads. That is v0.2, kept separate precisely because it is the only part with unknown difficulty and an external dependency.

Constraints that hold regardless of tier:

- **No writes to any Riot endpoint.** No party join, no party code generation, no friend requests, no name service, no storefront. Writes are far closer to the automation that actually gets accounts banned, and this line does not move without an explicit decision.
- **Only the player's own data is read.** `core-game` returns all ten players plus game server connection details. None of that is read, logged, or displayed.
- **No username/password flow.** Tokens come from the local endpoint, which requires the client to already be running and logged in. The app never sees a credential.
- **Backoff on failure, and stop entirely after repeated failures** until the next game start. A failing version header fails identically every time, so retrying is both useless and the kind of traffic that attracts attention.

**Consequences**: the remote tier will break, probably on a client patch day, when a version header shifts or an endpoint moves. When it does, the agent drops out of the presence and everything else keeps working, plus a visible status: a state on the status bridge, a line on the Help screen, and a log entry. Not a toast, because this is a cosmetic degradation and interrupting someone mid-match over a missing agent name is worse than the missing name.

Because the remote tier authenticates to Riot on the user's behalf, it gets a settings row and plain-language copy saying so, and a kill switch. A user who would rather not have the app talk to Riot's servers at all can have exactly the v0.1 behaviour.
