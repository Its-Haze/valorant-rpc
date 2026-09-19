# The Riot Client transport is written here, not borrowed from lcu-gopher

**Status**: accepted

`Its-Haze/lcu-gopher` already implements the thing this app needs: lockfile discovery, an HTTP client that tolerates Riot's self-signed local certificate, and a WAMP-framed websocket over `wss://riot:{password}@localhost:{port}`. The Riot Client speaks the same framing as the League Client, down to the subscribe frame `[5, "OnJsonApiEvent_..."]` and the event shape `[8, name, {data, eventType, uri}]`. Reusing it looks obvious.

We are not reusing it. lcu-gopher is a published library that `league-rpc` pins in `go.mod`, and league-rpc has real users. Generalizing it from "League Client" to "any Riot local API" is a breaking change to its surface, and it would make a release of a library that one shipping app depends on into a prerequisite for starting a second app. The dependency runs the wrong way: an unreleased project would be dictating changes to a released one.

So valorant-rpc implements `internal/riotclient` and `internal/riotchat` itself, duplicating the envelope parsing, reconnect and dispatch that lcu-gopher already has.

**Consequences**: the duplication is real and will drift. It sits on a known extraction list alongside the game-agnostic infrastructure copied from league-rpc, and a shared `riot-rpc-core` becomes worth doing once valorant-rpc has shipped a stable release and league-rpc has no pending work. Not before: an extraction that lands while both apps are moving has to chase two sets of changes at once.

Until then, a bug found in the copied transport should be checked against lcu-gopher and league-rpc and reported, not silently fixed in one tree.

Polling on a timer instead of subscribing was considered and rejected. The archived `colinhartigan/valorant-rpc` wrote websocket support and then commented it out in favour of a timer. Polling is the wrong default when the client will push, and it costs latency on every phase change.
