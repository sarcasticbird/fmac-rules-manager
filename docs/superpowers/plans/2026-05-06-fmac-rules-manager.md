# FMAC Rules Manager Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Firefox/Zen extension + Go native messaging host that provides CRUD, import, and export for Multi-Account Containers site-to-container rules by reading/writing MAC's IndexedDB SQLite directly.

**Architecture:** Extension (vanilla JS, manifest v2) provides a full-page options table editor. Background script relays commands to a Go native messaging host. The host handles all profile discovery, SQLite I/O, and blob encoding/decoding. After writes, the extension toggles MAC via the `management` API to force a reload.

**Tech Stack:** Go (native host, `modernc.org/sqlite`), vanilla JS (extension), `web-ext` (build/lint)

---

### Task 1: Project Scaffolding

**Files:**
- Create: `package.json`
- Create: `src/manifest.json`
- Create: `src/icon.svg`
- Create: `host/go.mod`

- [ ] **Step 1: Create package.json**

```json
{
  "name": "fmac-rules-manager",
  "version": "0.1.0",
  "private": true,
  "description": "Manage Multi-Account Containers site rules with a table editor, import, and export.",
  "scripts": {
    "lint": "web-ext lint --source-dir src",
    "start": "web-ext run --source-dir src --target firefox-desktop",
    "build": "web-ext build --source-dir src --artifacts-dir dist --overwrite-dest"
  },
  "devDependencies": {
    "web-ext": "8.10.0"
  }
}
```

- [ ] **Step 2: Create src/manifest.json**

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

- [ ] **Step 3: Create a minimal SVG icon**

Create `src/icon.svg` — a simple container/box icon in the extension's brand color. Use a 96x96 viewBox.

- [ ] **Step 4: Initialize Go module**

```bash
cd host
go mod init github.com/sarcasticbird/fmac-rules-manager/host
go get modernc.org/sqlite
```

- [ ] **Step 5: Install npm deps**

```bash
npm install
```

- [ ] **Step 6: Commit scaffolding**

```bash
git add package.json package-lock.json src/manifest.json src/icon.svg host/go.mod host/go.sum
git commit -m "chore: project scaffolding with manifest, package.json, and go module"
```

---

### Task 2: Go Host — Native Messaging Protocol

**Files:**
- Create: `host/protocol.go`
- Create: `host/protocol_test.go`

The native messaging protocol uses 4-byte little-endian length prefix on stdin/stdout. Each message is JSON.

- [ ] **Step 1: Write failing tests for protocol read/write**

```go
// host/protocol_test.go
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestReadMessage(t *testing.T) {
	msg := map[string]string{"cmd": "list"}
	payload, _ := json.Marshal(msg)

	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(len(payload)))
	buf.Write(payload)

	result, err := readMessage(&buf)
	if err != nil {
		t.Fatalf("readMessage error: %v", err)
	}

	var parsed map[string]string
	json.Unmarshal(result, &parsed)
	if parsed["cmd"] != "list" {
		t.Errorf("expected cmd=list, got %s", parsed["cmd"])
	}
}

func TestWriteMessage(t *testing.T) {
	resp := Response{OK: true, Data: map[string]any{"count": 5}}

	var buf bytes.Buffer
	err := writeMessage(&buf, resp)
	if err != nil {
		t.Fatalf("writeMessage error: %v", err)
	}

	// Read the length prefix
	var length uint32
	binary.Read(&buf, binary.LittleEndian, &length)

	// Read the JSON payload
	payload := make([]byte, length)
	buf.Read(payload)

	var parsed Response
	json.Unmarshal(payload, &parsed)
	if !parsed.OK {
		t.Error("expected ok=true")
	}
}

func TestReadMessageTooLarge(t *testing.T) {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(2*1024*1024))
	buf.Write(make([]byte, 100))

	_, err := readMessage(&buf)
	if err == nil {
		t.Error("expected error for oversized message")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd host && go test -run TestReadMessage -v
```

Expected: compilation error — `readMessage`, `writeMessage`, `Response` not defined.

- [ ] **Step 3: Implement protocol.go**

```go
// host/protocol.go
package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const maxMessageSize = 1024 * 1024 // 1MB

type Request struct {
	Cmd           string     `json:"cmd"`
	Site          string     `json:"site,omitempty"`
	UserContextID int        `json:"userContextId,omitempty"`
	NeverAsk      *bool      `json:"neverAsk,omitempty"`
	Rules         []RuleSpec `json:"rules,omitempty"`
}

type RuleSpec struct {
	Site          string `json:"site"`
	UserContextID int    `json:"userContextId"`
	NeverAsk      bool   `json:"neverAsk"`
}

type Response struct {
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

func readMessage(r io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return nil, fmt.Errorf("reading length: %w", err)
	}
	if length > maxMessageSize {
		return nil, fmt.Errorf("message too large: %d bytes", length)
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("reading payload: %w", err)
	}
	return buf, nil
}

func writeMessage(w io.Writer, resp Response) error {
	payload, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshaling response: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(len(payload))); err != nil {
		return fmt.Errorf("writing length: %w", err)
	}
	if _, err := w.Write(payload); err != nil {
		return fmt.Errorf("writing payload: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd host && go test -run TestReadMessage -v && go test -run TestWriteMessage -v && go test -run TestReadMessageTooLarge -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add host/protocol.go host/protocol_test.go
git commit -m "feat: native messaging protocol read/write"
```

---

### Task 3: Go Host — Profile Discovery

**Files:**
- Create: `host/profile.go`
- Create: `host/profile_test.go`

Finds the browser profile directory and MAC extension UUID.

- [ ] **Step 1: Write failing tests**

```go
// host/profile_test.go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseProfilesINI(t *testing.T) {
	dir := t.TempDir()
	ini := `[Install1234]
Default=Profiles/abc123.default-release

[Profile0]
Name=default-release
Path=Profiles/abc123.default-release
Default=1
`
	os.WriteFile(filepath.Join(dir, "profiles.ini"), []byte(ini), 0644)
	os.MkdirAll(filepath.Join(dir, "Profiles", "abc123.default-release"), 0755)

	profileDir, err := findProfileDir(dir)
	if err != nil {
		t.Fatalf("findProfileDir: %v", err)
	}
	expected := filepath.Join(dir, "Profiles", "abc123.default-release")
	if profileDir != expected {
		t.Errorf("expected %s, got %s", expected, profileDir)
	}
}

func TestParseProfilesINIFallback(t *testing.T) {
	dir := t.TempDir()
	ini := `[Profile0]
Name=default
Path=Profiles/xyz.default
Default=1
`
	os.WriteFile(filepath.Join(dir, "profiles.ini"), []byte(ini), 0644)
	os.MkdirAll(filepath.Join(dir, "Profiles", "xyz.default"), 0755)

	profileDir, err := findProfileDir(dir)
	if err != nil {
		t.Fatalf("findProfileDir: %v", err)
	}
	expected := filepath.Join(dir, "Profiles", "xyz.default")
	if profileDir != expected {
		t.Errorf("expected %s, got %s", expected, profileDir)
	}
}

func TestFindMACUUID(t *testing.T) {
	dir := t.TempDir()
	prefs := `user_pref("extensions.webextensions.uuids", "{\"@testpilot-containers\":\"7d2af900-bdc2-447e-b8fe-bea337a8dfd2\",\"other\":\"abc\"}");`
	os.WriteFile(filepath.Join(dir, "prefs.js"), []byte(prefs), 0644)

	uuid, err := findMACUUID(dir)
	if err != nil {
		t.Fatalf("findMACUUID: %v", err)
	}
	if uuid != "7d2af900-bdc2-447e-b8fe-bea337a8dfd2" {
		t.Errorf("expected 7d2af900-..., got %s", uuid)
	}
}

func TestBuildIDBPath(t *testing.T) {
	uuid := "7d2af900-bdc2-447e-b8fe-bea337a8dfd2"
	profileDir := "/fake/profile"
	path := buildIDBPath(profileDir, uuid)
	expected := "/fake/profile/storage/default/moz-extension+++7d2af900-bdc2-447e-b8fe-bea337a8dfd2^userContextId=4294967295/idb"
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd host && go test -run TestParseProfilesINI -v
```

Expected: compilation error.

- [ ] **Step 3: Implement profile.go**

