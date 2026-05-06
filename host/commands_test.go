package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExportCommand(t *testing.T) {
	dbPath := createTestDB(t)
	profileDir := t.TempDir()

	ctx := &cmdContext{dbPath: dbPath, profileDir: profileDir}
	resp := handleExport(ctx)

	if !resp.OK {
		t.Fatalf("export failed: %s", resp.Error)
	}

	data, _ := json.Marshal(resp.Data)
	var export ExportData
	json.Unmarshal(data, &export)

	if len(export.Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(export.Rules))
	}
}

func TestImportCommand(t *testing.T) {
	dbPath := createTestDB(t)
	profileDir := t.TempDir()

	ctx := &cmdContext{dbPath: dbPath, profileDir: profileDir}

	importRules := []RuleSpec{
		{Site: "new-site.com", UserContextID: 6, NeverAsk: true},
		{Site: "github.com", UserContextID: 3, NeverAsk: true},
	}

	resp := handleImport(ctx, importRules, "merge")
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
