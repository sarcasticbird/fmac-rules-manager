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
