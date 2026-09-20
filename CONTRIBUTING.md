# Contributing

Thanks for taking a look. Bug reports and pull requests are both welcome. If you are about to build something large, open an issue first so we can agree on the shape of it before you spend a weekend on it.

## Prerequisites

- **Go**, at the version pinned in [`go.mod`](go.mod).
- **Node.js**, at the version the release workflow builds on, in [`.github/workflows/release.yml`](.github/workflows/release.yml). Nothing enforces a minimum locally, but stay close to what a release is built with.
- **[Task](https://taskfile.dev/)**, the task runner. `go install github.com/go-task/task/v3/cmd/task@latest`.
- **The Wails v3 CLI.** Install it at the same version as the `wails/v3` dependency in `go.mod`, not `@latest`. The CLI and the runtime have to agree, and a mismatched pair fails in ways that look like your code is broken:

  ```powershell
  go install github.com/wailsapp/wails/v3/cmd/wails3@<version from go.mod>
  ```

- **NSIS**, only if you want to build the installer with `task package`.

## Getting a build

```powershell
git clone https://github.com/its-haze/valorant-rpc.git
cd valorant-rpc
go mod download
task build
```

Run `task --list` for everything available. The ones you will actually use are `task dev` for a hot-reloading dev server, `task build` for a production binary, and `task package` for the NSIS installer. Both build tasks print where they wrote the output.

`task build` accepts `VERSION=x.y.z`, which is injected into `internal/version` at link time. Leave it off for local work: an un-injected build reports itself as a dev build and disables the self-update flow, which is what you want while developing.

Namespaced tasks live under `build/Taskfile.yml` and `build/windows/Taskfile.yml`. Some of them (the server, Docker and iOS targets) are leftovers from the Wails project template and do nothing useful for a Windows-only desktop app.

## Tests

```powershell
go test ./...
```

The frontend has its own suite:

```powershell
cd frontend; npm test
```

Also run `gofmt -l .` before opening a pull request. It should print nothing.

Some tests in `internal/discord` hit the live network, because a presence image that stops resolving fails silently: Discord just shows no picture. They check every agent, map and tier URL against valorant-api.com. Pass `-short` to skip them if you are offline.

## Finding your way around

`cmd/` holds the entry points. The GUI application is the real one; there is also a headless daemon that runs the same presence engine with no window attached, which is the easier thing to debug against when the problem is on the Riot Client or Discord side rather than in the UI. Each `cmd/` subdirectory has a short package comment saying which is which.

`internal/` is one package per concern, and the names are literal: the Riot Client connection, presence decoding, Discord presence building, the config tree, updates, and so on. `internal/daemon` is where they are wired together, so start there when you want to see how a session state change becomes a presence update.

## A few conventions

The app has no command-line flags. Every setting lives in the config tree and is edited in the GUI. If you are adding an option, it needs a home on one of the settings screens.

Discord presence is built per context: a context maps to a builder, and each builder has a matching user-editable template with a default. Adding a context means touching all three, and the existing ones show the pattern.

The vocabulary is written down in [`CONTEXT.md`](CONTEXT.md), and it is worth a skim before naming anything. This app and its sibling `league-rpc` deliberately do not share terms, so League words arriving in Valorant code is a real and easy mistake.

Architecture decisions live in [`docs/adr/`](docs/adr/). If you are changing the presence heartbeat, when the Discord connection is allowed to open, where presence art comes from, or anything that talks to Riot, read the relevant one first. It probably explains why the obvious approach was rejected.

## Releases

Tagging a version triggers the release workflow, which builds the installer and the update binary, then signs a checksum file with an ed25519 key held in a protected environment. The app verifies that signature against a public key compiled into itself before installing anything. [`docs/release-signing.md`](docs/release-signing.md) has the details, including how to verify a release by hand.

A tag with a hyphen in it, like `v0.1.0-rc1`, publishes as a prerelease. Installed clients only ever look at `/releases/latest`, which skips prereleases, so a hyphen tag is the safe way to exercise the pipeline without shipping anything to anyone.
