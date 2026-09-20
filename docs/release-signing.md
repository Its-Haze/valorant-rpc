# Release signing

This repository has its own ed25519 keypair. It is not league-rpc's, and the
two must never be shared: a compromise of one signing key should not be able to
push an update to the other app's users.

Every tagged release publishes four assets. The NSIS installer
(`valorant-rpc-<version>-setup.exe`) is the only one a person is meant to
download and run. The raw binary (`internal/updates.ReleaseAsset`) is what the
in-app updater swaps in, named `.bin` so it doesn't look double-clickable
sitting next to the installer. `SHA256SUMS` lists the sha256 of both, and
`SHA256SUMS.sig` holds a detached ed25519 signature over each of those digests.
`internal/updates` checks the digest and the signature before it writes
anything to disk, and refuses a release that carries no signature for its
binary. That refusal is the point: the digest travels in the same release as
the binary, so on its own it proves the download was not corrupted and nothing
about who published it. A release missing `SHA256SUMS.sig`, or whose sidecar
has no line naming the binary, fails the check and the app stays on the version
it has.

The practical consequence is that a broken `sign` job blocks a release rather
than degrading it. That is the intended trade.

## How the workflow is split

`.github/workflows/release.yml` runs on any `v*` tag push, in three jobs:

1. `build` packages the binary and the installer on a Windows runner.
2. `sign` runs `cmd/sign-release` on Linux, and is the only job that declares
   `environment: release-signing`. The private key is an *environment* secret,
   so it is not visible to `build` or `release` at all. That isolation is the
   whole point of the extra job.
3. `release` assembles the notes from `.github/release-notes/<tag>.md` and
   publishes everything with `gh release create`.

A tag containing a hyphen (`v0.0.1-test`, `v1.2.0-rc1`) publishes as a GitHub
prerelease. The app asks for `/releases/latest`, which skips prereleases, so a
hyphen tag is invisible to installed clients. That is what makes a dry run
safe. `internal/updates/releasepipeline_test.go` fails if either half of that
rule disappears from the workflow.

## The key

Fingerprint of the embedded public key, the sha256 of its raw 32 bytes:

```
cd2e794abb289df796cfb6087af422c4f27d4f204011a75adbc11b56cfb57303
```

Reproduce it from a checkout:

```
openssl pkey -pubin -in internal/updates/keys/update-public.pem -outform DER \
  | tail -c 32 | sha256sum
```

`internal/updates/releasepipeline_test.go` compares that fingerprint against
the key compiled into the binary, so rotating the key without updating this
file fails the test. Nothing runs it automatically yet: this repo has no CI
workflow, only the release one. Run `go test ./internal/updates/` after a
rotation until ticket 17 lands.

## Generating the keypair

```
openssl genpkey -algorithm ed25519 -out update-private.pem
openssl pkey -in update-private.pem -pubout -out update-public.pem
```

Commit the public half to `internal/updates/keys/update-public.pem` and update
the fingerprint above.

For the private half, go to Settings > Environments on
`Its-Haze/valorant-rpc`, create the `release-signing` environment, and add the
full contents of `update-private.pem` as an environment secret named
`UPDATE_SIGNING_KEY`. Not a repository secret: a repository secret is readable
by every job in the workflow, which throws away the isolation described above.
Then delete the local private key file. It never gets committed, and there is
no copy of it anywhere else.

## Verifying a release by hand

```
sha256sum -c SHA256SUMS
```

and for the signature, take the hex digest for the asset from `SHA256SUMS`,
take the hex signature for the same filename from `SHA256SUMS.sig`, and check
the second against the first with the public key. The signature covers the
32-byte digest, not the file, so a verifier has to hex-decode both sides first.

## Rotating the key

1. Generate a new keypair as above.
2. Ship a build that accepts the old key and the new one, so clients still on
   the old key can verify the release that moves them across. Nothing supports
   this today: `BuildConfig` passes one `PublicKey`, and Wails parses exactly
   one. Writing dual-key support is the first task of any rotation.
3. Point `UPDATE_SIGNING_KEY` at the new private key and sign with it.
4. After enough time that no client is plausibly still on the old build, ship a
   build that trusts only the new key.

Steps 2 and 4 are unavoidable. Swapping the key in one release strands every
user who hasn't updated yet, because their installed build rejects the very
release that would fix them.
