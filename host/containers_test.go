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
