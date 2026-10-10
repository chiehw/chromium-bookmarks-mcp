# Contributing

Thanks for hacking on chromium-bookmarks-mcp. This guide covers local setup, running the three-process stack against an unpacked extension, the test suite, and where each process writes its logs.

## Prerequisites

- [Go](https://go.dev/) 1.27+ — the MCP stdio proxy and native host are a single Go binary.
- [Node.js](https://nodejs.org/) 22+ — only needed to build and test the browser extension.
- A Chromium-based browser (Chrome, Brave, Edge, Arc, or Chromium) for end-to-end testing.

## Setup

```bash
go test ./...
make build
npm install   # extension + shared types
```

Layout:

```
chromium-bookmarks-mcp/
├── cmd/chromium-bookmarks-mcp/   # CLI entry
├── internal/                     # native protocol, host, proxy, register
├── packages/shared/              # Shared wire types + constants (extension)
└── apps/extension/               # Browser extension (WXT + TypeScript)
```

## The three-process architecture

At runtime the system is three independent processes with two independent spawners. Understanding who spawns what is the key to debugging it:

```
AI client (Claude Code / Desktop / Cursor ...)
    | spawns, MCP stdio JSON-RPC
    v
Process C — MCP stdio proxy   (cmd/chromium-bookmarks-mcp, default command)
    | HTTP to 127.0.0.1:19420
    v
Process A — native host + HTTP server   (same binary, argv0 native-host)
    | Chrome Native Messaging (stdin/stdout, 4-byte framed)
    v
Browser extension service worker   (apps/extension/entrypoints/background/)
    | chrome.bookmarks API
    v
Browser bookmarks
```

Two separate things start processes:

- The **browser** spawns Process A. When the extension opens a native-messaging port, Chrome runs the registered host binary (`~/.cache/chromium-bookmarks-mcp/native-host`, installed by `register`). The host is 1:1 with that port — when the extension disconnects, its stdin closes and the host exits. You cannot meaningfully run Process A by hand; let the browser launch it.
- The **AI client** spawns Process C. Process C also runs `Ensure()` on startup (idempotent — see below), then bridges MCP stdio to the host's HTTP server.

## Running the full stack locally

1. **Build / load the unpacked extension.** Run the extension in dev mode (hot reload):

   ```bash
   npm run dev -w @chromium-bookmarks-mcp/extension
   ```

   This opens a browser with the unpacked extension loaded from `.output/`. Copy its ID from `chrome://extensions`.

2. **Reconcile the dev extension ID with the native host.** The native-host manifest pins `allowed_origins` to a single extension ID, defaulting to the published Web Store ID. Your unpacked build has a different ID, so the host will refuse the connection until you reconcile them. Pick one (see `.env.example` for both):

   - **Register the dev ID** (simplest):

     ```bash
     make build
     ./bin/chromium-bookmarks-mcp register <your-dev-extension-id>
     ```

     This copies the current binary into the cache dir as the native host and rewrites `allowed_origins` (and the Windows registry key, on Windows) to your dev ID.

   - **Or pin `manifest.key`** in `apps/extension/wxt.config.ts` so the unpacked build derives the *same* ID as the published one — no re-registration needed. Keep the key out of git.

3. **Start the stdio proxy.** In normal use your AI client spawns this, but you can run it by hand to exercise the bridge:

   ```bash
   ./bin/chromium-bookmarks-mcp
   ```

   Or wire it into your client, e.g. Claude Code:

   ```bash
   claude mcp add bookmarks -- /absolute/path/to/chromium-bookmarks-mcp
   ```

4. **Activate the connection.** Open the browser, click the extension icon. A green dot / **Connected** means the extension reached Process A. The extension only attempts a connection on browser launch, popup open, or **Refresh status** — there is no background polling, so click Refresh after restarting the host.

5. **Verify.** Ask the agent to call the `ping` tool, or run `./bin/chromium-bookmarks-mcp doctor` to diagnose registration and the localhost port.

Do not use `go run` for `register` or for the proxy you will actually connect to: Chrome launches a copy of the current executable, and `go run` compiles to a temp file that disappears.

### About registration

`Ensure()` runs at proxy startup (Process C), **not** at install time. It is idempotent: it only writes a manifest when one is missing or its `path` has gone stale, so it is safe to run on every startup. Use the explicit `register` / `unregister` subcommands when changing extension IDs or clearing manifests.

Chrome Native Messaging hosts cannot take extra argv, so `register` copies this binary to `~/.cache/chromium-bookmarks-mcp/native-host` (`.exe` on Windows) and points every browser manifest at that copy. The copy detects native-host mode from its own filename.

## Release binaries

Push a version tag to run the same flow as pika: cross-compile six platform binaries, zip the Chrome extension, attach `SHA256SUMS`, and publish a GitHub Release.

```bash
git tag v0.3.0
git push origin v0.3.0
```

Locally: `make build` (current OS) or `make release` (`dist/` for linux/darwin/windows × amd64/arm64 plus `chromium-bookmarks-mcp-*-chrome.zip`).

## Tests

```bash
go test ./...                  # server: protocol, host, proxy, browsers, register
npm test -w @chromium-bookmarks-mcp/extension   # handlers against an in-memory chrome.bookmarks fake
```

To run one package:

```bash
go test ./internal/protocol
npm test -w @chromium-bookmarks-mcp/extension -- test/ssrf.test.ts
```

## Where each process logs

Each process logs to a different place — knowing which is essential, because the most fragile process (the native host) logs where it is hardest to see.

| Process | Stream | Where to read it |
|---------|--------|------------------|
| **Process A — native host** | `stderr` | Spawned by the browser, so its stderr is **captured and largely discarded by Chrome**. Set `BOOKMARKS_MCP_LOG` to a writable file path to capture host logs. Lines include buffer-overflow resets, "Extension disconnected", and "HTTP server listening on 127.0.0.1:19420". |
| **Process C — stdio proxy** | `stderr` | `stdout` is reserved for MCP JSON-RPC, so all diagnostics go to stderr — surfaced in your AI client's MCP server log, or directly in your terminal when you run the proxy by hand. Lines include "MCP stdio proxy started." and "auto-register failed: ...". |
| **Browser extension service worker** | `console.log` / `warn` / `error`, prefixed `[BM-MCP]` | `chrome://extensions` -> the extension's **service worker** link -> DevTools Console. |

When the chain is broken, walk it back-to-front: extension console first (did the SW connect?), then assume the native host (does `doctor` find the manifest? is port 19420 free?), then the proxy log.

## Conventions

- Go for the MCP server and native host; TypeScript for the extension. Match the existing code style; no emoji in code or commit messages.
- Keep the wire protocol types in `packages/shared` as the source of truth for the extension; `internal/types` must stay in sync.
- Do not commit secrets: the unpacked-dev `manifest.key`, any `.env`, or personal data. `.env.example` is the committed reference; `.env` is gitignored.
