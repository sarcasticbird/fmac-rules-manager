package main

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"
)

type cmdContext struct {
	dbPath     string
	profileDir string
}

type ExportData struct {
	Version    int          `json:"version"`
	ExportedAt string       `json:"exportedAt"`
	Browser    string       `json:"browser"`
	Containers []Container  `json:"containers"`
	Rules      []ExportRule `json:"rules"`
	TotalRules int          `json:"totalRules"`
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
	if site == "" || userContextID <= 0 {
		return Response{OK: false, Error: "site and userContextId are required"}
	}

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
	if site == "" || userContextID <= 0 {
		return Response{OK: false, Error: "site and userContextId are required"}
	}

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
			log.Printf("import: failed to add %s: %v", r.Site, err)
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