```go
// host/profile.go
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var uuidRegexp = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

func browserDirs() []string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "darwin" {
		return []string{
			filepath.Join(home, "Library", "Application Support", "zen"),
			filepath.Join(home, "Library", "Application Support", "Firefox"),
		}
	}
	return []string{
		filepath.Join(home, ".zen"),
		filepath.Join(home, ".mozilla", "firefox"),
	}
}

func discoverProfile() (profileDir string, browserDir string, err error) {
	for _, dir := range browserDirs() {
		if _, err := os.Stat(filepath.Join(dir, "profiles.ini")); err != nil {
			continue
		}
		profileDir, err := findProfileDir(dir)
		if err == nil {
			return profileDir, dir, nil
		}
	}
	return "", "", fmt.Errorf("no Zen or Firefox profile found")
}

func findProfileDir(browserDir string) (string, error) {
	f, err := os.Open(filepath.Join(browserDir, "profiles.ini"))
	if err != nil {
		return "", err
	}
	defer f.Close()

	var installDefault string
	var profileDefault string
	var currentPath string
	inInstall := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "[Install") {
			inInstall = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inInstall = false
		}

		if inInstall && strings.HasPrefix(line, "Default=") {
			installDefault = strings.TrimPrefix(line, "Default=")
			continue
		}

		if strings.HasPrefix(line, "Path=") {
			currentPath = strings.TrimPrefix(line, "Path=")
		}
		if strings.HasPrefix(line, "Default=1") && currentPath != "" {
			profileDefault = currentPath
		}
	}

	if installDefault != "" {
		dir := filepath.Join(browserDir, installDefault)
		if _, err := os.Stat(dir); err == nil {
			return dir, nil
		}
	}
	if profileDefault != "" {
		dir := filepath.Join(browserDir, profileDefault)
		if _, err := os.Stat(dir); err == nil {
			return dir, nil
		}
	}
	return "", fmt.Errorf("no default profile found in %s", browserDir)
}

func findMACUUID(profileDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(profileDir, "prefs.js"))
	if err != nil {
		return "", fmt.Errorf("reading prefs.js: %w", err)
	}

	content := string(data)
	idx := strings.Index(content, "testpilot-containers")
	if idx == -1 {
		return "", fmt.Errorf("multi-account containers extension not found in prefs.js")
	}

	// Find the UUID after testpilot-containers
	rest := content[idx:]
	match := uuidRegexp.FindString(rest)
	if match == "" {
		return "", fmt.Errorf("could not extract MAC UUID from prefs.js")
	}
	return match, nil
}

func buildIDBPath(profileDir, uuid string) string {
	return filepath.Join(
		profileDir,
		"storage", "default",
		fmt.Sprintf("moz-extension+++%s^userContextId=4294967295", uuid),
		"idb",
	)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd host && go test -run TestParseProfilesINI -v && go test -run TestFindMACUUID -v && go test -run TestBuildIDBPath -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add host/profile.go host/profile_test.go
git commit -m "feat: profile discovery for Zen and Firefox"
```

---

### Task 4: Go Host — Containers.json Reader

**Files:**
- Create: `host/containers.go`
- Create: `host/containers_test.go`

Reads container definitions (name, icon, color) from the profile's `containers.json`.

- [ ] **Step 1: Write failing tests**

```go
// host/containers_test.go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadContainers(t *testing.T) {
	dir := t.TempDir()
	data := `{
		"version": 4,
		"identities": [
			{"userContextId": 1, "public": true, "icon": "fingerprint", "color": "blue", "name": "Personal"},
			{"userContextId": 2, "public": true, "icon": "briefcase", "color": "red", "name": "Work"},
			{"userContextId": 3, "public": false, "icon": "fence", "color": "green", "name": "Mozilla"},
			{"userContextId": 4, "public": true, "icon": "tree", "color": "green", "name": "Social"}
		]
	}`
	os.WriteFile(filepath.Join(dir, "containers.json"), []byte(data), 0644)

	containers, err := readContainers(dir)
	if err != nil {
		t.Fatalf("readContainers: %v", err)
	}
	// Should only include public containers
	if len(containers) != 3 {
		t.Fatalf("expected 3 public containers, got %d", len(containers))
	}
	if containers[0].Name != "Personal" {
		t.Errorf("expected Personal, got %s", containers[0].Name)
	}
	if containers[1].UserContextID != 2 {
		t.Errorf("expected userContextId=2, got %d", containers[1].UserContextID)
	}
}

func TestReadContainersMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := readContainers(dir)
	if err == nil {
		t.Error("expected error for missing containers.json")
	}
}

func TestContainerMap(t *testing.T) {
	containers := []Container{
		{UserContextID: 1, Name: "Personal", Icon: "fingerprint", Color: "blue"},
		{UserContextID: 2, Name: "Work", Icon: "briefcase", Color: "red"},
	}
	m := containerMap(containers)
	c, ok := m[2]
	if !ok {
		t.Fatal("expected container 2 in map")
	}
	if c.Name != "Work" {
		t.Errorf("expected Work, got %s", c.Name)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd host && go test -run TestReadContainers -v
```

Expected: compilation error.

- [ ] **Step 3: Implement containers.go**

```go
// host/containers.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Container struct {
	UserContextID int    `json:"userContextId"`
	Name          string `json:"name"`
	Icon          string `json:"icon"`
	Color         string `json:"color"`
}

type containersFile struct {
	Identities []struct {
		UserContextID int    `json:"userContextId"`
		Public        bool   `json:"public"`
		Icon          string `json:"icon"`
		Color         string `json:"color"`
		Name          string `json:"name"`
	} `json:"identities"`
}

func readContainers(profileDir string) ([]Container, error) {
	data, err := os.ReadFile(filepath.Join(profileDir, "containers.json"))
	if err != nil {
		return nil, fmt.Errorf("reading containers.json: %w", err)
	}

	var cf containersFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parsing containers.json: %w", err)
	}

	var containers []Container
	for _, id := range cf.Identities {
		if !id.Public {
			continue
		}
		containers = append(containers, Container{
			UserContextID: id.UserContextID,
			Name:          id.Name,
			Icon:          id.Icon,
			Color:         id.Color,
		})
	}
	return containers, nil
}

func containerMap(containers []Container) map[int]Container {
	m := make(map[int]Container, len(containers))
	for _, c := range containers {
		m[c.UserContextID] = c
	}
	return m
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd host && go test -run TestReadContainers -v && go test -run TestContainerMap -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add host/containers.go host/containers_test.go
git commit -m "feat: containers.json reader"
```

---

### Task 5: Go Host — IDB Key Codec

**Files:**
- Create: `host/idbkey.go`
- Create: `host/idbkey_test.go`

Encodes/decodes IDB keys. Keys are byte-plus-one encoded with a leading `0x30` byte (which decodes to the `/` prefix).

- [ ] **Step 1: Write failing tests**

```go
// host/idbkey_test.go
package main

import (
	"bytes"
	"testing"
)

func TestDecodeIDBKey(t *testing.T) {
	// Hex from real data: decodes to "/siteContainerMap@@_aistudio.google.com"
	raw := []byte{
		0x30, 0x74, 0x69, 0x75, 0x65, 0x44, 0x70, 0x6F,
		0x75, 0x62, 0x6A, 0x6F, 0x66, 0x73, 0x4E, 0x62,
		0x71, 0x41, 0x41, 0x60, 0x62, 0x6A, 0x74, 0x75,
		0x76, 0x65, 0x6A, 0x70, 0x2F, 0x68, 0x70, 0x70,
		0x68, 0x6D, 0x66, 0x2F, 0x64, 0x70, 0x6E,
	}
	decoded := decodeIDBKey(raw)
	expected := "/siteContainerMap@@_aistudio.google.com"
	if decoded != expected {
		t.Errorf("expected %q, got %q", expected, decoded)
	}
}

func TestEncodeIDBKey(t *testing.T) {
	key := "/siteContainerMap@@_github.com"
	encoded := encodeIDBKey(key)
	decoded := decodeIDBKey(encoded)
	if decoded != key {
		t.Errorf("round-trip failed: expected %q, got %q", key, decoded)
	}
}

func TestSiteContainerMapKey(t *testing.T) {
	key := siteContainerMapKey("github.com")
	decoded := decodeIDBKey(key)
	expected := "/siteContainerMap@@_github.com"
	if decoded != expected {
		t.Errorf("expected %q, got %q", expected, decoded)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	sites := []string{"github.com", "mail.google.com", "us-east-2.signin.aws.amazon.com"}
	for _, site := range sites {
		key := siteContainerMapKey(site)
		decoded := decodeIDBKey(key)
		expected := "/siteContainerMap@@_" + site
		if decoded != expected {
			t.Errorf("site %s: expected %q, got %q", site, expected, decoded)
		}
	}
}

func TestIsSiteContainerMapKey(t *testing.T) {
	good := encodeIDBKey("/siteContainerMap@@_github.com")
	bad := encodeIDBKey("/containerTabsOpened")

	if !isSiteContainerMapKey(good) {
		t.Error("expected true for siteContainerMap key")
	}
	if isSiteContainerMapKey(bad) {
		t.Error("expected false for non-siteContainerMap key")
	}
}

func TestExtractSiteFromKey(t *testing.T) {
	key := encodeIDBKey("/siteContainerMap@@_docs.google.com")
	site := extractSiteFromKey(key)
	if site != "docs.google.com" {
		t.Errorf("expected docs.google.com, got %s", site)
	}
}

// Verify encoded key matches real captured data
func TestEncodeMatchesReal(t *testing.T) {
	// Real key for "aistudio.google.com" from the live IDB
	realKey := []byte{
		0x30, 0x74, 0x69, 0x75, 0x65, 0x44, 0x70, 0x6F,
		0x75, 0x62, 0x6A, 0x6F, 0x66, 0x73, 0x4E, 0x62,
		0x71, 0x41, 0x41, 0x60, 0x62, 0x6A, 0x74, 0x75,
		0x76, 0x65, 0x6A, 0x70, 0x2F, 0x68, 0x70, 0x70,
		0x68, 0x6D, 0x66, 0x2F, 0x64, 0x70, 0x6E,
	}
	generated := siteContainerMapKey("aistudio.google.com")
	if !bytes.Equal(realKey, generated) {
		t.Errorf("generated key doesn't match real key\nreal:      %x\ngenerated: %x", realKey, generated)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd host && go test -run TestDecodeIDBKey -v
```

Expected: compilation error.

- [ ] **Step 3: Implement idbkey.go**

