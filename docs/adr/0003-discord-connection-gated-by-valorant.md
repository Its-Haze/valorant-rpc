# Discord's connection is gated by Valorant's process, not independent of it

**Status**: accepted

ADR-0002 established that the Discord and Riot Connection Supervisors run independently: Valorant being closed must not block waiting for Discord, and vice versa. That holds for the Riot side, but for Discord it would mean connecting and publishing presence the moment Discord itself was running, whether or not Valorant was open. Closing Valorant would leave the last real presence sitting on the user's profile indefinitely, because nothing ever told the Discord connection to go away.

There is no presence worth showing without Valorant running, so there is no reason to hold a Discord IPC connection open without it. `DiscordSupervisor` takes a `GameDetector` (satisfied by `RiotSupervisor`) and requires `GameRunning()` to be true both before attempting `Connect()` and for its gated `IsConnected()` to report true. This reuses the Supervisor's existing retry-on-disconnect loop rather than adding teardown logic: the moment Valorant's process disappears the gated `IsConnected()` flips false, `Supervisor.Run()` disconnects and goes back to polling the gate, and the connection returns only when the process does.

**Consequences**: the presence loop logs once, not per poll tick, when Valorant is running but Discord isn't reachable yet, so a user can tell the app is waiting rather than stuck. This supersedes ADR-0002's "the two supervisors run independently" for the Discord side; the Riot Supervisor is unaffected.

The process gate ANDs two separate `IsRunning` calls rather than passing both names to one, because `Checker.IsRunning` is an any-of. Both `VALORANT-Win64-Shipping.exe` and `RiotClientServices.exe` have to be present. Checking only the Riot Client would put a Valorant-branded card on screen while someone plays League or Teamfight Tactics, which is the same mistake league-rpc's version of this ADR rejected from the other direction.

`VALORANT-Win64-Shipping.exe` covers the menus too. It is the Valorant client, not just a live match, so gating on it does not mean presence disappears between games.
