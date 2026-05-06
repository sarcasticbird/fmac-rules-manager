package main

import "fmt"

type cmdContext struct {
	dbPath     string
	profileDir string
}

type ExportData struct {
	Rules []SiteRule `json:"rules"`
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

	return Response{OK: true, Data: ExportData{Rules: rules}}
}

func handleImport(ctx *cmdContext, specs []RuleSpec, mode string) Response {
	existing, blobs, err := readRules(ctx.dbPath)
	if err != nil {
		return Response{OK: false, Error: fmt.Sprintf("reading rules: %v", err)}
	}

	res, err := importRules(ctx.dbPath, specs, existing, blobs, mode == "replace")
	if err != nil {
		return Response{OK: false, Error: fmt.Sprintf("import failed: %v", err)}
	}

	return Response{OK: true, Data: ImportResult{Added: res.added, Updated: res.updated, Skipped: res.skipped}}
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
		return handleImport(ctx, req.Rules, req.Mode)
	default:
		return Response{OK: false, Error: fmt.Sprintf("unknown command: %s", req.Cmd)}
	}
}