```go
// host/idbkey.go
package main

import "strings"

const siteContainerMapPrefix = "/siteContainerMap@@_"

func decodeIDBKey(raw []byte) string {
	var b strings.Builder
	for _, c := range raw {
		if c > 0 {
			b.WriteByte(c - 1)
		}
	}
	return b.String()
}

func encodeIDBKey(key string) []byte {
	encoded := make([]byte, len(key))
	for i, c := range []byte(key) {
		encoded[i] = c + 1
	}
	return encoded
}

func siteContainerMapKey(site string) []byte {
	return encodeIDBKey(siteContainerMapPrefix + site)
}

func isSiteContainerMapKey(key []byte) bool {
	decoded := decodeIDBKey(key)
	return strings.HasPrefix(decoded, siteContainerMapPrefix)
}

func extractSiteFromKey(key []byte) string {
	decoded := decodeIDBKey(key)
	return strings.TrimPrefix(decoded, siteContainerMapPrefix)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd host && go test -run TestDecodeIDBKey -v && go test -run TestEncodeIDBKey -v && go test -run TestSiteContainerMapKey -v && go test -run TestEncodeDecodeRoundTrip -v && go test -run TestIsSiteContainerMapKey -v && go test -run TestExtractSiteFromKey -v && go test -run TestEncodeMatchesReal -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add host/idbkey.go host/idbkey_test.go
git commit -m "feat: IDB key encode/decode codec"
```

---

### Task 6: Go Host — IDB Blob Reader

**Files:**
- Create: `host/blob.go`
- Create: `host/blob_test.go`

Extracts `userContextId` and `neverAsk` from structured clone data blobs. Uses marker-scanning (proven by the backup script).

- [ ] **Step 1: Write failing tests with real captured blobs**

```go
// host/blob_test.go
package main

import (
	"encoding/hex"
	"testing"
)

// Real blob: neverAsk=true, userContextId=6, UUID=b022020c-1449-4a95-99a4-d8b89264d582
var blobTrueCtx6, _ = hex.DecodeString("A801040300010104F1FF0106700800FFFF0D0000800400FFFF75736572436F6E746578744964000000010D18003601290C000000080D101C6E6576657241736B0114180200FFFF14000005404C6964656E746974794D61634164646F6E55554944012400240D20BC62303232303230632D313434392D346139352D393961342D64386238393236346435383200000000000000001300FFFF")

// Real blob: neverAsk=false, userContextId=2, UUID=4e30c912-fbd7-43ca-af2f-e6bc24fd1d90
var blobFalseCtx2, _ = hex.DecodeString("A801040300010104F1FF0106700800FFFF0D0000800400FFFF75736572436F6E746578744964000000010D18003201290C000000080D10486E6576657241736B0100000002" + "00FFFF14000005404C6964656E746974794D61634164646F6E55554944013800240D20BC34653330633931322D666264372D343363612D616632662D65366263323466643164393000000000000000001300FFFF")

// Real blob: neverAsk=true, userContextId=10 (two digits), UUID for facebook container
var blobTrueCtx10, _ = hex.DecodeString("A801040300010104F1FF0106700800FFFF0D0000800400FFFF75736572436F6E746578744964000000020D18043130012A0800000008" + "0D101C6E6576657241736B0116180200FFFF14000005404C6964656E746974794D61634164646F6E55554944012400240D20BC62303232303230632D313434392D346139352D393961342D64386238393236346435383200000000000000001300FFFF")

func TestExtractUserContextID(t *testing.T) {
	tests := []struct {
		name     string
		blob     []byte
		expected int
	}{
		{"single digit ctx=6", blobTrueCtx6, 6},
		{"single digit ctx=2", blobFalseCtx2, 2},
		{"two digit ctx=10", blobTrueCtx10, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, err := extractUserContextID(tt.blob)
			if err != nil {
				t.Fatalf("extractUserContextID: %v", err)
			}
			if ctx != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, ctx)
			}
		})
	}
}

func TestExtractNeverAsk(t *testing.T) {
	tests := []struct {
		name     string
		blob     []byte
		expected bool
	}{
		{"neverAsk=true", blobTrueCtx6, true},
		{"neverAsk=false", blobFalseCtx2, false},
		{"neverAsk=true (ctx=10)", blobTrueCtx10, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			na, err := extractNeverAsk(tt.blob)
			if err != nil {
				t.Fatalf("extractNeverAsk: %v", err)
			}
			if na != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, na)
			}
		})
	}
}

func TestExtractUUID(t *testing.T) {
	uuid, err := extractUUID(blobTrueCtx6)
	if err != nil {
		t.Fatalf("extractUUID: %v", err)
	}
	if uuid != "b022020c-1449-4a95-99a4-d8b89264d582" {
		t.Errorf("expected b022020c-..., got %s", uuid)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd host && go test -run TestExtract -v
```

Expected: compilation error.

- [ ] **Step 3: Implement blob.go reader functions**

```go
// host/blob.go
package main

import (
	"bytes"
	"fmt"
	"strconv"
)

var (
	markerUserContextID     = []byte("userContextId")
	markerNeverAsk          = []byte("neverAsk")
	markerIdentityMacAddonUUID = []byte("identityMacAddonUUID")
)

func extractUserContextID(blob []byte) (int, error) {
	pos := bytes.Index(blob, markerUserContextID)
	if pos == -1 {
		return 0, fmt.Errorf("userContextId marker not found")
	}

	i := pos + len(markerUserContextID)
	for i < len(blob) && (blob[i] < 0x30 || blob[i] > 0x39) {
		i++
	}

	var digits []byte
	for i < len(blob) && blob[i] >= 0x30 && blob[i] <= 0x39 {
		digits = append(digits, blob[i])
		i++
	}

	if len(digits) == 0 {
		return 0, fmt.Errorf("no digits found after userContextId")
	}

	val, err := strconv.Atoi(string(digits))
	if err != nil {
		return 0, fmt.Errorf("parsing userContextId digits: %w", err)
	}
	return val, nil
}

func extractNeverAsk(blob []byte) (bool, error) {
	pos := bytes.Index(blob, markerNeverAsk)
	if pos == -1 {
		return false, fmt.Errorf("neverAsk marker not found")
	}

	// After "neverAsk" text, skip 1 byte (0x01), then check the value byte.
	// true encoding:  01 14/16 ...   (byte after 0x01 is > 0x00)
	// false encoding: 01 00 00 00 ...
	valOffset := pos + len(markerNeverAsk) + 1
	if valOffset >= len(blob) {
		return false, fmt.Errorf("neverAsk value out of bounds")
	}

	return blob[valOffset] != 0x00, nil
}

func extractUUID(blob []byte) (string, error) {
	pos := bytes.Index(blob, markerIdentityMacAddonUUID)
	if pos == -1 {
		return "", fmt.Errorf("identityMacAddonUUID marker not found")
	}

	// Scan forward from the marker to find a UUID pattern (36 chars: 8-4-4-4-12 hex)
	searchStart := pos + len(markerIdentityMacAddonUUID)
	for i := searchStart; i < len(blob)-36; i++ {
		candidate := string(blob[i : i+36])
		if isUUIDFormat(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("UUID not found after identityMacAddonUUID marker")
}

func isUUIDFormat(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd host && go test -run TestExtract -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add host/blob.go host/blob_test.go
git commit -m "feat: IDB structured clone blob reader"
```

---

### Task 7: Go Host — IDB Blob Writer (Clone and Patch)

**Files:**
- Modify: `host/blob.go`
- Modify: `host/blob_test.go`

Constructs new data blobs by cloning an existing blob and patching the variable fields (userContextId digits, neverAsk encoding, UUID). This avoids reimplementing Firefox's structured clone serialization.

- [ ] **Step 1: Write failing tests for blob construction**

Add to `host/blob_test.go`:

```go
func TestBuildBlob(t *testing.T) {
	// Use the real neverAsk=true blob as template
	newBlob, err := buildBlob(blobTrueCtx6, 3, true)
	if err != nil {
		t.Fatalf("buildBlob: %v", err)
	}

	// Verify the new blob has the correct userContextId
	ctx, err := extractUserContextID(newBlob)
	if err != nil {
		t.Fatalf("extractUserContextID from new blob: %v", err)
	}
	if ctx != 3 {
		t.Errorf("expected ctx=3, got %d", ctx)
	}

	// Verify neverAsk is still true
	na, err := extractNeverAsk(newBlob)
	if err != nil {
		t.Fatalf("extractNeverAsk from new blob: %v", err)
	}
	if !na {
		t.Error("expected neverAsk=true")
	}

	// Verify UUID was replaced (not the same as template)
	uuid, err := extractUUID(newBlob)
	if err != nil {
		t.Fatalf("extractUUID from new blob: %v", err)
	}
	if uuid == "b022020c-1449-4a95-99a4-d8b89264d582" {
		t.Error("UUID should be different from template")
	}
	if !isUUIDFormat(uuid) {
		t.Errorf("generated UUID is not valid format: %s", uuid)
	}
}

func TestBuildBlobNeverAskFalse(t *testing.T) {
	newBlob, err := buildBlob(blobFalseCtx2, 5, false)
	if err != nil {
		t.Fatalf("buildBlob: %v", err)
	}

	ctx, _ := extractUserContextID(newBlob)
	if ctx != 5 {
		t.Errorf("expected ctx=5, got %d", ctx)
	}

	na, _ := extractNeverAsk(newBlob)
	if na {
		t.Error("expected neverAsk=false")
	}
}

func TestBuildBlobSameDigitCount(t *testing.T) {
	// Template has ctx=6 (1 digit), build with ctx=3 (1 digit) — same digit count
	newBlob, err := buildBlob(blobTrueCtx6, 3, true)
	if err != nil {
		t.Fatalf("buildBlob: %v", err)
	}
	// Same digit count means blob length should match template
	if len(newBlob) != len(blobTrueCtx6) {
		t.Errorf("expected length %d, got %d", len(blobTrueCtx6), len(newBlob))
	}
}

func TestPickTemplate(t *testing.T) {
	templates := [][]byte{blobTrueCtx6, blobFalseCtx2}

	tmpl := pickTemplate(templates, true)
	na, _ := extractNeverAsk(tmpl)
	if !na {
		t.Error("expected neverAsk=true template")
	}

	tmpl = pickTemplate(templates, false)
	na, _ = extractNeverAsk(tmpl)
	if na {
		t.Error("expected neverAsk=false template")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd host && go test -run TestBuildBlob -v
```

