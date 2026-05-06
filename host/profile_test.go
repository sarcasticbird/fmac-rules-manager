package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseProfilesINI(t *testing.T) {
	dir := t.TempDir()
	ini := "[Install1234]\nDefault=Profiles/abc123.default-release\n\n[Profile0]\nName=default-release\nPath=Profiles/abc123.default-release\nDefault=1\n"
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
	ini := "[Profile0]\nName=default\nPath=Profiles/xyz.default\nDefault=1\n"
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
	prefs := "user_pref(\"extensions.webextensions.uuids\", \"{\\\"@testpilot-containers\\\":\\\"7d2af900-bdc2-447e-b8fe-bea337a8dfd2\\\",\\\"other\\\":\\\"abc\\\"}\");"
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
