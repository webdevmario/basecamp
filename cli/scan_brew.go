package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

type brewInfoEntry struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Versions struct {
		Stable string `json:"stable"`
	} `json:"versions"`
	Desc string `json:"desc"`
}

type brewCaskEntry struct {
	Token   string `json:"token"`
	Name    []string `json:"name"`
	Version string `json:"version"`
	Desc    string `json:"desc"`
}

func scanHomebrew() Category {
	if !commandExists("brew") {
		return Category{
			ID: "brew", Label: "Homebrew", Icon: "🍺",
			Desc:  "Homebrew is not installed",
			Items: nil,
		}
	}

	var items []Item

	// Get outdated packages for comparison
	outdatedMap := make(map[string]string) // name -> new version
	outdatedLines := runLines("brew", "outdated", "--verbose")
	for _, line := range outdatedLines {
		// Format: "package (installed) < new_version"
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			name := parts[0]
			// Try to extract the new version
			if idx := strings.Index(line, "< "); idx != -1 {
				outdatedMap[name] = strings.TrimSpace(line[idx+2:])
			} else {
				outdatedMap[name] = "newer available"
			}
		}
	}

	// Scan formulae
	formulaeRaw := run("brew", "list", "--formula", "--versions")
	for _, line := range strings.Split(formulaeRaw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		version := parts[len(parts)-1] // last version is the active one

		status := "current"
		systemNote := ""

		if newVer, ok := outdatedMap[name]; ok {
			status = "outdated"
			systemNote = fmt.Sprintf("Version %s available", newVer)
		}

		items = append(items, Item{
			Name:       name,
			Detail:     fmt.Sprintf("%s · formula", version),
			Status:     status,
			SystemNote: systemNote,
		})
	}

	// Scan casks
	caskRaw := run("brew", "list", "--cask", "--versions")
	for _, line := range strings.Split(caskRaw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		version := parts[len(parts)-1]

		status := "current"
		systemNote := ""

		if newVer, ok := outdatedMap[name]; ok {
			status = "outdated"
			systemNote = fmt.Sprintf("Version %s available", newVer)
		}

		// Try to get the display name
		displayName := name

		items = append(items, Item{
			Name:       displayName,
			Detail:     fmt.Sprintf("%s · cask", version),
			Status:     status,
			SystemNote: systemNote,
		})
	}

	// Try to get descriptions for items (batch, to avoid slowness)
	// This is optional — skip if brew info is too slow
	enrichBrewDescriptions(items)

	return Category{
		ID:    "brew",
		Label: "Homebrew",
		Icon:  "🍺",
		Desc:  "Formulae, casks, and taps",
		Items: items,
	}
}

func enrichBrewDescriptions(items []Item) {
	// Only enrich items that don't have a system note yet
	// Use brew info --json for batch lookup
	var formulaNames []string
	for _, item := range items {
		if strings.Contains(item.Detail, "formula") && item.SystemNote == "" {
			formulaNames = append(formulaNames, item.Name)
		}
	}

	if len(formulaNames) == 0 {
		return
	}

	// Batch lookup — brew info --json accepts multiple names
	args := append([]string{"info", "--json=v2"}, formulaNames...)
	raw := run("brew", args...)
	if raw == "" {
		return
	}

	var result struct {
		Formulae []struct {
			Name string `json:"name"`
			Desc string `json:"desc"`
		} `json:"formulae"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return
	}

	descMap := make(map[string]string)
	for _, f := range result.Formulae {
		if f.Desc != "" {
			descMap[f.Name] = f.Desc
		}
	}

	for i := range items {
		if desc, ok := descMap[items[i].Name]; ok && items[i].SystemNote == "" {
			items[i].SystemNote = desc
		}
	}
}

// Helper to check if brew JSON parsing is available
func _unusedBrewJSON() {
	_ = json.Unmarshal
	_ = brewInfoEntry{}
	_ = brewCaskEntry{}
}
