package main

import (
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
