# Container Rules Manager

A Firefox extension for managing [Multi-Account Containers](https://addons.mozilla.org/en-US/firefox/addon/multi-account-containers/) site-to-container assignment rules. Add, edit, bulk-manage, and import/export rules through a dedicated options page — without clicking through the MAC popup one site at a time.

Also works with [Zen Browser](https://zen-browser.app/).

## Features

- **Table view** of all site-to-container assignments with sorting and filtering
- **Bulk add** — paste or type comma-separated hostnames to assign them to a container at once
- **Bulk operations** — select multiple rules for reassignment or deletion
- **Import/Export** — back up rules as JSON, edit the file, and import it back (merge or full replace)
- **Container picker** — filter by container with rule counts, search across all rules
- **Duplicate detection** — warns when a site is already assigned to a different container
- **Hostname validation** — cleans pasted URLs down to the hostname MAC expects (strips protocols, paths, handles ports)

## How It Works

Multi-Account Containers stores its site assignments in an IndexedDB database backed by SQLite. Firefox locks this database while MAC is enabled, so Container Rules Manager uses a Go native messaging host to read and write the SQLite file directly.

**Workflow:**

1. Temporarily disable Multi-Account Containers via `about:addons`
2. Open Container Rules Manager (toolbar icon or `about:addons` → Extensions → Container Rules Manager → Preferences)
3. Add, edit, delete, or import rules
4. Re-enable Multi-Account Containers — your changes take effect immediately

The extension detects whether MAC is enabled and shows a guided lock/unlock panel.

## Installation

### Extension

Install from [Firefox Add-ons](https://addons.mozilla.org/en-US/firefox/addon/container-rules-manager/) (pending review), or load temporarily for development:

1. Open `about:debugging#/runtime/this-firefox`
2. Click "Load Temporary Add-on"
3. Select `src/manifest.json`

### Native Messaging Host

The extension requires a native messaging host binary. Build it and run the install script:

```bash
cd host
go build -o ../dist/fmac-rules-manager-host .
cd ..
./host-manifest/install.sh
```

The install script:
- Copies the binary to `/usr/local/bin/fmac-rules-manager-host`
- Installs the native messaging manifest for Firefox (and Zen Browser if detected)

**Supported platforms:** macOS (darwin), Linux

## Building

**Prerequisites:** Go 1.22+, Node.js (for `web-ext`)

```bash
# Build the native host
cd host
go build -o ../dist/fmac-rules-manager-host .

# Run Go tests
go test -v ./...
cd ..

# Lint the extension
npm install
npm run lint

# Package the extension as .xpi
npm run build
```

The packaged extension will be in `dist/`.

## Project Structure

```
src/                    # Firefox extension
  manifest.json         # Extension manifest (Manifest V2)
  background.js         # Native messaging relay + MAC status check
  options.html/css/js   # Rules manager UI
  icon.svg              # Toolbar icon

host/                   # Go native messaging host
  main.go               # Entry point — reads stdin, dispatches, writes stdout
  protocol.go           # Native messaging wire format (4-byte LE length prefix + JSON)
  commands.go            # Command dispatcher (list, add, update, delete, export, import)
  idb.go                # SQLite read/write for MAC's IndexedDB
  idbkey.go             # IDB key encode/decode (siteContainerMap keys)
  blob.go               # Structured clone blob reader/writer (clone-and-patch)
  containers.go         # Reads containers.json for container metadata
  profile.go            # Profile discovery for Firefox and Zen Browser
  *_test.go             # Tests

host-manifest/          # Native messaging manifests and install script
```

## Permissions

| Permission | Reason |
|---|---|
| `nativeMessaging` | Communicate with the Go host to read/write MAC's database |
| `management` | Check whether Multi-Account Containers is enabled or disabled |
| `downloads` | Export rules as a downloadable JSON file |
| `storage` | Reserved for future settings persistence |

## Technical Notes

- MAC stores site rules as exact hostnames — no wildcards, no paths. Non-standard ports are appended directly (e.g., `localhost5173` for `localhost:5173`).
- MAC's IndexedDB uses Firefox's structured clone format. The host uses a clone-and-patch approach: copy an existing blob as a template, then patch in the new `userContextId`, `neverAsk`, and `identityMacAddonUUID` values.
- Firefox's IDB SQLite schema has triggers that call `update_refcount`, a C function registered at runtime. The host registers a no-op to prevent write failures.
- The host opens SQLite with `journal_mode(wal)` for safe concurrent access if the browser happens to touch the file.
- Import in "replace" mode runs delete-all + insert-all in a single transaction for atomicity.

## License

MIT
