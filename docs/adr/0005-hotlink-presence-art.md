# Presence art is hotlinked from valorant-api.com, not mirrored

**Status**: accepted

Agent, map and competitive tier art comes from `media.valorant-api.com` at stable per-UUID URLs, and the app hands Discord those URLs directly. Nothing is mirrored into a repository and nothing is uploaded as a Discord application asset.

Mirroring was considered, by analogy with league-rpc's `Its-Haze/league-assets`. It trades a rare CDN outage for guaranteed slow rot, and rot is the worse failure because it is invisible: an outage is loud and fixes itself, while a mirror that stopped being updated two agents ago just quietly shows the wrong picture. The archived `colinhartigan/valorant-rpc` uploaded PNGs as Discord application assets and ended up carrying 19 agents against today's count, with rank icons stopping at tier 24 when Radiant is 27. Nobody noticed for a long time.

Discord application asset keys were rejected for the same reason, plus a hard ceiling on how many an application may hold.

**Consequences**: the app depends on a free community API at presence-send time, and a valorant-api.com outage means missing images until it returns. That is accepted. The mitigation is that a missing image degrades to no picture rather than a broken presence.

Because nothing here is under our control, the guard is a test rather than a process. `internal/discord/assets_live_test.go` resolves every agent, map and tier UUID the catalogue knows about against the live API and fails on a 404. It samples player cards rather than checking all of them, with `VALORANT_RPC_FULL_ASSET_SCAN` to check every one. That test is the whole reason hotlinking is safe to choose, so it must not be allowed to rot into a skip.

Two images are exceptions and *are* committed here: `assets/valorant-logo.png` and its dimmed idle variant, hotlinked out of this repository the way league-rpc hotlinks its own. They are the fallback when no Riot art applies. `TestLogoURLsPointAtCommittedAssets` proves the files exist in the tree, and `TestRepoHostedImagesResolve` proves the URLs actually serve an image.

That second test earns its keep. GitHub answers a request for a *missing* file under `blob/<branch>/<path>?raw=true` with 200 and an HTML page, not a 404, so a broken hotlink passes a status-code check cleanly. `probeImage` requires a `Content-Type` of `image/*`. Any future check of a repo-hosted image must go through it rather than trusting the status alone.