Expected: compilation error.

- [ ] **Step 3: Implement blob builder**

Add to `host/blob.go`:

```go
import (
	"crypto/rand"
	"fmt"
)

func generateUUID() string {
	var uuid [16]byte
	rand.Read(uuid[:])
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // variant 2
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

func buildBlob(template []byte, userContextID int, neverAsk bool) ([]byte, error) {
	ctxStr := strconv.Itoa(userContextID)

	// Find the userContextId digits in the template
	tmplCtxID, err := extractUserContextID(template)
	if err != nil {
		return nil, fmt.Errorf("reading template userContextId: %w", err)
	}
	tmplCtxStr := strconv.Itoa(tmplCtxID)

	// Find the digit positions in the template
	pos := bytes.Index(template, markerUserContextID)
	i := pos + len(markerUserContextID)
	for i < len(template) && (template[i] < 0x30 || template[i] > 0x39) {
		i++
	}
	digitStart := i

	// Build new blob by replacing the digits
	var result []byte

	if len(ctxStr) == len(tmplCtxStr) {
		// Same digit count: simple byte replacement
		result = make([]byte, len(template))
		copy(result, template)
		for j := 0; j < len(ctxStr); j++ {
			result[digitStart+j] = ctxStr[j]
		}
	} else {
		// Different digit count: need to find the full userContextId encoding region
		// and reconstruct. The bytes before the digits include a length indicator.
		result, err = rebuildWithNewCtxID(template, digitStart, tmplCtxStr, ctxStr)
		if err != nil {
			return nil, err
		}
	}

	// Replace UUID
	newUUID := generateUUID()
	uuidPos := bytes.Index(result, markerIdentityMacAddonUUID)
	if uuidPos == -1 {
		return nil, fmt.Errorf("identityMacAddonUUID marker not found in result")
	}
	searchStart := uuidPos + len(markerIdentityMacAddonUUID)
	for j := searchStart; j < len(result)-36; j++ {
		if isUUIDFormat(string(result[j : j+36])) {
			copy(result[j:j+36], []byte(newUUID))
			break
		}
	}

	// Handle neverAsk if it differs from template
	tmplNA, _ := extractNeverAsk(template)
	if tmplNA != neverAsk {
		result, err = toggleNeverAsk(result, neverAsk)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func rebuildWithNewCtxID(template []byte, digitStart int, oldCtx, newCtx string) ([]byte, error) {
	// The structured clone encodes the userContextId string with a length byte
	// at a known offset before the digits. We need to find and update it.
	// Length byte is at (digitStart - 3) based on observed format:
	// ... XX 0D 18 LL DD DD ... where LL encodes length info, DD are digits
	digitEnd := digitStart + len(oldCtx)

	var result bytes.Buffer
	result.Write(template[:digitStart])
	result.Write([]byte(newCtx))
	result.Write(template[digitEnd:])

	blob := result.Bytes()

	// Update the length-related bytes before the digits
	// Observed pattern: byte at (digitStart - 4) correlates with digit count
	// Single digit: 0x00 at (digitStart - 4)
	// Two digits: 0x04 at (digitStart - 4)
	// Also byte at (digitStart - markerLen - 3) holds digit count
	markerPos := bytes.Index(blob, markerUserContextID)
	countPos := markerPos + len(markerUserContextID) + 3 // 3 null bytes then count
	if countPos < len(blob) {
		blob[countPos] = byte(len(newCtx))
	}

	return blob, nil
}

func toggleNeverAsk(blob []byte, neverAsk bool) ([]byte, error) {
	pos := bytes.Index(blob, markerNeverAsk)
	if pos == -1 {
		return nil, fmt.Errorf("neverAsk marker not found")
	}

	afterMarker := pos + len(markerNeverAsk)

	// Current encoding after "neverAsk":
	// true:  01 XX 18 02 00 FF FF     (7 bytes, XX = 0x14 or 0x16)
	// false: 01 00 00 00 02 00 FF FF  (8 bytes)

	currentTrue := blob[afterMarker+1] != 0x00

	if currentTrue == neverAsk {
		return blob, nil
	}

	if currentTrue && !neverAsk {
		// Replace 7-byte true encoding with 8-byte false encoding
		trueEnd := afterMarker + 7
		var result bytes.Buffer
		result.Write(blob[:afterMarker])
		result.Write([]byte{0x01, 0x00, 0x00, 0x00, 0x02, 0x00, 0xFF, 0xFF})
		result.Write(blob[trueEnd:])
		return result.Bytes(), nil
	}

	// Replace 8-byte false encoding with 7-byte true encoding
	falseEnd := afterMarker + 8
	var result bytes.Buffer
	result.Write(blob[:afterMarker])
	result.Write([]byte{0x01, 0x14, 0x18, 0x02, 0x00, 0xFF, 0xFF})
	result.Write(blob[falseEnd:])
	return result.Bytes(), nil
}

func pickTemplate(blobs [][]byte, neverAsk bool) []byte {
	for _, b := range blobs {
		na, err := extractNeverAsk(b)
		if err == nil && na == neverAsk {
			return b
		}
	}
	// No exact match — return any blob (we'll toggle neverAsk)
	if len(blobs) > 0 {
		return blobs[0]
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd host && go test -run TestBuildBlob -v && go test -run TestPickTemplate -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add host/blob.go host/blob_test.go
git commit -m "feat: IDB blob writer with clone-and-patch approach"
```

---

### Task 8: Go Host — IDB SQLite Read/Write

**Files:**
- Create: `host/idb.go`
- Create: `host/idb_test.go`

Reads and writes siteContainerMap entries from the IDB SQLite file.

- [ ] **Step 1: Write failing tests**

```go
// host/idb_test.go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func createTestDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")

	db, err := openIDB(dbPath)
	if err != nil {
		t.Fatalf("openIDB: %v", err)
	}
	defer db.Close()

	// Create schema matching Firefox IDB
	_, err = db.Exec(`
		CREATE TABLE object_store (
			id INTEGER PRIMARY KEY,
			auto_increment INTEGER NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			key_path TEXT
		);
		INSERT INTO object_store VALUES (1, 0, 'storage-local-data', '');
		CREATE TABLE object_data (
			object_store_id INTEGER NOT NULL,
			key BLOB NOT NULL,
			index_data_values BLOB DEFAULT NULL,
			file_ids TEXT,
			data BLOB NOT NULL,
			PRIMARY KEY (object_store_id, key)
		) WITHOUT ROWID;
	`)
	if err != nil {
		t.Fatalf("creating schema: %v", err)
	}

	// Insert real test data
	key := siteContainerMapKey("github.com")
	_, err = db.Exec("INSERT INTO object_data (object_store_id, key, data) VALUES (1, ?, ?)",
		key, blobTrueCtx6)
	if err != nil {
		t.Fatalf("inserting test data: %v", err)
	}

	key2 := siteContainerMapKey("mail.google.com")
	_, err = db.Exec("INSERT INTO object_data (object_store_id, key, data) VALUES (1, ?, ?)",
		key2, blobFalseCtx2)
	if err != nil {
		t.Fatalf("inserting test data 2: %v", err)
	}

	return dbPath
}

func TestReadRules(t *testing.T) {
	dbPath := createTestDB(t)
	rules, blobs, err := readRules(dbPath)
	if err != nil {
		t.Fatalf("readRules: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	if len(blobs) != 2 {
		t.Errorf("expected 2 template blobs, got %d", len(blobs))
	}

	found := false
	for _, r := range rules {
		if r.Site == "github.com" && r.UserContextID == 6 && r.NeverAsk {
			found = true
		}
	}
	if !found {
		t.Error("expected github.com rule with ctx=6, neverAsk=true")
	}
}

func TestAddRule(t *testing.T) {
	dbPath := createTestDB(t)
	_, blobs, _ := readRules(dbPath)

	err := addRule(dbPath, "docs.google.com", 3, true, blobs)
	if err != nil {
		t.Fatalf("addRule: %v", err)
	}

	rules, _, _ := readRules(dbPath)
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}

	found := false
	for _, r := range rules {
		if r.Site == "docs.google.com" && r.UserContextID == 3 {
			found = true
		}
	}
	if !found {
		t.Error("expected docs.google.com rule")
	}
}

func TestUpdateRule(t *testing.T) {
	dbPath := createTestDB(t)
	_, blobs, _ := readRules(dbPath)

	err := updateRule(dbPath, "github.com", 4, true, blobs)
	if err != nil {
		t.Fatalf("updateRule: %v", err)
	}

	rules, _, _ := readRules(dbPath)
	for _, r := range rules {
		if r.Site == "github.com" {
			if r.UserContextID != 4 {
				t.Errorf("expected ctx=4, got %d", r.UserContextID)
			}
			return
		}
	}
	t.Error("github.com rule not found after update")
}

func TestDeleteRule(t *testing.T) {
	dbPath := createTestDB(t)
	err := deleteRule(dbPath, "github.com")
	if err != nil {
		t.Fatalf("deleteRule: %v", err)
	}

	rules, _, _ := readRules(dbPath)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Site == "github.com" {
		t.Error("github.com should have been deleted")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd host && go test -run TestReadRules -v
```

