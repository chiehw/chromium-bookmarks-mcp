<p align="center">
  <img src="apps/extension/public/icon.svg" width="128" height="128" alt="Chromium Bookmarks MCP">
</p>

<h1 align="center">Chromium Bookmarks MCP</h1>

<p align="center">
  <strong>Give AI agents real-time read/write access to your browser bookmarks via <a href="https://modelcontextprotocol.io/">Model Context Protocol</a>.</strong>
</p>

<p align="center">
  <a href="https://chromewebstore.google.com/detail/chromium-bookmarks-mcp/ipcgfbbojaphhaoanjalmjmooeobjein"><img src="https://img.shields.io/badge/Chrome_Web_Store-published-4285F4?logo=googlechrome&logoColor=white" alt="Chrome Web Store"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <a href="https://modelcontextprotocol.io/"><img src="https://img.shields.io/badge/MCP-compatible-blue" alt="MCP Compatible"></a>
</p>

<p align="center">
  Works with <strong>Claude Code</strong> &middot; <strong>Claude Desktop</strong> &middot; <strong>Cursor</strong> &middot; <strong>Windsurf</strong> &middot; <strong>VS Code Copilot</strong> &middot; and any MCP client<br>
  Supports <strong>Chrome</strong> &middot; <strong>Brave</strong> &middot; <strong>Edge</strong> &middot; <strong>Arc</strong> &middot; and any Chromium-based browser
</p>

---

## Prerequisites

