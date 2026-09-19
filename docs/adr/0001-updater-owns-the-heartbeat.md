# Updater owns debounce, heartbeat, and reclaim as one goroutine

**Status**: accepted

Discord lets another client overwrite our Rich Presence if we stop sending updates. Valorant has no native Discord integration to lose to, but the Discord client itself still drops a presence that goes quiet, and a user running a second RPC tool is a real case. Holding a presence requires three behaviours working together: an initial debounced send, a periodic heartbeat resend so ours stays current, and a faster "reclaim burst" right after any real change, all serialized so they never race each other on the same Discord connection.

All of that lives in `discord.Updater` (`internal/discord/updater.go`), which debounces `State` changes behind a mutex. The debounce timer is a repeating ticker rather than a one-shot: after the first real send, `Updater` keeps resending the current presence on a heartbeat cadence, with a shorter interval for a few cycles right after a real change, until the next real change resets it.

`discord.Client.UpdatePresence`/`ClearPresence` must only ever be called through `Updater`. No separate goroutine touches the Discord connection directly, so `Updater`'s existing `sync.Mutex` is enough to serialize debounce, heartbeat, and reclaim against each other.

Running debounce, heartbeat, and reclaim as separate goroutines, each owning its own timer, was considered for a cleaner separation of concerns. Rejected: a single goroutine driven by one ticker covers all three with one clock and one lock, and splitting them apart would only add coordination between goroutines that all touch the same connection anyway.

This decision is inherited from league-rpc, where the overwrite pressure is stronger because League ships its own Discord integration. The mechanism is worth keeping here even though the adversary is weaker, since the cost is one ticker.
