# Container Rules Manager — Design Spec

## Overview

A Firefox/Zen Browser extension that provides a management UI for Multi-Account Containers (MAC) site-to-container rules. It does not enforce rules itself — MAC remains the enforcement engine. The extension communicates with MAC's IndexedDB (stored as SQLite on disk) through a Go native messaging host.

## Names

- **Package / directory:** `fmac-rules-manager`
- **Listing name:** Container Rules Manager
- **Gecko ID:** `fmac-rules-manager@sarcasticbird.com`
- **Native host name:** `fmac_rules_manager`

## Scope

### In scope
- CRUD for site-to-container rules (read/write MAC's IDB)
- Import/export rules as JSON
- Full-page options table editor
- Go native messaging host (thick — handles all profile/IDB/SQLite logic)
- MAC reload via `management` API after writes

### Out of scope
- Request interception / rule enforcement (MAC does this)
- Container CRUD (containers are read-only, from `containers.json`)
- Popup UI
- Automated/scheduled backups
- Native messaging host installer (manual install for now)
- Firefox Sync support

## Repo Structure

```
fmac-rules-manager/
├── src/                        # Extension (web-ext source)
│   ├── manifest.json
│   ├── options.html            # Full-page table editor
│   ├── options.js
│   ├── options.css
│   ├── background.js           # Native messaging relay + MAC management
│   └── icon.svg
├── host/                       # Go native messaging host
│   ├── main.go                 # Entry point, native messaging protocol
│   ├── profile.go              # Profile/IDB discovery
│   ├── idb.go                  # IndexedDB SQLite read/write + key codec
│   ├── containers.go           # containers.json parsing
│   └── go.mod
├── host-manifest/              # Native messaging JSON manifests
│   ├── fmac_rules_manager.darwin.json
│   └── fmac_rules_manager.linux.json
├── package.json                # web-ext scripts (lint, build, start)
├── dist/
├── listing/
└── docs/
```

## Extension

### Manifest (v2)

```json
{
  "manifest_version": 2,
  "name": "Container Rules Manager",
  "description": "Manage Multi-Account Containers site rules with a table editor, import, and export.",
  "version": "0.1.0",
  "icons": {
    "48": "icon.svg",
    "96": "icon.svg"
  },
  "browser_specific_settings": {
    "gecko": {
      "id": "fmac-rules-manager@sarcasticbird.com",
      "strict_min_version": "91.0"
    }
  },
  "options_ui": {
    "page": "options.html",
    "open_in_tab": true
  },
  "background": {
    "scripts": ["background.js"],
    "persistent": false
  },
  "permissions": [
    "nativeMessaging",
    "management",
    "downloads",
    "storage"
  ]
}
```

### Permissions Rationale

| Permission | Reason |
|-----------|--------|
| `nativeMessaging` | Communicate with Go host for SQLite access |
| `management` | Disable/enable MAC to force reload after writes |
| `downloads` | Export JSON file to disk |
| `storage` | Extension-local preferences (sort order, etc.) |

No `webRequest`, no `<all_urls>` — this extension never touches navigation.

### Background Script

Thin relay between the options page and the native host:

- Connects to native host via `browser.runtime.connectNative("fmac_rules_manager")`
- Listens for `browser.runtime.onMessage` from the options page
- Forwards commands to the native host, returns responses
- After successful write operations (`add`, `update`, `delete`, `import`), triggers MAC reload:
  1. `browser.management.setEnabled("@testpilot-containers", false)`
  2. Short delay (~500ms)
  3. `browser.management.setEnabled("@testpilot-containers", true)`
  4. If this fails (permission error, MAC not found), returns a warning in the response indicating browser restart is needed

### Options Page (Full-Page Tab)

The sole UI surface. On load, sends a `list` command and renders the rules table.

**Table columns:**
- Checkbox (for bulk select)
- Site (hostname)
- Container (name + color badge, rendered as a dropdown for editing)
- Actions (edit/delete buttons)

**Features:**
- Filter/search input — filters by site or container name
- Sortable columns — click header to sort
- Add rule — text input for hostname + container dropdown
- Inline edit — click a row's container cell to change assignment via dropdown
- Bulk operations — select multiple rows, bulk reassign or bulk delete
- Export button — sends `export` command, triggers `browser.downloads.download()` for a timestamped JSON file
- Import button — file picker (`<input type="file">`), reads JSON, sends `import` command, refreshes table

**States:**
- Loading — spinner while waiting for `list` response
- Error — banner with message (host not installed, profile not found, etc.)
- Empty — message with guidance if no rules exist
- Connected — normal table view

## Go Native Messaging Host

### Protocol

Standard native messaging: 4-byte little-endian length prefix on stdin/stdout. Each message is a JSON object.

**Request envelope:**
```json
{"cmd": "list"}
{"cmd": "add", "site": "github.com", "userContextId": 2}
```

**Response envelope:**
```json
{"ok": true, "data": {...}}
{"ok": false, "error": "human-readable message"}
```

### Commands

| Command | Input | Output |
|---------|-------|--------|
| `list` | `{}` | `{containers: [...], rules: [...]}` |
| `add` | `{site, userContextId}` | `{rule: {site, userContextId, container}}` |
| `update` | `{site, userContextId}` | `{rule: {site, userContextId, container}}` |
| `delete` | `{site}` | `{deleted: true}` |
| `export` | `{}` | Full export JSON (rules + containers + metadata) |
| `import` | `{rules: [...]}` | `{added: N, updated: N, skipped: N}` |

### Profile Discovery

Reuses logic from the existing `zen-containers-backup` script:

1. Check `~/Library/Application Support/zen/Profiles` (Zen first)
2. Fall back to `~/Library/Application Support/Firefox/Profiles`
3. On Linux: `~/.zen/` then `~/.mozilla/firefox/`
4. Parse `profiles.ini` — prefer `[Install*]` section's `Default` key, fall back to `[Profile*]` with `Default=1`
5. Find MAC extension UUID from `prefs.js` via the `extensions.webextensions.uuids` pref
6. Construct IDB path: `storage/default/moz-extension+++{UUID}^userContextId=4294967295/idb/`

### IDB Read/Write

**Reading** (from the backup script's proven logic):
- Open the `.sqlite` file in the IDB directory
- Query `SELECT key, data FROM object_data`
- Filter keys starting with `/siteContainerMap@@_`
- Decode keys: each byte minus 1, filter nulls, to get the hostname
- Extract `userContextId` from the data blob: find the `userContextId` marker, scan forward for digit bytes

**Writing:**
- Encode the key: hostname bytes each plus 1, null-terminated, prefixed with `/siteContainerMap@@_`
- Construct the data blob matching MAC's expected format (the exact binary structure will be reverse-engineered from a real IDB during implementation — the read path's `extract_user_context_id` shows the data contains structured JS object fields as binary)
- `INSERT OR REPLACE INTO object_data (key, data) VALUES (?, ?)`
- For deletes: `DELETE FROM object_data WHERE key = ?`

**Safety:**
- Use `sqlite3` `.backup` for reads when possible (safe even with WAL lock)
- For writes, open the database directly — if SQLite returns `SQLITE_BUSY`, retry once after a short pause, then return an error
- Never write while the browser has an active transaction (best-effort detection)

### Container Info

Read-only from `containers.json` in the profile root:
- Parse the `identities` array
- Filter to `public: true` entries
- Map `userContextId` to `{name, icon, color}` for display in the extension UI

## Data Flow

### Read
```
Options page loads
  -> browser.runtime.sendMessage({cmd: "list"})
  -> background.js connects to native host
  -> Go host: find profile -> read containers.json -> read IDB SQLite
  -> returns {containers: [...], rules: [...]}
  -> options page renders table
```

### Write
```
User adds/edits/deletes a rule
  -> browser.runtime.sendMessage({cmd: "add", site: "x.com", userContextId: 2})
  -> background.js relays to native host
  -> Go host writes to IDB SQLite
  -> returns {ok: true, data: {rule: {...}}}
  -> background.js triggers MAC disable/enable cycle
  -> options page sends fresh "list" to refresh table
```

## Export Format

Compatible with the existing `zen-containers-backup` script output:

```json
{
  "version": 1,
  "exportedAt": "2026-05-06T12:00:00Z",
  "browser": "zen",
  "containers": [
    {"userContextId": 2, "name": "Work", "icon": "briefcase", "color": "red"}
  ],
  "rules": [
    {"site": "github.com", "userContextId": 2, "container": "Work"}
  ],
  "totalRules": 1
}
```

**Import** accepts the same format. Only the `rules` array is used. Each rule is matched by `site`: existing rules are updated, new ones are inserted. Returns `{added: N, updated: N, skipped: N}`.

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Native host not installed | Background catches connection error, options page shows install instructions |
| Profile not found | Host returns `{ok: false, error: "..."}`, shown as banner in UI |
| MAC not installed | Host returns error (no UUID in prefs.js) |
| SQLite busy/locked | Host retries once after 200ms, then returns error |
| MAC reload fails | Background returns warning in response; UI shows "restart browser to apply" |
| Invalid import JSON | Host validates and returns error with details |

## Native Messaging Host Manifests

### macOS (`fmac_rules_manager.darwin.json`)

Installed to `~/Library/Application Support/Mozilla/NativeMessagingHosts/` (Firefox) or the Zen equivalent.

```json
{
  "name": "fmac_rules_manager",
  "description": "Native host for Container Rules Manager",
  "path": "/usr/local/bin/fmac-rules-manager-host",
  "type": "stdio",
  "allowed_extensions": ["fmac-rules-manager@sarcasticbird.com"]
}
```

### Linux (`fmac_rules_manager.linux.json`)

Installed to `~/.mozilla/native-messaging-hosts/` or `~/.zen/native-messaging-hosts/`.

Same content, different `path` as appropriate.

## Reference

- Existing backup script: `dotfiles/dot_local/bin/executable_zen-containers-backup`
- MAC extension ID: `@testpilot-containers`
- Concept doc: `dotfiles/docs/zen-container-manager-extension.md`
- Pattern extensions: `github-navigator` (web-ext + src/ structure), `default-container-handler` (contextualIdentities patterns)
