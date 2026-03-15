package main

import (
	"fmt"
	"path/filepath"
	"os"
	"strings"
)

func scanRuntimes() Category {
	var items []Item

	// Node.js via nvm
	if nodeItem := scanNodeNVM(); nodeItem != nil {
		items = append(items, *nodeItem)
	} else if commandExists("node") {
		version := run("node", "--version")
		items = append(items, Item{
			Name:   "Node.js",
			Detail: fmt.Sprintf("%s via system", version),
			Status: "current",
		})
	}

	// Python via pyenv
	if pyItem := scanPythonPyenv(); pyItem != nil {
		items = append(items, *pyItem)
	} else if commandExists("python3") {
		version := run("python3", "--version")
		version = strings.TrimPrefix(version, "Python ")
		items = append(items, Item{
			Name:   "Python",
			Detail: fmt.Sprintf("%s via system", version),
			Status: "current",
		})
	}

	// Ruby via rbenv
	if commandExists("rbenv") {
		version := run("rbenv", "version-name")
		others := runLines("rbenv", "versions", "--bare")
		note := ""
		if len(others) > 1 {
			note = fmt.Sprintf("Also installed: %s", strings.Join(filterOut(others, version), ", "))
		}
		items = append(items, Item{
			Name:       "Ruby",
			Detail:     fmt.Sprintf("%s via rbenv", version),
			Status:     "current",
			SystemNote: note,
		})
	} else if commandExists("ruby") {
		version := run("ruby", "--version")
		items = append(items, Item{
			Name:   "Ruby",
			Detail: fmt.Sprintf("%s via system", strings.Fields(version)[1]),
			Status: "current",
		})
	}

	// Go
	if commandExists("go") {
		version := run("go", "version")
		// "go version go1.23.4 darwin/arm64"
		parts := strings.Fields(version)
		ver := ""
		if len(parts) >= 3 {
			ver = strings.TrimPrefix(parts[2], "go")
		}
		items = append(items, Item{
			Name:   "Go",
			Detail: fmt.Sprintf("%s via brew", ver),
			Status: "current",
		})
	}

	// Rust via rustup
	if commandExists("rustup") {
		version := run("rustc", "--version")
		// "rustc 1.84.0 (hash date)"
		parts := strings.Fields(version)
		ver := ""
		if len(parts) >= 2 {
			ver = parts[1]
		}
		toolchain := run("rustup", "default")
		note := ""
		if strings.Contains(toolchain, "stable") {
			note = "Stable toolchain active"
		}
		items = append(items, Item{
			Name:       "Rust",
			Detail:     fmt.Sprintf("%s via rustup", ver),
			Status:     "current",
			SystemNote: note,
		})
	}

	// Java via sdkman or system
	if commandExists("java") {
		version := run("java", "-version") // java -version outputs to stderr
		if version == "" {
			// Try alternative
			version = run("java", "--version")
		}
		if version != "" {
			firstLine := strings.Split(version, "\n")[0]
			items = append(items, Item{
				Name:   "Java",
				Detail: firstLine,
				Status: "current",
			})
		}
	}

	// Deno
	if commandExists("deno") {
		version := run("deno", "--version")
		firstLine := strings.Split(version, "\n")[0]
		ver := strings.TrimPrefix(firstLine, "deno ")
		items = append(items, Item{
			Name:   "Deno",
			Detail: fmt.Sprintf("%s via brew", ver),
			Status: "current",
		})
	}

	// Bun
	if commandExists("bun") {
		version := run("bun", "--version")
		items = append(items, Item{
			Name:   "Bun",
			Detail: fmt.Sprintf("%s via brew", version),
			Status: "current",
		})
	}

	return Category{
		ID:    "versions",
		Label: "Runtimes",
		Icon:  "🔧",
		Desc:  "Languages, version managers, and toolchains",
		Items: items,
	}
}

// scanNodeNVM scans nvm-managed Node.js versions and their global packages
func scanNodeNVM() *Item {
	// Check if nvm directory exists
	home, _ := os.UserHomeDir()
	nvmDir := filepath.Join(home, ".nvm", "versions", "node")
	if !fileExists(nvmDir) {
		return nil
	}

	entries, err := os.ReadDir(nvmDir)
	if err != nil {
		return nil
	}

	currentVersion := run("node", "--version") // e.g. "v22.12.0"
	currentVersion = strings.TrimPrefix(currentVersion, "v")

	var versions []VersionEntry
	var otherVersions []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		ver := strings.TrimPrefix(entry.Name(), "v")

		// Get global packages for this version
		npmBin := filepath.Join(nvmDir, entry.Name(), "lib", "node_modules")
		globals := listNPMGlobals(npmBin)

		label := ""
		if ver == currentVersion {
			label = "current"
		} else {
			otherVersions = append(otherVersions, ver)
		}

		versions = append(versions, VersionEntry{
			Version: ver,
			Label:   label,
			Globals: globals,
		})
	}

	// Sort so current is first
	sortVersionsCurrent(versions)

	note := ""
	if len(otherVersions) > 0 {
		note = fmt.Sprintf("Also installed: %s", strings.Join(otherVersions, ", "))
	}

	return &Item{
		Name:           "Node.js",
		Detail:         fmt.Sprintf("%s via nvm", currentVersion),
		Status:         "current",
		SystemNote:     note,
		VersionManager: "nvm",
		Versions:       versions,
	}
}

