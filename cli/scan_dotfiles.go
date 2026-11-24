package main

import (
	"fmt"
	"os"
	"strings"
)

func scanDotfiles() Category {
	dotfiles := []struct {
		name string
		path string
		desc string // extra context to look for
	}{
		{".zshrc", "~/.zshrc", ""},
		{".zprofile", "~/.zprofile", ""},
		{".zshenv", "~/.zshenv", ""},
		{".zlogin", "~/.zlogin", ""},
		{".bash_profile", "~/.bash_profile", ""},
		{".bashrc", "~/.bashrc", ""},
		{".profile", "~/.profile", ""},
		{".gitconfig", "~/.gitconfig", ""},
		{".npmrc", "~/.npmrc", ""},
		{".yarnrc", "~/.yarnrc", ""},
		{".ssh/config", "~/.ssh/config", ""},
		{".tmux.conf", "~/.tmux.conf", ""},
		{".vimrc", "~/.vimrc", ""},
		{".config/nvim/init.lua", "~/.config/nvim/init.lua", ""},
		{".config/nvim/init.vim", "~/.config/nvim/init.vim", ""},
		{".editorconfig", "~/.editorconfig", ""},
		{".hushlogin", "~/.hushlogin", ""},
		{".wgetrc", "~/.wgetrc", ""},
		{".curlrc", "~/.curlrc", ""},
		{".config/starship.toml", "~/.config/starship.toml", ""},
		{".docker/config.json", "~/.docker/config.json", ""},
	}

	var items []Item
	currentShell := os.Getenv("SHELL")
	hasNvimConfig := fileExists("~/.config/nvim/init.lua") || fileExists("~/.config/nvim/init.vim")

	for _, df := range dotfiles {
		if !fileExists(df.path) {
			continue
		}

		lines := countFileLines(df.path)
		days := daysSince(fileModTime(df.path))

		// "stale" means probably a leftover nothing reads anymore, not just "old":
		// a config that works rarely needs editing.
		status := "active"
		systemNote := ""
		isBash := df.name == ".bashrc" || df.name == ".bash_profile" || df.name == ".profile"
		switch {
		case isBash && strings.HasSuffix(currentShell, "zsh"):
			status = "stale"
			systemNote = "Your shell is zsh, which doesn't read this; likely a leftover (some installers still write here)"
		case df.name == ".vimrc" && hasNvimConfig:
			status = "stale"
			systemNote = "Neovim has its own config, so this .vimrc may be legacy"
		case df.name == ".yarnrc" && !commandExists("yarn"):
			status = "stale"
			systemNote = "yarn isn't on your PATH, so nothing reads this"
		default:
			systemNote = analyzeDotfile(df.name, df.path, lines)
		}
		if days > 365 {
			systemNote += fmt.Sprintf(" · last edited %.1f years ago", float64(days)/365)
		}

		detail := fmt.Sprintf("~/%s", df.name)
		if lines > 0 {
			detail = fmt.Sprintf("~/%s · %d lines", df.name, lines)
		}

		items = append(items, Item{
			Name:       df.name,
			Detail:     detail,
			Status:     status,
			SystemNote: systemNote,
		})
	}

	return Category{
		ID:    "dotfiles",
		Label: "Dotfiles & Shell",
		Icon:  "⚙️",
		Desc:  "Shell configs, aliases, and environment setup",
		Items: items,
	}
}

// analyzeDotfile extracts useful info from file contents
func analyzeDotfile(name, path string, lines int) string {
	content := strings.Join(readFileLines(path), "\n")
	var notes []string

	switch {
	case name == ".zshrc":
		if strings.Contains(content, "oh-my-zsh") {
			notes = append(notes, "oh-my-zsh")
		}
		if strings.Contains(content, "starship") {
			notes = append(notes, "starship prompt")
		}
		aliasCount := strings.Count(content, "alias ")
		if aliasCount > 0 {
			notes = append(notes, fmt.Sprintf("%d aliases", aliasCount))
		}
		if strings.Contains(content, "nvm") {
			notes = append(notes, "nvm init")
		}
		if strings.Contains(content, "pyenv") {
			notes = append(notes, "pyenv init")
		}
		if strings.Contains(content, "rbenv") {
			notes = append(notes, "rbenv init")
		}
	case name == ".gitconfig":
		aliasCount := 0
		inAlias := false
		for _, line := range readFileLines(path) {
			if strings.Contains(line, "[alias]") {
				inAlias = true
				continue
			}
			if strings.HasPrefix(line, "[") {
				inAlias = false
			}
			if inAlias && strings.Contains(line, "=") {
				aliasCount++
			}
		}
		if aliasCount > 0 {
			notes = append(notes, fmt.Sprintf("%d aliases", aliasCount))
		}
		if strings.Contains(content, "delta") {
			notes = append(notes, "delta pager")
		}
		if strings.Contains(content, "gpgsign = true") {
			notes = append(notes, "GPG signing")
		}
	case name == ".ssh/config":
		hostCount := strings.Count(content, "Host ")
		if hostCount > 0 {
			notes = append(notes, fmt.Sprintf("%d hosts configured", hostCount))
		}
	case name == ".npmrc":
		if strings.Contains(content, "save-exact") {
			notes = append(notes, "save-exact enabled")
		}
		if strings.Contains(content, "registry") {
			notes = append(notes, "custom registry")
		}
	case name == ".tmux.conf":
		if strings.Contains(content, "tpm") || strings.Contains(content, "tmux-plugins") {
			notes = append(notes, "TPM plugins")
		}
	case name == ".hushlogin":
		return "Suppresses terminal MOTD"
	case name == ".editorconfig":
		if strings.Contains(content, "indent_size") {
			notes = append(notes, "global editor defaults")
		}
	}

	if len(notes) > 0 {
		return strings.Join(notes, " · ")
	}
	return fmt.Sprintf("%d lines", lines)
}
