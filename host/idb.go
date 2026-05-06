package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

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