// listNPMGlobals reads global packages from a node_modules directory
func listNPMGlobals(nodeModulesDir string) []string {
	entries, err := os.ReadDir(nodeModulesDir)
	if err != nil {
		return nil
	}

	var globals []string
	for _, entry := range entries {
		name := entry.Name()
		// Skip npm itself and hidden dirs
		if name == "npm" || name == "corepack" || strings.HasPrefix(name, ".") {
			continue
		}

		// Handle scoped packages (@scope/name)
		if strings.HasPrefix(name, "@") {
			scopeDir := filepath.Join(nodeModulesDir, name)
			scopeEntries, err := os.ReadDir(scopeDir)
			if err != nil {
				continue
			}
			for _, se := range scopeEntries {
				pkgName := name + "/" + se.Name()
				version := readPackageVersion(filepath.Join(scopeDir, se.Name()))
				if version != "" {
					globals = append(globals, pkgName+"@"+version)
				} else {
					globals = append(globals, pkgName)
				}
			}
			continue
		}

		version := readPackageVersion(filepath.Join(nodeModulesDir, name))
		if version != "" {
			globals = append(globals, name+"@"+version)
		} else {
			globals = append(globals, name)
		}
	}
	return globals
}

// readPackageVersion reads version from package.json
func readPackageVersion(pkgDir string) string {
	data, err := os.ReadFile(filepath.Join(pkgDir, "package.json"))
	if err != nil {
		return ""
	}
	// Quick and dirty version extraction without full JSON parse
	content := string(data)
	idx := strings.Index(content, `"version"`)
	if idx == -1 {
		return ""
	}
	rest := content[idx+9:]
	start := strings.Index(rest, `"`)
	if start == -1 {
		return ""
	}
	rest = rest[start+1:]
	end := strings.Index(rest, `"`)
	if end == -1 {
		return ""
	}
	return rest[:end]
}

// scanPythonPyenv scans pyenv-managed Python versions
func scanPythonPyenv() *Item {
	if !commandExists("pyenv") {
		return nil
	}

	currentVersion := run("pyenv", "version-name")
	allVersions := runLines("pyenv", "versions", "--bare")

	var versions []VersionEntry
	var otherVersions []string

	for _, ver := range allVersions {
		ver = strings.TrimSpace(ver)
		if ver == "" || strings.HasPrefix(ver, "/") {
			continue
		}

		// Get pip packages for this version
		globals := listPipGlobals(ver)

		label := ""
		if ver == currentVersion {
			label = "current"
		} else {
			otherVersions = append(otherVersions, ver)
		}

		versions = append(versions, VersionEntry{
			Version: ver,
			Label:   label,
			Globals: globals,
		})
	}

	sortVersionsCurrent(versions)

	note := ""
	if len(otherVersions) > 0 {
		note = fmt.Sprintf("Also installed: %s", strings.Join(otherVersions, ", "))
	}

	return &Item{
		Name:           "Python",
		Detail:         fmt.Sprintf("%s via pyenv", currentVersion),
		Status:         "current",
		SystemNote:     note,
		VersionManager: "pyenv",
		Versions:       versions,
	}
}

// listPipGlobals gets pip packages for a specific pyenv version
func listPipGlobals(version string) []string {
	home, _ := os.UserHomeDir()
	pipBin := filepath.Join(home, ".pyenv", "versions", version, "bin", "pip")

	if !fileExists(pipBin) {
		return nil
	}

	lines := runLines(pipBin, "list", "--format=freeze")
	var globals []string
	for _, line := range lines {
		// Format: package==version
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Convert == to @
		pkg := strings.Replace(line, "==", "@", 1)
		// Skip standard lib packages
		name := strings.Split(pkg, "@")[0]
		if isStdPythonPkg(name) {
			continue
		}
		globals = append(globals, pkg)
	}
	return globals
}

func isStdPythonPkg(name string) bool {
	std := map[string]bool{
		"pip": true, "setuptools": true, "wheel": true,
		"pkg_resources": true, "distribute": true,
	}
	return std[strings.ToLower(name)]
}

// sortVersionsCurrent moves the "current" version to index 0
func sortVersionsCurrent(versions []VersionEntry) {
	for i, v := range versions {
		if v.Label == "current" && i > 0 {
			versions[0], versions[i] = versions[i], versions[0]
			break
		}
	}
}

func filterOut(list []string, exclude string) []string {
	var result []string
	for _, s := range list {
		if strings.TrimSpace(s) != exclude {
			result = append(result, strings.TrimSpace(s))
		}
	}
	return result
}
