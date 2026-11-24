package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func scanFonts() Category {
	var items []Item

	home, _ := os.UserHomeDir()
	fontDirs := []struct {
		path   string
		source string
	}{
		{filepath.Join(home, "Library", "Fonts"), "user"},
		{"/Library/Fonts", "system"},
	}

	// Track font families (group variants together)
	families := make(map[string]*fontFamily)

	for _, fd := range fontDirs {
		entries, err := os.ReadDir(fd.path)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}

			ext := strings.ToLower(filepath.Ext(name))
			if ext != ".ttf" && ext != ".otf" && ext != ".ttc" && ext != ".woff" && ext != ".woff2" {
				continue
			}

			// Extract family name (strip variant suffixes)
			familyName := extractFontFamily(name)

			if fam, ok := families[familyName]; ok {
				fam.variants++
				if fd.source == "user" {
					fam.source = "user"
				}
			} else {
				families[familyName] = &fontFamily{
					name:     familyName,
					variants: 1,
					source:   fd.source,
				}
			}
		}
	}

	// Also check for brew-installed fonts (cask font taps)
	brewFonts := runLines("brew", "list", "--cask")
	brewFontSet := make(map[string]bool)
	for _, cask := range brewFonts {
		if strings.HasPrefix(cask, "font-") {
			fontName := strings.TrimPrefix(cask, "font-")
			fontName = strings.ReplaceAll(fontName, "-", " ")
			fontName = strings.Title(fontName)
			brewFontSet[fontName] = true
		}
	}

	for _, fam := range families {
		source := fam.source
		if brewFontSet[fam.name] {
			source = "brew cask"
		}

		// Check if font is referenced in active configs
		systemNote := ""
		if isDevFont(fam.name) {
			systemNote = checkFontUsage(fam.name)
		}

		items = append(items, Item{
			Name:       fam.name,
			Detail:     fmt.Sprintf("%d variants · %s", fam.variants, source),
			Status:     "active",
			SystemNote: systemNote,
		})
	}

	return Category{
		ID:    "fonts",
		Label: "Fonts",
		Icon:  "Aa",
		Desc:  "Developer and UI fonts",
		Items: items,
	}
}

type fontFamily struct {
	name     string
	variants int
	source   string
}

// fontStyle matches a style suffix, alone or run together: "Bold", "SemiBoldItalic", "Retina".
var fontStyle = regexp.MustCompile(`(?i)^(thin|hairline|extra|ultra|semi|demi|light|regular|book|medium|bold|black|heavy|italic|oblique|retina)+$`)

// fontStyleAbbrev matches the short forms some families use after a space ("Anonymous Pro BI").
var fontStyleAbbrev = regexp.MustCompile(`^(B|I|BI|R)$`)

var fontAxes = regexp.MustCompile(`\[[^\]]*\]`) // variable font axes, "[wght]"

// extractFontFamily turns a font file name into its family:
// "JetBrainsMono-ExtraBoldItalic.ttf" and "JetBrainsMono-Italic[wght].ttf" are "JetBrainsMono",
// "FuraMono-Bold Powerline.otf" is "FuraMono Powerline", "Anonymous Pro BI.ttf" is "Anonymous Pro".
func extractFontFamily(filename string) string {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	name = strings.TrimSpace(fontAxes.ReplaceAllString(name, ""))

	if i := strings.LastIndex(name, "-"); i > 0 {
		base, kept := name[:i], []string{}
		for _, w := range strings.Fields(name[i+1:]) {
			if !fontStyle.MatchString(w) {
				kept = append(kept, w)
			}
		}
		if len(kept) > 0 {
			base += " " + strings.Join(kept, " ")
		}
		return base
	}

	words := strings.Fields(name)
	for len(words) > 1 {
		last := words[len(words)-1]
		if !fontStyle.MatchString(last) && !fontStyleAbbrev.MatchString(last) {
			break
		}
		words = words[:len(words)-1]
	}
	return strings.Join(words, " ")
}

func isDevFont(name string) bool {
	devFonts := []string{
		"JetBrains", "Fira", "SF Mono", "Monaco", "Menlo",
		"Hack", "Iosevka", "Cascadia", "Source Code",
		"Consolas", "Inconsolata", "Berkeley", "Monaspace",
		"Geist", "Nerd", "Meslo",
	}
	lower := strings.ToLower(name)
	for _, df := range devFonts {
		if strings.Contains(lower, strings.ToLower(df)) {
			return true
		}
	}
	return false
}

func checkFontUsage(fontName string) string {
	// Check common config files for font references
	configFiles := map[string]string{
		"~/Library/Application Support/Code/User/settings.json": "VS Code",
		"~/.config/alacritty/alacritty.yml":                     "Alacritty",
		"~/.config/kitty/kitty.conf":                            "Kitty",
	}

	// Check iTerm2 plist
	itermPlist := "~/Library/Preferences/com.googlecode.iterm2.plist"
	if fileExists(itermPlist) {
		configFiles[itermPlist] = "iTerm2"
	}

	lower := strings.ToLower(fontName)
	for path, app := range configFiles {
		if !fileExists(path) {
			continue
		}
		content := strings.Join(readFileLines(path), "\n")
		if strings.Contains(strings.ToLower(content), lower) {
			return fmt.Sprintf("Referenced in %s config", app)
		}
	}
	return ""
}