Expected: compilation error.

- [ ] **Step 3: Implement idb.go**

```go
// host/idb.go
package main

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"os"

	_ "modernc.org/sqlite"
)

type SiteRule struct {
	Site          string `json:"site"`
	UserContextID int    `json:"userContextId"`
	NeverAsk      bool   `json:"neverAsk"`
}

func findSQLiteFile(idbDir string) (string, error) {
	entries, err := os.ReadDir(idbDir)
	if err != nil {
		return "", fmt.Errorf("reading IDB directory: %w", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".sqlite" {
			return filepath.Join(idbDir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("no .sqlite file found in %s", idbDir)
}

func openIDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_busy_timeout=200")
	if err != nil {
		return nil, fmt.Errorf("opening SQLite: %w", err)
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

func readRules(dbPath string) ([]SiteRule, [][]byte, error) {
	db, err := openIDB(dbPath)
	if err != nil {
		return nil, nil, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT key, data FROM object_data WHERE object_store_id = 1")
	if err != nil {
		return nil, nil, fmt.Errorf("querying object_data: %w", err)
	}
	defer rows.Close()

	var rules []SiteRule
	var templateBlobs [][]byte

	for rows.Next() {
		var keyBlob, dataBlob []byte
		if err := rows.Scan(&keyBlob, &dataBlob); err != nil {
			return nil, nil, fmt.Errorf("scanning row: %w", err)
		}

		if !isSiteContainerMapKey(keyBlob) {
			continue
		}

		site := extractSiteFromKey(keyBlob)
		ctxID, err := extractUserContextID(dataBlob)
		if err != nil {
			continue
		}
		neverAsk, _ := extractNeverAsk(dataBlob)

		rules = append(rules, SiteRule{
			Site:          site,
			UserContextID: ctxID,
			NeverAsk:      neverAsk,
		})
		templateBlobs = append(templateBlobs, dataBlob)
	}

	return rules, templateBlobs, rows.Err()
}

func addRule(dbPath string, site string, userContextID int, neverAsk bool, templateBlobs [][]byte) error {
	tmpl := pickTemplate(templateBlobs, neverAsk)
	if tmpl == nil {
		return fmt.Errorf("no template blob available — at least one existing rule is required")
	}

	blob, err := buildBlob(tmpl, userContextID, neverAsk)
	if err != nil {
		return fmt.Errorf("building blob: %w", err)
	}

	key := siteContainerMapKey(site)

	db, err := openIDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(
		"INSERT OR REPLACE INTO object_data (object_store_id, key, data) VALUES (1, ?, ?)",
		key, blob,
	)
	if err != nil {
		return fmt.Errorf("inserting rule: %w", err)
	}
	return nil
}

func updateRule(dbPath string, site string, userContextID int, neverAsk bool, templateBlobs [][]byte) error {
	return addRule(dbPath, site, userContextID, neverAsk, templateBlobs)
}

func deleteRule(dbPath string, site string) error {
	key := siteContainerMapKey(site)

	db, err := openIDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	result, err := db.Exec(
		"DELETE FROM object_data WHERE object_store_id = 1 AND key = ?",
		key,
	)
	if err != nil {
		return fmt.Errorf("deleting rule: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("rule not found: %s", site)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd host && go test -run "TestReadRules|TestAddRule|TestUpdateRule|TestDeleteRule" -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add host/idb.go host/idb_test.go
git commit -m "feat: IDB SQLite read/write for site rules"
```

---

### Task 9: Go Host — Command Dispatcher + Export/Import

**Files:**
- Create: `host/commands.go`
- Create: `host/commands_test.go`
- Create: `host/main.go`

Wires up the command dispatcher and implements export/import.

- [ ] **Step 1: Write failing tests for export/import**

```go
// host/commands_test.go
package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestExportCommand(t *testing.T) {
	dbPath := createTestDB(t)
	profileDir := t.TempDir()

	// Create containers.json in the profile dir
	containersJSON := `{"identities": [
		{"userContextId": 2, "public": true, "icon": "briefcase", "color": "red", "name": "Work"},
		{"userContextId": 6, "public": true, "icon": "tree", "color": "green", "name": "Google"}
	]}`
	writeTestFile(t, profileDir, "containers.json", containersJSON)

	ctx := &cmdContext{dbPath: dbPath, profileDir: profileDir}
	resp := handleExport(ctx)

	if !resp.OK {
		t.Fatalf("export failed: %s", resp.Error)
	}

	data, _ := json.Marshal(resp.Data)
	var export ExportData
	json.Unmarshal(data, &export)

	if export.Version != 1 {
		t.Errorf("expected version 1, got %d", export.Version)
	}
	if len(export.Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(export.Rules))
	}
	if len(export.Containers) != 2 {
		t.Errorf("expected 2 containers, got %d", len(export.Containers))
	}
}

func TestImportCommand(t *testing.T) {
	dbPath := createTestDB(t)
	profileDir := t.TempDir()

	ctx := &cmdContext{dbPath: dbPath, profileDir: profileDir}

	importRules := []RuleSpec{
		{Site: "new-site.com", UserContextID: 6, NeverAsk: true},
		{Site: "github.com", UserContextID: 3, NeverAsk: true}, // existing, should update
	}

	resp := handleImport(ctx, importRules)
	if !resp.OK {
		t.Fatalf("import failed: %s", resp.Error)
	}

	data, _ := json.Marshal(resp.Data)
	var result ImportResult
	json.Unmarshal(data, &result)

	if result.Added != 1 {
		t.Errorf("expected 1 added, got %d", result.Added)
	}
	if result.Updated != 1 {
		t.Errorf("expected 1 updated, got %d", result.Updated)
	}
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd host && go test -run "TestExportCommand|TestImportCommand" -v
```

Expected: compilation error.

- [ ] **Step 3: Implement commands.go**

