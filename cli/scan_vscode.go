package main

import (
	"fmt"
	"strings"
)

func scanVSCode() Category {
	if !commandExists("code") {
		return Category{
			ID: "vscode", Label: "VS Code", Icon: "💎",
			Desc:  "VS Code CLI not found — install 'code' command from VS Code",
			Items: nil,
		}
	}

	var items []Item

	// Get installed extensions
	extensions := runLines("code", "--list-extensions", "--show-versions")
	for _, ext := range extensions {
		// Format: publisher.name@version
		parts := strings.SplitN(ext, "@", 2)
		id := parts[0]
		version := ""
		if len(parts) > 1 {
			version = parts[1]
		}

		// Extract a display name from the ID
		nameParts := strings.SplitN(id, ".", 2)
		displayName := id
		if len(nameParts) > 1 {
			displayName = nameParts[1]
			// Convert kebab-case to title
			displayName = strings.ReplaceAll(displayName, "-", " ")
			displayName = strings.Title(displayName)
		}

		status := "active"
		systemNote := ""

		// Check for known deprecated/built-in extensions
		lowerID := strings.ToLower(id)
		switch {
		case strings.Contains(lowerID, "bracket-pair"):
			status = "stale"
			systemNote = "This feature is now built into VS Code natively"
		case strings.Contains(lowerID, "trailing-spaces") && isVSCodeSettingEnabled("files.trimTrailingWhitespace"):
			status = "stale"
			systemNote = "VS Code has built-in trailing whitespace trimming"
		}

		detail := id
		if version != "" {
			detail = fmt.Sprintf("%s@%s", id, version)
		}

		items = append(items, Item{
			Name:       displayName,
			Detail:     detail,
			Status:     status,
			SystemNote: systemNote,
		})
	}

	return Category{
		ID:    "vscode",
		Label: "VS Code",
		Icon:  "💎",
		Desc:  "Extensions and editor configuration",
		Items: items,
	}
}

// isVSCodeSettingEnabled checks if a VS Code setting is true in user settings
func isVSCodeSettingEnabled(key string) bool {
	settingsPath := "~/Library/Application Support/Code/User/settings.json"
	lines := readFileLines(settingsPath)
	content := strings.Join(lines, "\n")
	return strings.Contains(content, fmt.Sprintf(`"%s": true`, key))
}
