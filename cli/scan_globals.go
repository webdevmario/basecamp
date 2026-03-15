package main

import (
	"fmt"
	"strings"
)

func scanGlobalPackages() Category {
	var items []Item

	// npm global packages
	if commandExists("npm") {
		npmGlobals := run("npm", "list", "-g", "--depth=0", "--json")
		if npmGlobals != "" {
			// Parse JSON to get package names and versions
			// Quick parse without full json unmarshal
			lines := runLines("npm", "list", "-g", "--depth=0", "--parseable", "--long")
			for _, line := range lines {
				// Format: /path/to/node_modules/pkg:pkg@version
				if !strings.Contains(line, ":") {
					continue
				}
				parts := strings.SplitN(line, ":", 2)
				if len(parts) < 2 {
					continue
				}
				pkgInfo := parts[1]
				atIdx := strings.LastIndex(pkgInfo, "@")
				if atIdx <= 0 {
					continue
				}
				name := pkgInfo[:atIdx]
				version := pkgInfo[atIdx+1:]

				// Skip npm itself
				if name == "npm" || name == "corepack" {
					continue
				}

				status := "current"
				systemNote := ""

				// Check for known deprecated packages
				systemNote = checkDeprecatedNPM(name)
				if systemNote != "" {
					status = "stale"
				}

				items = append(items, Item{
					Name:       name,
					Detail:     fmt.Sprintf("%s · npm", version),
					Status:     status,
					SystemNote: systemNote,
				})
			}
		}
	}

	// pip3 global packages (only if not using pyenv — avoid double-counting)
	if commandExists("pip3") && !commandExists("pyenv") {
		lines := runLines("pip3", "list", "--format=freeze")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "==", 2)
			name := parts[0]
			version := ""
			if len(parts) > 1 {
				version = parts[1]
			}

			if isStdPythonPkg(name) {
				continue
			}

			items = append(items, Item{
				Name:   name,
				Detail: fmt.Sprintf("%s · pip", version),
				Status: "current",
			})
		}
	}

	// cargo global packages
	if commandExists("cargo") {
		lines := runLines("cargo", "install", "--list")
		var currentPkg string
		for _, line := range lines {
			if !strings.HasPrefix(line, " ") && strings.Contains(line, " v") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					currentPkg = parts[0]
					version := strings.Trim(parts[1], "v:")
					items = append(items, Item{
						Name:   currentPkg,
						Detail: fmt.Sprintf("%s · cargo", version),
						Status: "current",
					})
				}
			}
		}
	}

	return Category{
		ID:    "globals",
		Label: "Global Packages",
		Icon:  "📦",
		Desc:  "npm, pip, and cargo global installs",
		Items: items,
	}
}

func checkDeprecatedNPM(name string) string {
	deprecated := map[string]string{
		"create-react-app": "Deprecated upstream — use Vite or Next.js",
		"nodemon":          "tsx --watch or node --watch provides equivalent functionality",
		"tslint":           "Deprecated — use ESLint with typescript-eslint",
		"bower":            "Deprecated — use npm/yarn/pnpm directly",
		"gulp":             "Consider modern alternatives like npm scripts or Vite",
		"grunt":            "Deprecated — consider npm scripts or modern bundlers",
		"request":          "Deprecated — use node-fetch, undici, or axios",
	}
	if note, ok := deprecated[name]; ok {
		return note
	}
	return ""
}