- **[Go](https://go.dev) 1.27+** — the MCP server and native host are a single Go binary.
- A Chromium-based browser (Chrome, Brave, Edge, Arc, or Chromium itself).

## Quick Start

### 1. Install the Chrome extension

<a href="https://chromewebstore.google.com/detail/chromium-bookmarks-mcp/ipcgfbbojaphhaoanjalmjmooeobjein">
  <img src="https://img.shields.io/badge/Install_from-Chrome_Web_Store-4285F4?style=for-the-badge&logo=googlechrome&logoColor=white" alt="Install from Chrome Web Store" height="48">
</a>

Or download `chromium-bookmarks-mcp-*-chrome.zip` from [GitHub Releases](https://github.com/chiehw/chromium-bookmarks-mcp/releases), unzip it, then Chrome → `chrome://extensions` → Developer mode → **Load unpacked**.

### 2. Install the MCP server and add it to your AI client

Download a binary from [GitHub Releases](https://github.com/chiehw/chromium-bookmarks-mcp/releases) (linux / darwin / windows, amd64 and arm64), or:

```bash
go install github.com/chiehw/chromium-bookmarks-mcp/cmd/chromium-bookmarks-mcp@latest
```

**Claude Code:**
```bash
claude mcp add bookmarks -- chromium-bookmarks-mcp
```

**Claude Desktop** (`claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "bookmarks": {
      "command": "chromium-bookmarks-mcp"
    }
  }
}
```

> Registration runs when your MCP client first starts the stdio proxy (not at install time) — it copies the binary into `~/.cache/chromium-bookmarks-mcp/` as the native host and registers it for every detected Chromium browser, including writing the required HKCU registry keys on Windows. The step is idempotent and re-runs on every startup, so no manual `register` is needed.

**Cursor / Windsurf / other MCP clients** — use the same command: `chromium-bookmarks-mcp`

### 3. Activate the connection

Open your browser and click the extension icon. **This activates the connection** between the extension and the MCP server — you should see a green dot and **Connected** status. If you don't, click the **Refresh status** button in the popup.

Then ask your AI agent:
```
Use the ping tool to check if bookmarks MCP is connected
```

> **Note:** Your browser must be open for the MCP tools to work. The extension only attempts a connection on browser launch, popup open, or Refresh — there is no background polling. No data leaves your machine.

### 4. Back up before any cleanup

Bookmark deletes are **permanent — Chrome has no undo for them**. Before running any destructive tool (`bookmark_batch_delete`, `bookmark_deduplicate`, `bookmark_merge_folders` with `delete_source`, or `bookmark_delete_folder`), take a full snapshot:

```
Use bookmark_export_html to export all my bookmarks, then save the HTML to a file
```

`bookmark_export_html` returns the complete bookmark tree as standard Netscape Bookmark HTML — the same format the browser's own "Export bookmarks" produces. If a cleanup goes wrong, `bookmark_import_html` restores that snapshot back into your browser. Keep the export until you have verified the result.

## Features

**19 MCP tools** for complete bookmark management:

| Category | Tools |
|----------|-------|
| **Read** | `ping`, `bookmark_get_tree`, `bookmark_list`, `bookmark_search`, `bookmark_get`, `bookmark_count`, `bookmark_find_duplicates` |
| **Write** | `bookmark_create`, `bookmark_update`, `bookmark_move`, `bookmark_delete`, `bookmark_delete_folder` |
| **Batch** | `bookmark_batch_move`, `bookmark_merge_folders`, `bookmark_deduplicate`, `bookmark_batch_delete` |
| **Export/Import** | `bookmark_export_html`, `bookmark_import_html` |
| **Analysis** | `bookmark_check_dead_links` |

**Key capabilities:**
- Real-time operations while browser is open (no file manipulation)
- Folder path resolution (`"Bookmarks Bar > Tech > AI"`)
- Auto-create nested folders with `create_parents`
- Smart deduplication by URL
- Folder merge with optional dedup
- Dead link detection with HEAD/GET fallback
- Safety gates on destructive operations (`dry_run: true` to preview, `confirm: true` to execute)
- HTML export as a one-call backup before cleanup (`bookmark_export_html` / `bookmark_import_html`)

## Architecture

Three-process design with HTTP bridge:

```
Claude Code / AI Agent
    | (MCP stdio JSON-RPC)
    v
MCP Stdio Proxy (Process C)        <- spawned by AI client
    | (HTTP to localhost:19420)
    v
Native Host + HTTP Server (Process A) <- spawned by browser extension
    | (Chrome Native Messaging)
    v
Browser Extension (Service Worker)
    | (chrome.bookmarks API)
    v
Browser Bookmarks
```

## Tool Reference

### Read Tools

**`ping`** — Check if the extension is connected and responsive.

**`bookmark_get_tree`** — Get the bookmark tree structure.
- `folder_id` (optional) — Get subtree of a specific folder
- `depth` (optional, default: 2) — Max depth to return. Use `0` for unlimited.

**`bookmark_list`** — List bookmarks in a folder (non-recursive).
- `folder_id` (optional) — Folder to list. Default: root.
- `limit` / `offset` — Pagination.

**`bookmark_search`** — Full-text search across titles and URLs.
- `query` — Search text
- `folder_id` or `folder_path` — Scope search (e.g., `"Bookmarks Bar > Tech"`)
- `limit` (optional, default: 50)

**`bookmark_get`** — Get a single bookmark/folder by ID with folder path.

**`bookmark_count`** — Count bookmarks and folders, optionally scoped.

**`bookmark_find_duplicates`** — Find bookmarks with duplicate URLs.

### Write Tools

**`bookmark_create`** — Create a bookmark or folder.
- `title` — Required
- `url` — Omit to create a folder
- `parent_id` or `parent_path` + `create_parents: true` — Auto-create nested folders

**`bookmark_update`** — Update title or URL.

**`bookmark_move`** — Move to a different parent folder.

**`bookmark_delete`** — Delete a single bookmark. Root folders protected.

**`bookmark_delete_folder`** — Delete a folder and all contents. Requires `confirm: true`.

### Batch Tools

**`bookmark_batch_move`** — Move multiple bookmarks at once.

**`bookmark_merge_folders`** — Merge source folder into target.
- `deduplicate` — Skip bookmarks that already exist in target (by URL)
- `delete_source` — Remove source folder after merge. When `true`, this is destructive: pass `dry_run: true` first to preview, then `confirm: true` to execute. No undo.

**`bookmark_deduplicate`** — Remove duplicate bookmarks (same URL). Destructive, no undo.
- `folder_id` or `scope: 'global'` — **Required.** Scopes which bookmarks are considered; omitting both is rejected so an unscoped call can't sweep the whole tree.
- `keep` — `'first'` or `'last'`
- `dry_run: true` — Preview the exact items that would be removed. Run `bookmark_find_duplicates` or a `dry_run` first, then `confirm: true` to execute.

**`bookmark_batch_delete`** — Delete multiple bookmarks by IDs. Destructive, no undo: `dry_run: true` to preview, `confirm: true` to execute.

> Back up first with `bookmark_export_html`; `bookmark_import_html` restores it. See [Quick Start step 4](#4-back-up-before-any-cleanup).

### Export/Import

**`bookmark_export_html`** — Export as Netscape Bookmark HTML format. Use this as your backup/safety net before any destructive cleanup.

**`bookmark_import_html`** — Import from Netscape Bookmark HTML. Restores a snapshot produced by `bookmark_export_html`.

### Analysis

**`bookmark_check_dead_links`** — Check URLs for broken links (HTTP HEAD with GET fallback).
- `limit` (default: 50) — Max bookmarks to check
- `timeout_ms` (default: 5000) — Request timeout

## CLI Commands

```bash
chromium-bookmarks-mcp              # Start MCP stdio proxy (default)
chromium-bookmarks-mcp register     # Register native host for detected browsers
chromium-bookmarks-mcp register ID  # Register with specific extension ID (local dev)
chromium-bookmarks-mcp unregister   # Remove native host registration
chromium-bookmarks-mcp doctor       # Diagnose connection issues
```

> `register` is **automatic** on first proxy startup. Run it manually only when changing extension IDs (local dev), or after manually deleting native-host manifests.

Push a `v*` tag to publish a GitHub Release: linux/darwin/windows (amd64 + arm64) MCP binaries plus the Chrome extension zip.

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Extension shows "Disconnected" | Click **Refresh status** in the popup. If still disconnected, run `chromium-bookmarks-mcp doctor` to diagnose. |
| `register` doesn't detect browser | Pass the extension ID manually: `chromium-bookmarks-mcp register <ID>` |
| MCP tools timeout | Ensure your browser is open and the extension is enabled |
| Port 19420 conflict | Another instance may be running. Check with `lsof -i :19420` |
| Tools work but return empty | Your browser may need bookmarks — try creating one manually first |

For deeper diagnostics, run:
```bash
chromium-bookmarks-mcp doctor
```

## Supported Browsers

| Browser | macOS | Linux | Windows |
|---------|-------|-------|---------|
| Chrome | Yes | Yes | Yes |
| Brave | Yes | Yes | Yes |
| Edge | Yes | Yes | Yes |
| Arc | Yes | — | — |
| Chromium | Yes | Yes | — |

On Windows, `register` writes both the manifest JSON and the corresponding `HKCU\Software\<vendor>\<browser>\NativeMessagingHosts` registry key, which is how Chromium discovers native messaging hosts on Windows.

## Development

### Prerequisites

- [Go](https://go.dev/) 1.27+
- [Node.js](https://nodejs.org/) 22+ (extension only)
- A Chromium browser

### Project Structure

```
chromium-bookmarks-mcp/
├── cmd/chromium-bookmarks-mcp/  # CLI: serve / native-host / register / doctor
├── scripts/build-release.py     # Cross-compile release binaries
├── Makefile
├── internal/
│   ├── protocol/                # Chrome native messaging framing
│   ├── host/                    # Process A: native messaging + HTTP
│   ├── proxy/                   # Process C: MCP stdio proxy
│   ├── browsers/                # Browser detection
│   └── register/                # Native host registration
├── packages/shared/             # Shared types and constants (extension)
└── apps/extension/              # Browser extension (WXT + TypeScript)
    └── entrypoints/
        ├── background/          # Service worker + tool handlers
        ├── offscreen/           # Keepalive for MV3
        └── popup/               # Connection status UI
```

### Commands

```bash
go test ./...                  # Server unit tests
make build                     # bin/chromium-bookmarks-mcp
make release                   # dist/ binaries for all platforms
npm install                    # Extension dependencies
npm test -w @chromium-bookmarks-mcp/extension
npm run dev -w @chromium-bookmarks-mcp/extension     # Dev mode (hot reload)
npm run build -w @chromium-bookmarks-mcp/extension   # Production build
```

### Working with a local unpacked extension

The native-host manifest's `allowed_origins` defaults to the published Chrome Web Store extension ID. When developing against an unpacked extension you have two options:

1. **Pin the dev ID via `manifest.key`** — copy the public key from the Web Store Developer Dashboard ("More info" → "Public key") into `apps/extension/wxt.config.ts` as `manifest.key`. Chrome derives the unpacked extension's ID from this key, matching the published one. Do **not** commit the key — keep it in a local `.env`-style override or a private branch.
2. **Pass the dev ID to register** — load your unpacked extension, copy the ID from `chrome://extensions`, then run:
   ```bash
   chromium-bookmarks-mcp register <your-dev-extension-id>
   ```
   This rewrites `allowed_origins` to your dev ID for local testing.

## Security & Privacy

- Only 3 permissions: `bookmarks`, `nativeMessaging`, `offscreen`
- All communication is localhost only (`127.0.0.1`)
- No analytics, no telemetry, no external network calls
- No data leaves your machine
- Root folder deletion protected
- Destructive batch operations gated by `dry_run` preview and explicit `confirm: true`
- Back up first with `bookmark_export_html`; deletes are permanent and have no undo
- Full [Privacy Policy](privacy-policy.md)

## License

[MIT](LICENSE)
