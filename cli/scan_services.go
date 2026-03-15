package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func scanServices() Category {
	var items []Item

	// Homebrew services
	if commandExists("brew") {
		lines := runLines("brew", "services", "list")
		for _, line := range lines {
			if strings.HasPrefix(line, "Name") {
				continue // header
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			name := fields[0]
			rawStatus := fields[1]

			status := "running"
			if rawStatus != "started" {
				status = "active" // installed but not running
			}

			systemNote := "Brew service"
			if rawStatus == "started" {
				systemNote = "Brew service — auto-starts on boot"
			}

			// Try to find port
			port := guessServicePort(name)
			detail := ""
			if port > 0 {
				detail = fmt.Sprintf("port %d", port)
			} else {
				detail = rawStatus
			}

			items = append(items, Item{
				Name:       name,
				Detail:     detail,
				Status:     status,
				SystemNote: systemNote,
			})
		}
	}

	// Login items (modern launchctl approach)
	loginItems := runLines("osascript", "-e",
		`tell application "System Events" to get the name of every login item`)
	if len(loginItems) > 0 {
		// osascript returns comma-separated
		for _, line := range loginItems {
			apps := strings.Split(line, ", ")
			for _, app := range apps {
				app = strings.TrimSpace(app)
				if app == "" {
					continue
				}
				items = append(items, Item{
					Name:       app,
					Detail:     "Login item",
					Status:     "running",
					SystemNote: "Starts at login",
				})
			}
		}
	}

	// User Launch Agents
	home, _ := os.UserHomeDir()
	agentDir := filepath.Join(home, "Library", "LaunchAgents")
	agents, err := os.ReadDir(agentDir)
	if err == nil {
		for _, entry := range agents {
			name := entry.Name()
			if !strings.HasSuffix(name, ".plist") || strings.HasPrefix(name, ".") {
				continue
			}

			// Check if it's loaded
			label := strings.TrimSuffix(name, ".plist")
			loaded := run("launchctl", "print", fmt.Sprintf("gui/%d/%s", os.Getuid(), label))

			status := "active"
			if loaded != "" {
				status = "running"
			}

			items = append(items, Item{
				Name:       label,
				Detail:     "Launch Agent",
				Status:     status,
				SystemNote: fmt.Sprintf("~/%s", filepath.Join("Library", "LaunchAgents", name)),
			})
		}
	}

	return Category{
		ID:    "services",
		Label: "Services",
		Icon:  "🟢",
		Desc:  "Background services and login items",
		Items: items,
	}
}

func guessServicePort(name string) int {
	portMap := map[string]int{
		"postgresql":    5432,
		"postgresql@14": 5432,
		"postgresql@15": 5432,
		"postgresql@16": 5432,
		"postgresql@17": 5432,
		"mysql":         3306,
		"mysql@8.0":     3306,
		"redis":         6379,
		"memcached":     11211,
		"mongodb":       27017,
		"nginx":         80,
		"httpd":         8080,
		"dnsmasq":       53,
		"rabbitmq":      5672,
		"minio":         9000,
		"elasticsearch": 9200,
	}

	for key, port := range portMap {
		if strings.Contains(strings.ToLower(name), key) {
			return port
		}
	}
	return 0
}