```go
// host/commands.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type cmdContext struct {
	dbPath     string
	profileDir string
}

type ExportData struct {
	Version    int         `json:"version"`
	ExportedAt string      `json:"exportedAt"`
	Browser    string      `json:"browser"`
	Containers []Container `json:"containers"`
	Rules      []ExportRule `json:"rules"`
	TotalRules int         `json:"totalRules"`
}

type ExportRule struct {
	Site          string `json:"site"`
	UserContextID int    `json:"userContextId"`
	Container     string `json:"container"`
	NeverAsk      bool   `json:"neverAsk"`
}

type ImportResult struct {
	Added   int `json:"added"`
	Updated int `json:"updated"`
	Skipped int `json:"skipped"`
}

func discoverContext() (*cmdContext, error) {
	profileDir, _, err := discoverProfile()
	if err != nil {
		return nil, err
	}

	uuid, err := findMACUUID(profileDir)
	if err != nil {
		return nil, err
	}

	idbDir := buildIDBPath(profileDir, uuid)
	dbPath, err := findSQLiteFile(idbDir)
	if err != nil {
		return nil, err
	}

	return &cmdContext{dbPath: dbPath, profileDir: profileDir}, nil
}

func detectBrowser(profileDir string) string {
	lower := filepath.ToSlash(profileDir)
	if strings.Contains(lower, "zen") {
		return "zen"
	}
	return "firefox"
}

func handleList(ctx *cmdContext) Response {
	rules, _, err := readRules(ctx.dbPath)
	if err != nil {
		return Response{OK: false, Error: fmt.Sprintf("reading rules: %v", err)}
	}

	containers, err := readContainers(ctx.profileDir)
	if err != nil {
		return Response{OK: false, Error: fmt.Sprintf("reading containers: %v", err)}
	}

	return Response{OK: true, Data: map[string]any{
		"rules":      rules,
		"containers": containers,
	}}
}

func handleAdd(ctx *cmdContext, site string, userContextID int, neverAsk bool) Response {
	_, blobs, err := readRules(ctx.dbPath)
	if err != nil {
		return Response{OK: false, Error: fmt.Sprintf("reading rules: %v", err)}
	}

	if err := addRule(ctx.dbPath, site, userContextID, neverAsk, blobs); err != nil {
		return Response{OK: false, Error: fmt.Sprintf("adding rule: %v", err)}
	}

	return Response{OK: true, Data: map[string]any{
		"rule": SiteRule{Site: site, UserContextID: userContextID, NeverAsk: neverAsk},
	}}
}

func handleUpdate(ctx *cmdContext, site string, userContextID int, neverAsk bool) Response {
	_, blobs, err := readRules(ctx.dbPath)
	if err != nil {
		return Response{OK: false, Error: fmt.Sprintf("reading rules: %v", err)}
	}

	if err := updateRule(ctx.dbPath, site, userContextID, neverAsk, blobs); err != nil {
		return Response{OK: false, Error: fmt.Sprintf("updating rule: %v", err)}
	}

	return Response{OK: true, Data: map[string]any{
		"rule": SiteRule{Site: site, UserContextID: userContextID, NeverAsk: neverAsk},
	}}
}

func handleDelete(ctx *cmdContext, site string) Response {
	if err := deleteRule(ctx.dbPath, site); err != nil {
		return Response{OK: false, Error: fmt.Sprintf("deleting rule: %v", err)}
	}

	return Response{OK: true, Data: map[string]any{"deleted": true}}
}

func handleExport(ctx *cmdContext) Response {
	rules, _, err := readRules(ctx.dbPath)
	if err != nil {
		return Response{OK: false, Error: fmt.Sprintf("reading rules: %v", err)}
	}

	containers, err := readContainers(ctx.profileDir)
	if err != nil {
		return Response{OK: false, Error: fmt.Sprintf("reading containers: %v", err)}
	}

	cMap := containerMap(containers)
	exportRules := make([]ExportRule, len(rules))
	for i, r := range rules {
		containerName := fmt.Sprintf("Unknown (%d)", r.UserContextID)
		if c, ok := cMap[r.UserContextID]; ok {
			containerName = c.Name
		}
		exportRules[i] = ExportRule{
			Site:          r.Site,
			UserContextID: r.UserContextID,
			Container:     containerName,
			NeverAsk:      r.NeverAsk,
		}
	}

	export := ExportData{
		Version:    1,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Browser:    detectBrowser(ctx.profileDir),
		Containers: containers,
		Rules:      exportRules,
		TotalRules: len(exportRules),
	}

	return Response{OK: true, Data: export}
}

func handleImport(ctx *cmdContext, importRules []RuleSpec) Response {
	existing, blobs, err := readRules(ctx.dbPath)
	if err != nil {
		return Response{OK: false, Error: fmt.Sprintf("reading rules: %v", err)}
	}

	existingMap := make(map[string]bool)
	for _, r := range existing {
		existingMap[r.Site] = true
	}

	var added, updated, skipped int
	for _, r := range importRules {
		if r.Site == "" || r.UserContextID <= 0 {
			skipped++
			continue
		}

		err := addRule(ctx.dbPath, r.Site, r.UserContextID, r.NeverAsk, blobs)
		if err != nil {
			skipped++
			continue
		}

		if existingMap[r.Site] {
			updated++
		} else {
			added++
		}
	}

	return Response{OK: true, Data: ImportResult{Added: added, Updated: updated, Skipped: skipped}}
}

func dispatch(req Request) Response {
	ctx, err := discoverContext()
	if err != nil {
		return Response{OK: false, Error: err.Error()}
	}

	neverAsk := true
	if req.NeverAsk != nil {
		neverAsk = *req.NeverAsk
	}

	switch req.Cmd {
	case "list":
		return handleList(ctx)
	case "add":
		return handleAdd(ctx, req.Site, req.UserContextID, neverAsk)
	case "update":
		return handleUpdate(ctx, req.Site, req.UserContextID, neverAsk)
	case "delete":
		return handleDelete(ctx, req.Site)
	case "export":
		return handleExport(ctx)
	case "import":
		return handleImport(ctx, req.Rules)
	default:
		return Response{OK: false, Error: fmt.Sprintf("unknown command: %s", req.Cmd)}
	}
}
```

- [ ] **Step 4: Implement main.go**

```go
// host/main.go
package main

import (
	"encoding/json"
	"log"
	"os"
)

func main() {
	log.SetOutput(os.Stderr)

	msg, err := readMessage(os.Stdin)
	if err != nil {
		log.Fatalf("reading message: %v", err)
	}

	var req Request
	if err := json.Unmarshal(msg, &req); err != nil {
		resp := Response{OK: false, Error: "invalid JSON request"}
		writeMessage(os.Stdout, resp)
		return
	}

	resp := dispatch(req)

	if err := writeMessage(os.Stdout, resp); err != nil {
		log.Fatalf("writing response: %v", err)
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd host && go test -run "TestExportCommand|TestImportCommand" -v
```

Expected: all PASS.

- [ ] **Step 6: Build the binary**

```bash
cd host && go build -o ../dist/fmac-rules-manager-host .
```

Expected: binary created at `dist/fmac-rules-manager-host`.

- [ ] **Step 7: Commit**

```bash
git add host/commands.go host/commands_test.go host/main.go
git commit -m "feat: command dispatcher with export/import and main entry point"
```

---

### Task 10: Native Messaging Host Manifests

**Files:**
- Create: `host-manifest/fmac_rules_manager.darwin.json`
- Create: `host-manifest/fmac_rules_manager.linux.json`
- Create: `host-manifest/install.sh`

- [ ] **Step 1: Create macOS manifest**

```json
{
  "name": "fmac_rules_manager",
  "description": "Native host for Container Rules Manager — reads/writes MAC site rules",
  "path": "/usr/local/bin/fmac-rules-manager-host",
  "type": "stdio",
  "allowed_extensions": ["fmac-rules-manager@sarcasticbird.com"]
}
```

- [ ] **Step 2: Create Linux manifest**

```json
{
  "name": "fmac_rules_manager",
  "description": "Native host for Container Rules Manager — reads/writes MAC site rules",
  "path": "/usr/local/bin/fmac-rules-manager-host",
  "type": "stdio",
  "allowed_extensions": ["fmac-rules-manager@sarcasticbird.com"]
}
```

- [ ] **Step 3: Create install script**

```bash
#!/bin/bash
set -euo pipefail

BINARY_NAME="fmac-rules-manager-host"
MANIFEST_NAME="fmac_rules_manager.json"

# Detect OS
case "$(uname -s)" in
  Darwin) OS="darwin" ;;
  Linux)  OS="linux" ;;
  *)      echo "Unsupported OS"; exit 1 ;;
esac

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Copy binary
echo "Installing $BINARY_NAME to /usr/local/bin/"
sudo cp "$SCRIPT_DIR/../dist/$BINARY_NAME" "/usr/local/bin/$BINARY_NAME"
sudo chmod 755 "/usr/local/bin/$BINARY_NAME"

# Install manifest for each browser
install_manifest() {
  local dir="$1"
  local browser="$2"
  mkdir -p "$dir"
  cp "$SCRIPT_DIR/${MANIFEST_NAME%.json}.$OS.json" "$dir/$MANIFEST_NAME"
  echo "Installed manifest for $browser: $dir/$MANIFEST_NAME"
}

if [ "$OS" = "darwin" ]; then
  install_manifest "$HOME/Library/Application Support/Mozilla/NativeMessagingHosts" "Firefox"
  if [ -d "$HOME/Library/Application Support/zen" ]; then
    install_manifest "$HOME/Library/Application Support/zen/NativeMessagingHosts" "Zen"
  fi
else
  install_manifest "$HOME/.mozilla/native-messaging-hosts" "Firefox"
  if [ -d "$HOME/.zen" ]; then
    install_manifest "$HOME/.zen/native-messaging-hosts" "Zen"
  fi
fi

echo "Done. Restart your browser for the extension to detect the host."
```

- [ ] **Step 4: Make install script executable and commit**

```bash
chmod +x host-manifest/install.sh
git add host-manifest/
git commit -m "feat: native messaging host manifests and install script"
```

---

### Task 11: Extension — Background Script

**Files:**
- Create: `src/background.js`

- [ ] **Step 1: Implement background.js**

```javascript
const HOST_NAME = "fmac_rules_manager";
const MAC_ID = "@testpilot-containers";

let port = null;
let pendingCallbacks = [];

function connectHost() {
  if (port) return port;
  port = browser.runtime.connectNative(HOST_NAME);

  port.onMessage.addListener((response) => {
    const cb = pendingCallbacks.shift();
    if (cb) cb(response);
  });

  port.onDisconnect.addListener((p) => {
    const error = p.error ? p.error.message : browser.runtime.lastError?.message || "disconnected";
    port = null;
    while (pendingCallbacks.length > 0) {
      const cb = pendingCallbacks.shift();
      cb({ ok: false, error: `Host disconnected: ${error}` });
    }
  });

  return port;
}

function sendToHost(message) {
  return new Promise((resolve) => {
    try {
      const p = connectHost();
      pendingCallbacks.push(resolve);
      p.postMessage(message);
    } catch (err) {
      resolve({
        ok: false,
        error: `Could not connect to native host. Is it installed? (${err.message})`,
      });
    }
  });
}

async function reloadMAC() {
  try {
    await browser.management.setEnabled(MAC_ID, false);
    await new Promise((r) => setTimeout(r, 500));
    await browser.management.setEnabled(MAC_ID, true);
    return { reloaded: true };
  } catch (err) {
    return { reloaded: false, warning: `Could not reload MAC: ${err.message}. Restart browser to apply changes.` };
  }
}

const WRITE_COMMANDS = new Set(["add", "update", "delete", "import"]);

browser.runtime.onMessage.addListener((message, sender, sendResponse) => {
  (async () => {
    const response = await sendToHost(message);

    if (response.ok && WRITE_COMMANDS.has(message.cmd)) {
      const reload = await reloadMAC();
      if (response.data && typeof response.data === "object") {
        response.data._macReload = reload;
      }
    }

    sendResponse(response);
  })();

  return true;
});
```

