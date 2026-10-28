# The window

A React and TypeScript UI in a Wails v2 host, served from the same daemon as
the Quickshell one. `../frontend/docs/` explains the design; this file is only
how to work on it.

```sh
frontend/scripts/dev-web.sh      # vite on 127.0.0.1:5173 against the demo daemon
frontend/scripts/build-host.sh   # typecheck, bundle, embed, link build/hikari-ui
frontend/scripts/dev-native.sh   # build both, run the daemon in demo mode, open the window
```

Layout:

| Path | What lives there |
|---|---|
| `web/src/daemon` | types, endpoint discovery, RPC client, one module per domain |
| `web/src/components` | primitives and shell pieces, each with a CSS module |
| `web/src/pages` | one directory or file per route |
| `web/src/theme` | palette application and tokens |
| `web/test`, `web/e2e` | vitest unit tests, Playwright smoke tests |
| `host` | the Go program: window, asset handler, endpoint injection, file pickers |
| `scripts` | dev, build, memory measurement |

Rules the code follows: no `any`, no default exports, strict TypeScript with
`noUncheckedIndexedAccess`, components under about 150 lines, CSS modules and
Ryoku tokens instead of literal colours, all strings through `t()`.

The window never reads secrets: the daemon writes an endpoint and a token into
`$XDG_RUNTIME_DIR/hikari/ws.json` with 0600, and the host injects it into the
page. A daemon older than the websocket API has no such file, and the offline
screen says to rebuild it rather than pretending nothing is running.
