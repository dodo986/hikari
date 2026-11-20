# AGENTS.md — working on Hikari

Hikari is a Linux game launcher: one Go daemon (`hikarid`) owns the library, the
store engines, the launch pipeline and the mod backend; a React/TypeScript UI in
a Wails window talks to that daemon over a loopback websocket, and a CLI
(`hikari`) talks to it over a unix socket. This file is the short version of how
the code is laid out, what must not break, and how to build and test it.

## Ground rules for an agent working here

- **The daemon is the only writer.** The CLI, the window and the Vite dev page
  are clients. They call RPCs; they never edit `state.json`, `settings.toml`,
  engine config or the game prefix directly. Any new mutation belongs behind an
  RPC handler in `cmd/hikarid/`.
- **Keep a change small and local.** Fix the bug where it lives; do not
  restructure a package to make a fix fit.
- **Every bug fix gets a regression test** in the same package (`*_test.go`),
  one that fails on the old code and passes on the new one. Tests must run
  offline.
- **Never edit `vendor/` or `internal/mods/backend/`.** The first is the
  vendored module set; the second is the Amethyst mod backend vendored verbatim
  (Python, `upstream.commit` records the revision). Changes there are lost on
  the next vendor run.
- **Never run the install/update scripts** (`scripts/install-*.sh`,
  `scripts/hikari-update`, `packaging/AppRun`) as a test. They write to the
  user's home and replace binaries. Read them instead.
- **Do not commit build output** (`dist/`, `hikari`, `hikarid`,
  `amethyst_filegraph.abi3.so`, `frontend/host/dist/`).
- Do not add a dependency without vendor support: CI builds with the vendored
  tree (`go build ./...`, `GOPROXY` unset but the module is vendored, so a new
  import needs `go mod vendor`).

## Layout

| Path | What lives there |
| --- | --- |
| `cmd/hikarid/` | the daemon: RPC registration, sockets, websocket, jobs, secrets, update |
| `cmd/hikari/` | the CLI client (one verb per daemon capability, `--json` everywhere) |
| `internal/api/` | JSON-RPC core: server, unix transport, websocket transport, client, event bus |
| `internal/config/` | paths, `settings.toml`, `state.json`, per-game settings |
| `internal/library/` | the merged library service and the `Provider` interface |
| `internal/providers/` | one package per source: `epic` (legendary), `gog` (gogdl), `amazon` (nile), `steam`, `heroic` (read-only import), `local`, `demo` |
| `internal/engine/` | engine binaries (legendary/gogdl/nile/umu/wine) and the process runner |
| `internal/launch/` | launch plan, env, prefix, wrapper, stop, per-game effective settings |
| `internal/winemgr/`, `internal/umu/`, `internal/dxvk/`, `internal/opti/`, `internal/runtimes/` | Wine/Proton, umu ids, DXVK/VKD3D, OptiScaler, runtimes |
| `internal/mods/` | Go side of the mod backend (bridge to Python, state, stop notes) |
| `internal/art/`, `internal/artwork/` | artwork cache + SteamGridDB lookup |
| `internal/secrets/`, `internal/logging/`, `internal/update/`, `internal/archive/`, `internal/atomicfile/`, `internal/watcher/`, `internal/browserlogin/` | supporting services |
| `frontend/web/` | React UI (Vite, vitest, Playwright) |
| `frontend/host/` | the Wails window — **separate Go module** (`hikari/host`) |
| `packaging/` | desktop entries, systemd user unit, PKGBUILD, AppRun |
| `scripts/` | build/release/install scripts |
| `ai/` | design docs for the (not yet built) in-app agent; not the daemon |

## Build and test

Offline, from the repo root, with the vendored tree:

```sh
go build ./...
go vet ./...
GOPROXY=off go test ./...          # all packages, no network
GOPROXY=off go test -race ./...    # slower; do this before a release
gofmt -l $(git ls-files '*.go' | grep -v '^vendor/')   # CI fails on any output
```

Window (needs Node, GTK3 and WebKitGTK 4.1 dev packages; builds `hikari-ui`):

```sh
frontend/scripts/build-host.sh                 # tsc + vite + link the host
cd frontend/web && npm ci && npm test          # vitest unit tests
frontend/scripts/dev-web.sh                     # Vite against a demo daemon
frontend/scripts/dev-native.sh                  # demo daemon + window
```