- [ ] **Step 2: Commit**

```bash
git add src/background.js
git commit -m "feat: background script with native messaging relay and MAC reload"
```

---

### Task 12: Extension — Options Page HTML + CSS

**Files:**
- Create: `src/options.html`
- Create: `src/options.css`

- [ ] **Step 1: Create options.html**

```html
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Container Rules Manager</title>
  <link rel="stylesheet" href="options.css">
</head>
<body>
  <header>
    <h1>Container Rules Manager</h1>
    <div class="header-actions">
      <button id="btn-export" title="Export rules as JSON">Export</button>
      <label class="btn" id="btn-import-label" title="Import rules from JSON">
        Import
        <input type="file" id="btn-import" accept=".json" hidden>
      </label>
    </div>
  </header>

  <div id="banner" class="banner hidden"></div>

  <section id="add-rule-section">
    <input type="text" id="input-site" placeholder="hostname (e.g. github.com)">
    <select id="select-container"></select>
    <label class="checkbox-label">
      <input type="checkbox" id="check-neverask" checked>
      Always open
    </label>
    <button id="btn-add">Add Rule</button>
  </section>

  <section id="table-section">
    <div class="table-controls">
      <input type="text" id="input-filter" placeholder="Filter by site or container...">
      <div class="bulk-actions hidden" id="bulk-actions">
        <span id="selected-count">0 selected</span>
        <select id="bulk-reassign-container"></select>
        <button id="btn-bulk-reassign">Reassign</button>
        <button id="btn-bulk-delete" class="danger">Delete</button>
      </div>
    </div>

    <table id="rules-table">
      <thead>
        <tr>
          <th class="col-check"><input type="checkbox" id="check-all"></th>
          <th class="col-site sortable" data-sort="site">Site</th>
          <th class="col-container sortable" data-sort="container">Container</th>
          <th class="col-neverask">Always</th>
          <th class="col-actions">Actions</th>
        </tr>
      </thead>
      <tbody id="rules-body"></tbody>
    </table>

    <div id="empty-state" class="hidden">
      <p>No site rules found. Add a rule above or import from a JSON file.</p>
    </div>
  </section>

  <div id="loading" class="loading">Loading rules...</div>

  <script src="options.js"></script>
</body>
</html>
```

- [ ] **Step 2: Create options.css**

```css
* { box-sizing: border-box; margin: 0; padding: 0; }

body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  max-width: 960px;
  margin: 0 auto;
  padding: 24px;
  color: #1a1a2e;
  background: #f8f9fa;
}

header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

h1 { font-size: 1.5rem; font-weight: 600; }

.header-actions { display: flex; gap: 8px; }

button, .btn {
  padding: 8px 16px;
  border: 1px solid #d0d0d0;
  border-radius: 6px;
  background: #fff;
  cursor: pointer;
  font-size: 0.875rem;
  font-family: inherit;
  transition: background 0.15s;
}

button:hover, .btn:hover { background: #f0f0f0; }

button.danger { color: #d32f2f; border-color: #d32f2f; }
button.danger:hover { background: #fde8e8; }

button.primary { background: #1a73e8; color: #fff; border-color: #1a73e8; }
button.primary:hover { background: #1557b0; }

.banner {
  padding: 12px 16px;
  border-radius: 6px;
  margin-bottom: 16px;
  font-size: 0.875rem;
}

.banner.error { background: #fde8e8; color: #d32f2f; border: 1px solid #f5c6c6; }
.banner.success { background: #e8f5e9; color: #2e7d32; border: 1px solid #c8e6c9; }
.banner.warning { background: #fff8e1; color: #f57f17; border: 1px solid #ffecb3; }

.hidden { display: none !important; }

#add-rule-section {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
  align-items: center;
}

input[type="text"], select {
  padding: 8px 12px;
  border: 1px solid #d0d0d0;
  border-radius: 6px;
  font-size: 0.875rem;
  font-family: inherit;
}

#input-site { flex: 1; }

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 0.875rem;
  white-space: nowrap;
}

.table-controls {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  gap: 12px;
}

#input-filter { flex: 1; }

.bulk-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

table {
  width: 100%;
  border-collapse: collapse;
  background: #fff;
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 1px 3px rgba(0,0,0,0.08);
}

th {
  text-align: left;
  padding: 10px 12px;
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: #666;
  background: #fafafa;
  border-bottom: 1px solid #e0e0e0;
}

th.sortable { cursor: pointer; user-select: none; }
th.sortable:hover { color: #1a73e8; }

td {
  padding: 10px 12px;
  border-bottom: 1px solid #f0f0f0;
  font-size: 0.875rem;
}

tr:last-child td { border-bottom: none; }
tr:hover td { background: #fafafa; }
tr.selected td { background: #e3f2fd; }

.col-check { width: 40px; text-align: center; }
.col-neverask { width: 70px; text-align: center; }
.col-actions { width: 100px; text-align: right; }
.col-container { width: 180px; }

.container-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 0.8125rem;
}

.container-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
}

.action-btn {
  background: none;
  border: none;
  cursor: pointer;
  padding: 4px 8px;
  font-size: 0.875rem;
  color: #666;
}

.action-btn:hover { color: #1a73e8; }
.action-btn.delete:hover { color: #d32f2f; }

.loading {
  text-align: center;
  padding: 48px;
  color: #666;
}

#empty-state {
  text-align: center;
  padding: 48px;
  color: #999;
}

.edit-select {
  padding: 4px 8px;
  border: 1px solid #1a73e8;
  border-radius: 4px;
  font-size: 0.8125rem;
}
```

- [ ] **Step 3: Commit**

```bash
git add src/options.html src/options.css
git commit -m "feat: options page HTML and CSS layout"
```

---

### Task 13: Extension — Options Page JavaScript

**Files:**
- Create: `src/options.js`

This is the core UI logic: fetching rules, rendering the table, handling CRUD, import/export, filtering, sorting, bulk actions.

- [ ] **Step 1: Implement options.js**

