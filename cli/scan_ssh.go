package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func scanSSHKeys() Category {
	var items []Item

	home, _ := os.UserHomeDir()
	sshDir := filepath.Join(home, ".ssh")

	entries, err := os.ReadDir(sshDir)
	if err != nil {
		return Category{
			ID: "security", Label: "SSH & Keys", Icon: "🔐",
			Desc:  "SSH keys, GPG, and authentication",
			Items: nil,
		}
	}

	for _, entry := range entries {
		name := entry.Name()
		// Look for private key files (no extension, or specific patterns)
		if strings.HasSuffix(name, ".pub") || name == "config" || name == "known_hosts" ||
			name == "authorized_keys" || strings.HasPrefix(name, ".") {
			continue
		}

		fullPath := filepath.Join(sshDir, name)
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			continue
		}

		// Read first line to detect key type
		lines := readFileLines(fullPath)
		if len(lines) == 0 {
			continue
		}

		keyType := "unknown"
		status := "active"
		systemNote := ""
		firstLine := lines[0]

		switch {
		case strings.Contains(firstLine, "OPENSSH PRIVATE KEY"):
			// Could be ed25519, rsa, etc. — check the .pub file
			pubContent := strings.Join(readFileLines(fullPath+".pub"), " ")
			if strings.Contains(pubContent, "ssh-ed25519") {
				keyType = "ed25519"
			} else if strings.Contains(pubContent, "ssh-rsa") {
				keyType = "RSA"
				systemNote = "Consider migrating to ed25519 for stronger security"
			} else if strings.Contains(pubContent, "ecdsa") {
				keyType = "ECDSA"
			}
		case strings.Contains(firstLine, "RSA PRIVATE KEY"):
			keyType = "RSA"
			status = "stale"
			systemNote = "Legacy RSA key format — ed25519 is preferred"
		case strings.Contains(firstLine, "DSA PRIVATE KEY"):
			keyType = "DSA"
			status = "stale"
			systemNote = "DSA is deprecated — migrate to ed25519"
		}

		age := daysSince(info.ModTime())
		ageStr := ""
		if age > 0 {
			if age > 365 {
				ageStr = fmt.Sprintf("%.1f years old", float64(age)/365.0)
			} else {
				ageStr = fmt.Sprintf("%d days old", age)
			}
		}

		if age > 365*3 && status == "active" {
			systemNote = fmt.Sprintf("%s — consider rotating", ageStr)
		}

		displayName := fmt.Sprintf("SSH key (%s)", keyType)
		detail := fmt.Sprintf("~/.ssh/%s", name)
		if ageStr != "" {
			detail += " · " + ageStr
		}

		items = append(items, Item{
			Name:       displayName,
			Detail:     detail,
			Status:     status,
			SystemNote: systemNote,
		})
	}

	// Check GPG
	if commandExists("gpg") {
		gpgKeys := runLines("gpg", "--list-secret-keys", "--keyid-format", "long")
		for _, line := range gpgKeys {
			if strings.Contains(line, "sec ") {
				// Extract key ID
				parts := strings.Fields(line)
				for _, p := range parts {
					if strings.Contains(p, "/") {
						keyParts := strings.Split(p, "/")
						if len(keyParts) > 1 {
							items = append(items, Item{
								Name:       "GPG key",
								Detail:     keyParts[1],
								Status:     "active",
								SystemNote: "Commit signing key",
							})
						}
					}
				}
			}
		}
	}

	// Check 1Password SSH agent
	if fileExists("~/Library/Group Containers/2BUA8C4S2C.com.1password/t/agent.sock") {
		items = append(items, Item{
			Name:       "1Password SSH Agent",
			Detail:     "Active",
			Status:     "active",
			SystemNote: "Managing SSH keys via 1Password",
		})
	}

	return Category{
		ID:    "security",
		Label: "SSH & Keys",
		Icon:  "🔐",
		Desc:  "Keys, GPG, and authentication",
		Items: items,
	}
}
