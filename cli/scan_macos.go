package main

import (
	"fmt"
	"strings"
)

type defaultCheck struct {
	name   string
	domain string
	key    string
	cmd    string // the command to set this value
}

func scanMacOSDefaults() Category {
	checks := []defaultCheck{
		{"Dock auto-hide", "com.apple.dock", "autohide", "defaults write com.apple.dock autohide -bool true"},
		{"Dock size", "com.apple.dock", "tilesize", "defaults write com.apple.dock tilesize -integer %s"},
		{"Dock magnification", "com.apple.dock", "magnification", "defaults write com.apple.dock magnification -bool true"},
		{"Show hidden files", "com.apple.finder", "AppleShowAllFiles", "defaults write com.apple.finder AppleShowAllFiles -bool true"},
		{"Show file extensions", "NSGlobalDomain", "AppleShowAllExtensions", "defaults write NSGlobalDomain AppleShowAllExtensions -bool true"},
		{"Show path bar", "com.apple.finder", "ShowPathbar", "defaults write com.apple.finder ShowPathbar -bool true"},
		{"Screenshots location", "com.apple.screencapture", "location", "defaults write com.apple.screencapture location %s"},
		{"Screenshot format", "com.apple.screencapture", "type", "defaults write com.apple.screencapture type %s"},
		{"Key repeat rate", "NSGlobalDomain", "KeyRepeat", "defaults write NSGlobalDomain KeyRepeat -int %s"},
		{"Initial key repeat", "NSGlobalDomain", "InitialKeyRepeat", "defaults write NSGlobalDomain InitialKeyRepeat -int %s"},
		{"Tap to click", "com.apple.AppleMultitouchTrackpad", "Clicking", "defaults write com.apple.AppleMultitouchTrackpad Clicking -bool true"},
		{"Scroll direction natural", "NSGlobalDomain", "com.apple.swipescrolldirection", "defaults write NSGlobalDomain com.apple.swipescrolldirection -bool true"},
		{"Reduce motion", "com.apple.universalaccess", "reduceMotion", "defaults write com.apple.universalaccess reduceMotion -bool true"},
		{"Dark mode", "NSGlobalDomain", "AppleInterfaceStyle", ""},
	}

	var items []Item

	for _, check := range checks {
		value := run("defaults", "read", check.domain, check.key)
		if value == "" {
			continue // Not set or default
		}

		// Format the command with the actual value
		cmd := check.cmd
		if strings.Contains(cmd, "%s") {
			cmd = fmt.Sprintf(cmd, value)
		}

		items = append(items, Item{
			Name:       check.name,
			Detail:     value,
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
