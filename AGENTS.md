# syncthing fork

Personal fork of `syncthing/syncthing`. Default branch is `main`.
Upstream remote is the parent; this checkout tracks the fork (`joshuarli/syncthing`).

## Toolchain

Go 1.27.3. The `go` directive in `go.mod` is the single source of truth.
The prerelease workflow installs it via `go-version-file: go.mod`, so bumping
the toolchain means editing `go.mod` only. Verify with `go build ./cmd/syncthing`.

## Prereleases

Prereleases follow the repomod native binary contract (see `~/d/repomod/docs/tagging.md`).
The workflow is `.github/workflows/prerelease.yml`. Catalog tags for this repo
should be `tool`, `fork`, `go`.

Contract properties, all load-bearing:

- Manual `workflow_dispatch` only, with a preflight job that refuses dispatches
  off the default branch, and the release job rechecks ref and tag absence.
- Build matrix is exactly the portable set: `aarch64-apple-darwin`,
  `x86_64-unknown-linux-musl`, `aarch64-unknown-linux-musl`, plus one
  Android extra: `android-arm64` (`GOOS=android GOARCH=arm64`, arm64 only for
  the Pixel 7a's Tensor G2; no x86 emulator or 32-bit builds). No glibc targets.
  The Android binary is downloaded to `android-dist/`, never `dist/`, so the
  Linux static smoke scan (exactly one x86_64 and one aarch64 ELF, no
  `PT_INTERP`/`DT_NEEDED`) is unaffected. Android PIE executables always carry
  the `/system/bin/linker64` interpreter; that is the correct on-device form,
  not a violation.
- Linux binaries are built with `CGO_ENABLED=0` (pure-Go `modernc.org/sqlite`,
  no cgo), so they are fully static. The release job rejects any Linux ELF
  with `PT_INTERP`/`DT_NEEDED` before publication.
- All builds use `-tags noupgrade`: the fork must never self-upgrade against
  upstream `upgrades.syncthing.net` releases. `Version` is stamped with the
  fork version `v<base>-fork-<short-sha>` (`-X .../lib/build.Version=$FORK_VERSION`),
  where `<base>` tracks `version-name` in `android/gradle/libs.versions.toml`
  and `<short-sha>` is the first 12 hex of the built commit. The release tag
  stays `release-<sha>`; only the stamped `Version` must satisfy
  `lib/build`'s `^v\d+\.\d+\.\d+` check.
- The Android build additionally passes `-checklinkname=0`. Reason:
  `github.com/wlynxg/anet` v0.0.5 (used only by
  `lib/netutil/interfaces_android.go`) linknames `net.zoneCache`, which Go
  1.24+ rejects because the std definition has no push linkname. Desktop
  builds are unaffected (anet delegates to std `net` off Android). Revisit if
  anet is updated or the toolchain moves past 1.27.3.
- `go generate ./lib/api/auto/` must run before `go build`; `gui.files.go`
  is gitignored and never committed.
- Per-platform `syncthing-<target>.zip` archives plus a verified `SHA256SUMS`.
- Tag `release-${{ github.sha }}` is created at the triggering commit through
  the GitHub API, and `gh release create` passes `--verify-tag --prerelease`.
- Only `actions/checkout`, `actions/setup-go`, `actions/upload-artifact`,
  `actions/download-artifact` at their `~/d/repomod/actions.toml` pins.

Cut a prerelease by dispatching the workflow on `main`. Never retag; the
release job aborts if the tag exists.

Only `cmd/syncthing` is built and shipped. Server, infra, and dev binaries
are excluded (see below).

## Android wrapper

`android/` vendors a trimmed Syncthing-Fork wrapper (MPL-2.0, see
`android/LICENSE`; upstream `researchxxl/syncthing-android`, reference clone
in `.work/tmp/syncthing-android`). Only `arm64-v8a` is built (Pixel 7a);
store listings, wiki, Windows helpers, and F-Droid scripts were dropped.
The application ID stays `com.github.catfriend1.syncthingfork`.

- The native core compiles from this repo root: `android/syncthing/build-syncthing.py`
  sets `syncthing_dir` to the monorepo root, so core and wrapper can never skew.
  It passes `-no-upgrade` and `EXTRA_LDFLAGS=-checklinkname=0` (same anet/Go reason
  as the raw `android-arm64` prerelease binary).
- Version plumbing: the native core version is `v<base>-fork-<short-sha>`
  derived from `version-name` in `android/gradle/libs.versions.toml` plus the
  built commit, so the app-version match stays enforced. Bump `version-name` /
  `version-code` when rebasing onto a newer upstream; desktop prereleases read
  the same base.
- Toolchain: JDK 21 (Temurin), Android cmdline-tools + NDK 30 from the Gradle
  version catalog, Go from root `go.mod`. Bumping Go means editing `go.mod`
  AND `go_version` in `android/gradle/libs.versions.toml`.
- `.github/workflows/android.yml` builds `assembleRelease`, signs with a fresh
  per-run debug keystore (no secrets), and attaches
  `syncthing-android-arm64.apk` to the matching `release-<sha>` release.
  Dispatch order: `prerelease` first, then `android`, on the same commit.
  The filename stem keeps it outside fork-checked policy; `setup-java` is
  hand-pinned and `pinact lint --profile=correctness` must stay clean.

## Debloat

Goal: ship and run only the core sync engine. No relay/discovery
infrastructure, no phone-home, no self-upgrade against upstream releases.

### Cleanly excluded: separate binaries

These never enter a prerelease; they are simply not built. No code changes needed:

- `cmd/strelaysrv`, `cmd/stdiscosrv` (relay / discovery servers)
- `cmd/infra/strelaypoolsrv`, `cmd/infra/ursrv`, `cmd/infra/stcrashreceiver`,
  `cmd/infra/stupgrades` (pool, usage-reporting, crash, upgrade servers)
- `cmd/dev/*` (debug tools; keep only if actively used)

### Runtime posture, not code deletion

The relay/discovery/NAT client stack (`lib/relay`, `lib/discover`, `lib/stun`,
`lib/nat`, `lib/upnp`, `lib/pmp`, `lib/beacon`) is wired into
`lib/connections/service.go`, `lib/syncthing/syncthing.go`, `lib/api/api.go`,
and `lib/config` defaults. Deleting it means forking the connection stack;
that is out of scope. Disable it via configuration instead:

- `listenAddresses`: explicit `tcp://` and `quic://` addresses, no `dynamic+https://relays.syncthing.net/endpoint`
- `relaysEnabled: false`
- `globalAnnounceEnabled: false`, `globalAnnounceServers: []`
- `localAnnounceEnabled: false`
- `natEnabled: false`, `stunServers: []`
- `urAccepted: -1` (decline usage reporting; `urURL` untouched)
- `crashReportingEnabled: false`
- upgrades: prerelease builds already use `-tags noupgrade`, so shipped
  binaries cannot self-upgrade against upstream releases. For local builds run
  `go build -tags noupgrade ./cmd/syncthing`, or point `releasesURL` away
  from upstream.

### Explicitly out of scope

- Removing `lib/relay`, `lib/discover`, or related client packages.
- Removing the embedded web GUI (`-tags noassets` saves only ~2.3 MB and
  kills the desktop UI; the generated `gui.files.go` also collides with the
  `noassets` stub, so the build must delete it first).
- Chasing sub-megabyte dependencies. Measured linux/amd64 `go tool nm`
  breakdown (51.7 MB baseline, `noupgrade`): sqlite stack ~2.2 MB (no static
  alternative), cel-go/protovalidate ~1.8 MB (security hardening, keep),
  quic-go ~0.6 MB (`noquic` exists but guts the transport), leveldb migration
  ~0.3 MB (`internal/db/olddb`, one-time use), kong CLI ~0.2 MB, prometheus
  ~0.2 MB. aws-sdk-go, geoip, sentry, and the infra servers are not linked
  into the main binary at all. The 32 MB `fips140/drbg` entry is BSS: free
  on disk.
- Changing config defaults in `lib/config` (existing configs expand `default`
  keywords at runtime; changing defaults alters behavior for all users of this fork).
- Adding CI workflows besides `prerelease.yml` and `android.yml`. Fork policy
  only checks the `lint`, `prerelease`, and `test` stems; anything else is
  invisible to it, so keep it that way.

## Verification

- `go build ./cmd/syncthing` after toolchain or dependency changes.
- `pinact lint --profile=default --format=json --catalog ~/d/repomod/actions.toml -- .github/workflows/prerelease.yml`
  must return `[]`.
- Cross-compile check: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath
  -o /tmp/st-check ./cmd/syncthing` (needs `go generate ./lib/api/auto/` first);
  the result must be a statically linked ELF (`file` reports `statically linked`).