Release artifacts (`dist/`):

```sh
scripts/release.sh 0.9.0 x86_64                 # tarball + .sha256
scripts/build-appimage.sh 0.9.0 x86_64          # AppImage + .sha256 (needs appimagetool)
APPIMAGETOOL=/path/to/appimagetool scripts/build-appimage.sh 0.9.0
```

`build-appimage.sh` needs `appimagetool` on `PATH` (or `APPIMAGETOOL`), `cargo`
(for the filegraph extension via `scripts/build-backend.sh`), `npm`, and the
GTK/WebKit dev packages. It refuses to package a build whose `hikarid --version`
does not match the version argument.

## Invariants that are easy to break

- **Socket claim order.** `serve()` in `cmd/hikarid/main.go` binds the unix
  socket *first* (`api.Server.Listen`), then publishes the websocket endpoint
  (`startWS`). A second daemon must fail at the bind before it can touch
  `$XDG_RUNTIME_DIR/hikari/ws.json`; otherwise it strands every window by
  pointing the endpoint file at a port nobody serves. Keep `Listen`/`Serve`
  separate and keep `removeEndpoint` checking the PID in the file.
- **One token per daemon run.** The websocket token is regenerated on every
  start and written with mode `0600`; anything that reads artwork over the same
  listener is behind the same token (`WSConfig.Assets`).
- **Loopback only.** The websocket binds `127.0.0.1:0`; `hostAllowed` blocks DNS
  rebinding and `Origin` is checked. Do not widen either without a reason.
- **Paths and modes.** User data under `$XDG_DATA_HOME/hikari`, config under
  `$XDG_CONFIG_HOME/hikari`, endpoint under `$XDG_RUNTIME_DIR/hikari/ws.json`,
  socket at `$XDG_RUNTIME_DIR/hikari.sock`. Secrets go to the system keyring
  first, then a `0600` file (`internal/secrets`).
- **Writes are atomic.** Use `internal/atomicfile` for anything a crash could
  truncate (`state.json`, `settings.toml`, the endpoint, wine manifests).
- **State reads.** `StateStore.Entry` hands out a deep copy of the entry;
  mutate the store through `UpdateEntry`. A previously read entry never
  reflects later writes.
- **Read-only imports.** The Heroic and Steam providers read other launchers'
  trees and never write them.

## Adding an RPC end to end

1. Register the method in the matching `cmd/hikarid/*.go`
   (`srv.Register("ns.verb", handler)`) — namespaces today: `daemon`, `library`,
   `game`, `settings`, `auth`, `launch`, `logs`, `update`, `local`, `artwork`,
   `secrets`, `opti`, `mods`, `wine`, `engine`.
2. Add the CLI verb in `cmd/hikari/main.go` (keep `--json` working).
3. Add the client method and types in `frontend/web/src/daemon/api.ts` and
   `types.ts`.
4. Test the handler at the daemon level (`cmd/hikarid/*_test.go` uses demo mode
   or fakes); no network in tests.

## Existing test doubles (use them, do not build a new harness)

- `hikarid --demo`: fake in-memory providers, simulated installs/launches.
- `internal/providers/demo`, `internal/opti/fake.go`, `internal/mods/fake.go`.
- `internal/engine` tests run real short processes; `internal/launch` tests use
  `Plan.Prepare`/`Argv` with `/bin/sh`.
- Provider tests read fixtures from `internal/providers/*/testdata`.

## Gotchas

- Go version is pinned by `go.mod` (1.27.x) and CI runs `gofmt`, `go vet`,
  `go test`, `go build`. Run all four before claiming a change is done.
- `frontend/host` is a separate module: `go build ./...` from the root does not
  compile it. Use `frontend/scripts/build-host.sh`.
- Tests must not touch the real `$HOME`: point `XDG_CONFIG_HOME`,
  `XDG_DATA_HOME`, `XDG_RUNTIME_DIR` and (when relevant)
  `HIKARI_HEROIC_CONFIG` at `t.TempDir()` first, as `cmd/hikarid/serve_test.go`
  and `daemon_test.go` do.
- The daemon log is `$XDG_DATA_HOME/hikari/hikarid.log`; `hikari logs` reads it
  even with the daemon stopped.
- `go build` output names: `hikarid`, `hikari`, `hikari-ui` (window),
  `amethyst_filegraph.abi3.so` (mod filegraph extension).