```javascript
const CONTAINER_COLORS = {
  blue: "#37adff", turquoise: "#00c79a", green: "#51cd00",
  yellow: "#ffcb00", orange: "#ff9f00", red: "#ff613d",
  pink: "#ff4bda", purple: "#af51f5", toolbar: "#7c7c7d",
};

let allRules = [];
let containers = [];
let containersByID = {};
let selectedSites = new Set();
let sortField = "site";
let sortAsc = true;

async function sendCommand(cmd) {
  return browser.runtime.sendMessage(cmd);
}

function showBanner(message, type) {
  const banner = document.getElementById("banner");
  banner.textContent = message;
  banner.className = `banner ${type}`;
  banner.classList.remove("hidden");
  if (type === "success") {
    setTimeout(() => banner.classList.add("hidden"), 3000);
  }
}

function hideBanner() {
  document.getElementById("banner").classList.add("hidden");
}

function populateContainerDropdowns() {
  const selects = [
    document.getElementById("select-container"),
    document.getElementById("bulk-reassign-container"),
  ];
  selects.forEach((sel) => {
    sel.innerHTML = "";
    containers.forEach((c) => {
      const opt = document.createElement("option");
      opt.value = c.userContextId;
      opt.textContent = c.name;
      sel.appendChild(opt);
    });
  });
}

function containerBadge(ctxId) {
  const c = containersByID[ctxId];
  const name = c ? c.name : `Unknown (${ctxId})`;
  const color = c ? CONTAINER_COLORS[c.color] || "#7c7c7d" : "#7c7c7d";
  return `<span class="container-badge"><span class="container-dot" style="background:${color}"></span>${name}</span>`;
}

function renderTable() {
  const filter = document.getElementById("input-filter").value.toLowerCase();
  const tbody = document.getElementById("rules-body");
  const emptyState = document.getElementById("empty-state");

  let filtered = allRules;
  if (filter) {
    filtered = allRules.filter((r) => {
      const cName = (containersByID[r.userContextId]?.name || "").toLowerCase();
      return r.site.toLowerCase().includes(filter) || cName.includes(filter);
    });
  }

  filtered.sort((a, b) => {
    let va, vb;
    if (sortField === "site") {
      va = a.site;
      vb = b.site;
    } else {
      va = (containersByID[a.userContextId]?.name || "").toLowerCase();
      vb = (containersByID[b.userContextId]?.name || "").toLowerCase();
    }
    const cmp = va < vb ? -1 : va > vb ? 1 : 0;
    return sortAsc ? cmp : -cmp;
  });

  tbody.innerHTML = "";

  if (filtered.length === 0) {
    emptyState.classList.remove("hidden");
    document.getElementById("rules-table").classList.add("hidden");
    return;
  }

  emptyState.classList.add("hidden");
  document.getElementById("rules-table").classList.remove("hidden");

  filtered.forEach((rule) => {
    const tr = document.createElement("tr");
    if (selectedSites.has(rule.site)) tr.classList.add("selected");

    tr.innerHTML = `
      <td class="col-check"><input type="checkbox" data-site="${rule.site}" ${selectedSites.has(rule.site) ? "checked" : ""}></td>
      <td class="col-site">${rule.site}</td>
      <td class="col-container">${containerBadge(rule.userContextId)}</td>
      <td class="col-neverask">${rule.neverAsk ? "Yes" : "No"}</td>
      <td class="col-actions">
        <button class="action-btn edit" data-site="${rule.site}" title="Edit">Edit</button>
        <button class="action-btn delete" data-site="${rule.site}" title="Delete">Del</button>
      </td>
    `;
    tbody.appendChild(tr);
  });

  updateBulkActions();
}

function updateBulkActions() {
  const bulk = document.getElementById("bulk-actions");
  const count = document.getElementById("selected-count");
  if (selectedSites.size > 0) {
    bulk.classList.remove("hidden");
    count.textContent = `${selectedSites.size} selected`;
  } else {
    bulk.classList.add("hidden");
  }
}

async function loadRules() {
  document.getElementById("loading").classList.remove("hidden");
  const resp = await sendCommand({ cmd: "list" });
  document.getElementById("loading").classList.add("hidden");

  if (!resp.ok) {
    showBanner(resp.error, "error");
    return;
  }

  allRules = resp.data.rules || [];
  containers = resp.data.containers || [];
  containersByID = {};
  containers.forEach((c) => { containersByID[c.userContextId] = c; });

  populateContainerDropdowns();
  renderTable();
}

async function handleAdd() {
  const site = document.getElementById("input-site").value.trim().toLowerCase();
  const ctxId = parseInt(document.getElementById("select-container").value);
  const neverAsk = document.getElementById("check-neverask").checked;

  if (!site) {
    showBanner("Please enter a hostname.", "error");
    return;
  }

  const resp = await sendCommand({ cmd: "add", site, userContextId: ctxId, neverAsk });
  if (!resp.ok) {
    showBanner(resp.error, "error");
    return;
  }

  document.getElementById("input-site").value = "";
  showMACReloadStatus(resp.data._macReload);
  await loadRules();
}

async function handleDelete(site) {
  const resp = await sendCommand({ cmd: "delete", site });
  if (!resp.ok) {
    showBanner(resp.error, "error");
    return;
  }
  showMACReloadStatus(resp.data._macReload);
  await loadRules();
}

async function handleEdit(site) {
  const rule = allRules.find((r) => r.site === site);
  if (!rule) return;

  const row = document.querySelector(`button.edit[data-site="${site}"]`).closest("tr");
  const containerCell = row.querySelector(".col-container");

  const select = document.createElement("select");
  select.className = "edit-select";
  containers.forEach((c) => {
    const opt = document.createElement("option");
    opt.value = c.userContextId;
    opt.textContent = c.name;
    if (c.userContextId === rule.userContextId) opt.selected = true;
    select.appendChild(opt);
  });

  containerCell.innerHTML = "";
  containerCell.appendChild(select);
  select.focus();

  const save = async () => {
    const newCtxId = parseInt(select.value);
    if (newCtxId !== rule.userContextId) {
      const resp = await sendCommand({ cmd: "update", site, userContextId: newCtxId, neverAsk: rule.neverAsk });
      if (!resp.ok) {
        showBanner(resp.error, "error");
      } else {
        showMACReloadStatus(resp.data._macReload);
      }
    }
    await loadRules();
  };

  select.addEventListener("change", save);
  select.addEventListener("blur", save);
}

function showMACReloadStatus(reload) {
  if (!reload) return;
  if (reload.reloaded) {
    showBanner("Rule saved. MAC reloaded.", "success");
  } else {
    showBanner(reload.warning, "warning");
  }
}

async function handleExport() {
  const resp = await sendCommand({ cmd: "export" });
  if (!resp.ok) {
    showBanner(resp.error, "error");
    return;
  }

  const json = JSON.stringify(resp.data, null, 2);
  const blob = new Blob([json], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const timestamp = new Date().toISOString().replace(/[:.]/g, "-").slice(0, 19);

  browser.downloads.download({
    url,
    filename: `container-rules-${timestamp}.json`,
    saveAs: true,
  });
}

async function handleImport(file) {
  const text = await file.text();
  let data;
  try {
    data = JSON.parse(text);
  } catch {
    showBanner("Invalid JSON file.", "error");
    return;
  }

  const rules = data.rules || data.siteMappings || [];
  if (rules.length === 0) {
    showBanner("No rules found in file.", "error");
    return;
  }

  const importRules = rules.map((r) => ({
    site: r.site,
    userContextId: r.userContextId,
    neverAsk: r.neverAsk !== undefined ? r.neverAsk : true,
  }));

  const resp = await sendCommand({ cmd: "import", rules: importRules });
  if (!resp.ok) {
    showBanner(resp.error, "error");
    return;
  }

  showBanner(`Imported: ${resp.data.added} added, ${resp.data.updated} updated, ${resp.data.skipped} skipped.`, "success");
  showMACReloadStatus(resp.data._macReload);
  await loadRules();
}

async function handleBulkDelete() {
  for (const site of selectedSites) {
    await sendCommand({ cmd: "delete", site });
  }
  selectedSites.clear();
  showBanner("Deleted selected rules.", "success");
  await loadRules();
}

async function handleBulkReassign() {
  const ctxId = parseInt(document.getElementById("bulk-reassign-container").value);
  for (const site of selectedSites) {
    const rule = allRules.find((r) => r.site === site);
    await sendCommand({ cmd: "update", site, userContextId: ctxId, neverAsk: rule?.neverAsk ?? true });
  }
  selectedSites.clear();
  showBanner("Reassigned selected rules.", "success");
  await loadRules();
}

// Event listeners
document.getElementById("btn-add").addEventListener("click", handleAdd);
document.getElementById("input-site").addEventListener("keydown", (e) => {
  if (e.key === "Enter") handleAdd();
});
document.getElementById("btn-export").addEventListener("click", handleExport);
document.getElementById("btn-import").addEventListener("change", (e) => {
  if (e.target.files[0]) handleImport(e.target.files[0]);
  e.target.value = "";
});
document.getElementById("input-filter").addEventListener("input", renderTable);
document.getElementById("btn-bulk-delete").addEventListener("click", handleBulkDelete);
document.getElementById("btn-bulk-reassign").addEventListener("click", handleBulkReassign);

document.getElementById("check-all").addEventListener("change", (e) => {
  if (e.target.checked) {
    allRules.forEach((r) => selectedSites.add(r.site));
  } else {
    selectedSites.clear();
  }
  renderTable();
});

document.getElementById("rules-body").addEventListener("click", (e) => {
  const btn = e.target.closest("button");
  if (btn?.classList.contains("edit")) {
    handleEdit(btn.dataset.site);
  } else if (btn?.classList.contains("delete")) {
    handleDelete(btn.dataset.site);
  }
});

document.getElementById("rules-body").addEventListener("change", (e) => {
  if (e.target.type === "checkbox") {
    const site = e.target.dataset.site;
    if (e.target.checked) {
      selectedSites.add(site);
    } else {
      selectedSites.delete(site);
    }
    renderTable();
  }
});

document.querySelectorAll("th.sortable").forEach((th) => {
  th.addEventListener("click", () => {
    const field = th.dataset.sort;
    if (sortField === field) {
      sortAsc = !sortAsc;
    } else {
      sortField = field;
      sortAsc = true;
    }
    renderTable();
  });
});

loadRules();
```

- [ ] **Step 2: Commit**

```bash
git add src/options.js
git commit -m "feat: options page JavaScript with CRUD, import/export, filtering, and bulk actions"
```

---

### Task 14: Build, Lint, and Manual Integration Test

**Files:** None new — verification only.

- [ ] **Step 1: Build Go binary**

```bash
cd host && go build -o ../dist/fmac-rules-manager-host . && echo "Binary built: $(file ../dist/fmac-rules-manager-host)"
```

- [ ] **Step 2: Run all Go tests**

```bash
cd host && go test -v ./...
```

Expected: all PASS.

- [ ] **Step 3: Run Go vet and format check**

```bash
cd host && go vet ./... && gofmt -l .
```

Expected: no issues, no unformatted files.

- [ ] **Step 4: Lint extension**

```bash
npm run lint
```

Expected: no errors (warnings about permissions are OK).

- [ ] **Step 5: Install native host locally**

```bash
./host-manifest/install.sh
```

- [ ] **Step 6: Test with web-ext**

```bash
npm run start
```

Open the extension options page. Verify:
- Rules table loads and shows existing MAC rules
- Filter input filters the table
- Add rule works (enter hostname, select container, click Add)
- Edit rule works (click Edit, change container)
- Delete rule works (click Del)
- Export downloads a JSON file
- Import loads a JSON file and adds/updates rules
- Bulk select + reassign/delete works

- [ ] **Step 7: Commit any fixes from testing**

```bash
git add -A
git commit -m "fix: adjustments from integration testing"
```

---

### Task 15: Run All Tests and Final Verification

- [ ] **Step 1: Full Go test suite**

```bash
cd host && go test -v -count=1 ./...
```

- [ ] **Step 2: Go vet + fmt**

```bash
cd host && go vet ./... && go fmt ./...
```

- [ ] **Step 3: web-ext lint**

```bash
npm run lint
```

- [ ] **Step 4: Build extension**

```bash
npm run build
```

Expected: `.zip` created in `dist/`.

- [ ] **Step 5: Build Go binary for release**

```bash
cd host && GOOS=darwin GOARCH=arm64 go build -o ../dist/fmac-rules-manager-host-darwin-arm64 .
cd host && GOOS=darwin GOARCH=amd64 go build -o ../dist/fmac-rules-manager-host-darwin-amd64 .
cd host && GOOS=linux GOARCH=amd64 go build -o ../dist/fmac-rules-manager-host-linux-amd64 .
```
