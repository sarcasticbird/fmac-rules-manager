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
