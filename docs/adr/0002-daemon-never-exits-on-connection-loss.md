# The Daemon never exits on connection loss: it supervises reconnects forever

**Status**: accepted

The target shape is a background app: launched at Windows startup or manually, living in the system tray, running indefinitely. A user should never need to restart it by hand, not because Valorant isn't open yet, not because Valorant or Discord closed mid-session, not because Discord wasn't running when the Daemon started. Only a crash or an explicit tray "Quit" should end the process.

That rules out a one-shot `Connect()` / run / `Disconnect()` lifecycle. Each client is wrapped by a Connection Supervisor that retries indefinitely with backoff and treats "the other side closed" as an expected steady state to retry through, not an error that unwinds `main()`.

The two supervisors, Discord and Riot, run independently, so Valorant being closed never blocks on Discord and vice versa. ADR-0003 narrows this for the Discord side specifically.

**Consequences**: the Daemon's top-level goroutine starts the supervisors and then blocks only on an explicit shutdown signal, never on a connection error path. `discord.Client.Connect()` and the Riot Client's connect keep their one-shot-per-attempt signatures; supervising them means calling them in a retry loop from outside, not rewriting their internals.

No Riot Client connection means no presence, uniformly, with one exception: while Valorant's process is running but no presence has been read yet, a placeholder may show. That is gated behind `Behavior.ShowPlaceholderPresence`, which defaults to off, because unlike League there is a real chance a second RPC tool is already showing something and two cards fighting is worse than none.

A time-based grace period on disconnect was considered and rejected, for the same reason as in league-rpc: it trades a correct signal for a guess about why the connection dropped. Process detection answers the one case, active launch, that deserves an idle card instead of nothing.

One difference from league-rpc worth naming. There, a live LCU connection that has gone quiet is indistinguishable from a slow start, so the stall signal is time-based. Here `RiotSource.PresenceStalled` means "this connection has never read a presence", because `Watcher.Start` fetches the current snapshot the moment it connects. Once any presence arrives the connection is never stalled again, and a reconnect restarts the window.

**Amendment, 2026-09-19 (ticket 10)**: the placeholder now defaults to on. The reasoning above, that a second RPC tool may already be showing something, does not survive reading league-rpc: it gates its own placeholder on the League process being up and clears presence otherwise, so neither app ever holds an idle card and there is nothing to collide with. `Behavior.ShowPlaceholderPresence` stays, as a preference rather than a defence.
