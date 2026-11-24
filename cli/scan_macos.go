package main

import (
	"fmt"
	"strings"
)

type defaultCheck struct {
	name   string
	domain string
	key    string
	kind   string // bool, int, float, string: the type `defaults write` needs
}

func scanMacOSDefaults() Category {
	checks := []defaultCheck{
		{"Dock auto-hide", "com.apple.dock", "autohide", "bool"},
		{"Dock size", "com.apple.dock", "tilesize", "int"},
		{"Dock magnification", "com.apple.dock", "magnification", "bool"},
		{"Show hidden files", "com.apple.finder", "AppleShowAllFiles", "bool"},
		{"Show file extensions", "NSGlobalDomain", "AppleShowAllExtensions", "bool"},
		{"Show path bar", "com.apple.finder", "ShowPathbar", "bool"},
		{"Screenshots location", "com.apple.screencapture", "location", "string"},
		{"Screenshot format", "com.apple.screencapture", "type", "string"},
		{"Key repeat rate", "NSGlobalDomain", "KeyRepeat", "int"},
		{"Initial key repeat", "NSGlobalDomain", "InitialKeyRepeat", "int"},
		{"Tap to click", "com.apple.AppleMultitouchTrackpad", "Clicking", "bool"},
		{"Scroll direction natural", "NSGlobalDomain", "com.apple.swipescrolldirection", "bool"},
		{"Reduce motion", "com.apple.universalaccess", "reduceMotion", "bool"},
		{"Dark mode", "NSGlobalDomain", "AppleInterfaceStyle", "string"},
	}

	var items []Item

	for _, check := range checks {
		value := run("defaults", "read", check.domain, check.key)
		if value == "" {
			continue // Not set or default
		}

		// The playbook replays these, so the command must write back the value read here.
		arg := value
		detail := value
		switch check.kind {
		case "bool":
			on := value == "1" || strings.EqualFold(value, "true")
			arg = fmt.Sprint(on)
			detail = map[bool]string{true: "on", false: "off"}[on]
		case "string":
			arg = shellQuote(value)
		}
		cmd := fmt.Sprintf("defaults write %s %s -%s %s", check.domain, check.key, check.kind, arg)

		items = append(items, Item{
			Name:       check.name,
			Detail:     detail,
			Status:     "active",
			SystemNote: cmd,
		})
	}

	return Category{
		ID:    "macos",
		Label: "macOS Prefs",
		Icon:  "🍎",
		Desc:  "System defaults and customizations",
		Items: items,
	}
}

// shellQuote single-quotes a value for a bash command line when it needs it.
func shellQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n'\"\\$`!*?[](){}<>|&;#~") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
