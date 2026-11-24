package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Configs basecamp keeps saved. Contents go only to the private repo cloned at
// ~/.basecamp/dotfiles (never to the basecamp repo, which is public).

// configEntry is one thing to save: a file, a folder of files, or generated output.
type configEntry struct {
	ID    string
	Label string
	Group string
	// Source is relative to home, unless Resolve sets it.
	Source string
	// Resolve finds Source at run time (iTerm2 may load prefs from a custom folder).
	Resolve func(env *dotEnv) string
	// RepoPath is where the copy lives in the dotfiles repo (a folder for Dir entries).
	RepoPath string
	Dir      bool
	// PlistXML stores a plist as XML so it diffs as text.
	PlistXML bool
	// Generate builds the content (Brewfile, extension list). Generated entries
	// are never restored over anything; restore prints how to apply them.
	Generate func(env *dotEnv) ([]byte, error)
	// RestoreHint is printed by restore for generated entries.
	RestoreHint string
}

const vscodeUser = "Library/Application Support/Code/User"

var configManifest = []configEntry{
	{ID: "zshrc", Label: ".zshrc", Group: "Shell", Source: ".zshrc", RepoPath: "shell/.zshrc"},
	{ID: "zprofile", Label: ".zprofile", Group: "Shell", Source: ".zprofile", RepoPath: "shell/.zprofile"},
	{ID: "zshenv", Label: ".zshenv", Group: "Shell", Source: ".zshenv", RepoPath: "shell/.zshenv"},
	{ID: "zlogin", Label: ".zlogin", Group: "Shell", Source: ".zlogin", RepoPath: "shell/.zlogin"},
	{ID: "bashrc", Label: ".bashrc", Group: "Shell", Source: ".bashrc", RepoPath: "shell/.bashrc"},
	{ID: "bash_profile", Label: ".bash_profile", Group: "Shell", Source: ".bash_profile", RepoPath: "shell/.bash_profile"},
	{ID: "profile", Label: ".profile", Group: "Shell", Source: ".profile", RepoPath: "shell/.profile"},
	{ID: "inputrc", Label: ".inputrc", Group: "Shell", Source: ".inputrc", RepoPath: "shell/.inputrc"},
	{ID: "fish", Label: "fish config", Group: "Shell", Source: ".config/fish/config.fish", RepoPath: "shell/fish/config.fish"},
	{ID: "starship", Label: "starship.toml", Group: "Shell", Source: ".config/starship.toml", RepoPath: "shell/starship.toml"},
	{ID: "tmux", Label: ".tmux.conf", Group: "Shell", Source: ".tmux.conf", RepoPath: "shell/.tmux.conf"},

	{ID: "gitconfig", Label: ".gitconfig", Group: "Git and SSH", Source: ".gitconfig", RepoPath: "git/.gitconfig"},
	{ID: "git-config", Label: "git/config", Group: "Git and SSH", Source: ".config/git/config", RepoPath: "git/config"},
	{ID: "git-ignore", Label: "git/ignore", Group: "Git and SSH", Source: ".config/git/ignore", RepoPath: "git/ignore"},
	{ID: "gitignore-global", Label: ".gitignore_global", Group: "Git and SSH", Source: ".gitignore_global", RepoPath: "git/.gitignore_global"},
	{ID: "ssh-config", Label: "ssh config", Group: "Git and SSH", Source: ".ssh/config", RepoPath: "ssh/config"},

	{ID: "iterm2", Label: "iTerm2 preferences", Group: "Terminal", Resolve: itermPrefsPath, RepoPath: "iterm2/com.googlecode.iterm2.plist", PlistXML: true},
	{ID: "iterm2-dynamic", Label: "iTerm2 Dynamic Profiles", Group: "Terminal", Source: "Library/Application Support/iTerm2/DynamicProfiles", RepoPath: "iterm2/DynamicProfiles", Dir: true},

	{ID: "vscode-settings", Label: "VS Code settings", Group: "Editors", Source: vscodeUser + "/settings.json", RepoPath: "vscode/settings.json"},
	{ID: "vscode-keybindings", Label: "VS Code keybindings", Group: "Editors", Source: vscodeUser + "/keybindings.json", RepoPath: "vscode/keybindings.json"},
	{ID: "vscode-mcp", Label: "VS Code MCP servers", Group: "Editors", Source: vscodeUser + "/mcp.json", RepoPath: "vscode/mcp.json"},
	{ID: "vscode-snippets", Label: "VS Code snippets", Group: "Editors", Source: vscodeUser + "/snippets", RepoPath: "vscode/snippets", Dir: true},
	{ID: "vscode-extensions", Label: "VS Code extensions", Group: "Editors", RepoPath: "vscode/extensions.txt", Generate: genVSCodeExtensions,
		RestoreHint: "xargs -L1 code --install-extension < vscode/extensions.txt"},
	{ID: "zed-settings", Label: "Zed settings", Group: "Editors", Source: ".config/zed/settings.json", RepoPath: "zed/settings.json"},
	{ID: "zed-keymap", Label: "Zed keymap", Group: "Editors", Source: ".config/zed/keymap.json", RepoPath: "zed/keymap.json"},
	{ID: "vimrc", Label: ".vimrc", Group: "Editors", Source: ".vimrc", RepoPath: "vim/.vimrc"},
	{ID: "nvim", Label: "Neovim config", Group: "Editors", Source: ".config/nvim", RepoPath: "nvim", Dir: true},
	{ID: "editorconfig", Label: ".editorconfig", Group: "Editors", Source: ".editorconfig", RepoPath: "editors/.editorconfig"},

	{ID: "brewfile", Label: "Brewfile", Group: "Packages", RepoPath: "brew/Brewfile", Generate: genBrewfile,
		RestoreHint: "brew bundle --file=brew/Brewfile"},

	{ID: "gh-config", Label: "gh config", Group: "Tools", Source: ".config/gh/config.yml", RepoPath: "tools/gh/config.yml"},
	{ID: "npmrc", Label: ".npmrc", Group: "Tools", Source: ".npmrc", RepoPath: "tools/.npmrc"},
	{ID: "yarnrc", Label: ".yarnrc", Group: "Tools", Source: ".yarnrc", RepoPath: "tools/.yarnrc"},
	{ID: "curlrc", Label: ".curlrc", Group: "Tools", Source: ".curlrc", RepoPath: "tools/.curlrc"},
	{ID: "claude-settings", Label: "Claude Code settings", Group: "Tools", Source: ".claude/settings.json", RepoPath: "claude/settings.json"},
	{ID: "claude-md", Label: "Claude Code CLAUDE.md", Group: "Tools", Source: ".claude/CLAUDE.md", RepoPath: "claude/CLAUDE.md"},
}

// maxConfigSize skips anything bigger; config files are small and big files are usually caches.
const maxConfigSize = 2 << 20

// dotEnv is everything the dotfiles code touches on the machine, so tests can fake it.
type dotEnv struct {
	Home     string
	RepoDir  string
	Manifest []configEntry
	// Run runs a command and returns stdout.
	Run func(name string, args ...string) ([]byte, error)
	// Now is the clock (tests pin it).
	Now func() time.Time
}

func defaultDotEnv() *dotEnv {
	home, _ := os.UserHomeDir()
	repo := os.Getenv("BASECAMP_DOTFILES_DIR")
	if repo == "" {
		repo = filepath.Join(home, ".basecamp", "dotfiles")
	}
	return &dotEnv{
		Home:     home,
		RepoDir:  expandHome(repo),
		Manifest: configManifest,
		Run: func(name string, args ...string) ([]byte, error) {
			return exec.Command(name, args...).Output()
		},
		Now: time.Now,
	}
}

// configFile is one concrete file: a manifest entry, or one file inside a Dir entry.
type configFile struct {
	EntryID   string          `json:"entryId"`
	Label     string          `json:"label"`
	Group     string          `json:"group"`
	Source    string          `json:"source,omitempty"` // display path, ~ for home
	RepoPath  string          `json:"repoPath"`
	Generated bool            `json:"generated,omitempty"`
	Status    string          `json:"status"` // synced, changed, new, blocked, missing, error
	Secrets   []SecretFinding `json:"secrets,omitempty"`
	Error     string          `json:"error,omitempty"`
	Diff      string          `json:"diff,omitempty"`
	Lines     int             `json:"lines,omitempty"`
	Modified  string          `json:"modified,omitempty"`

	absSource string
	current   []byte // nil when the source is gone
	saved     []byte // nil when the repo has no copy
}

const (
	statusSynced  = "synced"
	statusChanged = "changed"
	statusNew     = "new"
	statusBlocked = "blocked"
	statusMissing = "missing"
	statusError   = "error"
)

func (e *dotEnv) abs(p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(e.Home, p)
}

func (e *dotEnv) display(p string) string {
	if rel, err := filepath.Rel(e.Home, p); err == nil && !strings.HasPrefix(rel, "..") {
		return "~/" + rel
	}
	return p
}

// collectConfigs reads every manifest entry and compares it with the repo copy.
func collectConfigs(env *dotEnv) []*configFile {
	var out []*configFile
	for _, entry := range env.Manifest {
		out = append(out, collectEntry(env, entry)...)
	}
	for _, f := range out {
		classify(f)
	}
	return out
}

func collectEntry(env *dotEnv, entry configEntry) []*configFile {
	base := configFile{EntryID: entry.ID, Label: entry.Label, Group: entry.Group, RepoPath: entry.RepoPath}

	if entry.Generate != nil {
		f := base
		f.Generated = true
		data, err := entry.Generate(env)
		if err != nil {
			f.Error = err.Error()
		} else if data != nil {
			f.current = data
		}
		f.saved = readOrNil(filepath.Join(env.RepoDir, entry.RepoPath))
		if f.current == nil && f.saved == nil && f.Error == "" {
			return nil
		}
		return []*configFile{&f}
	}

	src := entry.Source
	if entry.Resolve != nil {
		src = entry.Resolve(env)
	}
	src = env.abs(src)

	if entry.Dir {
		return collectDir(env, base, src)
	}

	f := base
	f.absSource = src
	if src != "" {
		f.Source = env.display(src)
	}
	if forbiddenPath(src) {
		f.Error = "never copied: this file holds keys or credentials"
		return []*configFile{&f}
	}
	readSource(env, &f, entry.PlistXML)
	f.saved = readOrNil(filepath.Join(env.RepoDir, entry.RepoPath))
	if f.current == nil && f.saved == nil && f.Error == "" {
		return nil // not on this Mac and never saved: nothing to show
	}
	return []*configFile{&f}
}

// collectDir expands a folder entry into one configFile per file, on either side.
func collectDir(env *dotEnv, base configFile, srcDir string) []*configFile {
	rels := map[string]bool{}
	walkFiles(srcDir, func(rel string) { rels[rel] = true })
	walkFiles(filepath.Join(env.RepoDir, base.RepoPath), func(rel string) { rels[rel] = true })

	names := make([]string, 0, len(rels))
	for r := range rels {
		names = append(names, r)
	}
	sort.Strings(names)

	var out []*configFile
	for _, rel := range names {
		f := base
		f.Label = base.Label + ": " + rel
		f.RepoPath = filepath.ToSlash(filepath.Join(base.RepoPath, rel))
		f.absSource = filepath.Join(srcDir, rel)
		f.Source = env.display(f.absSource)
		if forbiddenPath(f.absSource) {
			f.Error = "never copied: this file holds keys or credentials"
		} else {
			readSource(env, &f, false)
		}
		f.saved = readOrNil(filepath.Join(env.RepoDir, f.RepoPath))
		out = append(out, &f)
	}
	return out
}

func walkFiles(root string, fn func(rel string)) {
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != root && (d.Name() == ".git" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == ".DS_Store" || !d.Type().IsRegular() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		fn(rel)
		return nil
	})
}

func readSource(env *dotEnv, f *configFile, plistXML bool) {
	info, err := os.Stat(f.absSource)
	if err != nil {
		return // gone: current stays nil
	}
	if info.Size() > maxConfigSize {
		f.Error = fmt.Sprintf("skipped: %d KB is too big for a config file", info.Size()/1024)
		return
	}
	f.Modified = info.ModTime().UTC().Format(time.RFC3339)
	var data []byte
	if plistXML {
		data, err = env.Run("plutil", "-convert", "xml1", "-o", "-", f.absSource)
	} else {
		data, err = os.ReadFile(f.absSource)
	}
	if err != nil {
		f.Error = "could not read: " + err.Error()
		return
	}
	f.current = data
}

func readOrNil(p string) []byte {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	return data
}

func classify(f *configFile) {
	if f.current != nil {
		f.Lines = bytes.Count(f.current, []byte("\n"))
		f.Secrets = scanSecrets(f.current)
	}
	switch {
	case f.Error != "" && f.current == nil:
		f.Status = statusError
	case f.current == nil:
		f.Status = statusMissing
	case f.saved != nil && bytes.Equal(f.current, f.saved):
		f.Status = statusSynced
	case len(f.Secrets) > 0:
		f.Status = statusBlocked
	case f.saved == nil:
		f.Status = statusNew
	default:
		f.Status = statusChanged
	}
}

// ---- generated entries ----

// genBrewfile dumps Homebrew taps, formulae and casks only. npm, uv, go and VS Code
// entries depend on which tools are on PATH (a launchd job sees fewer than a shell),
// so they would flip between saves; VS Code extensions have their own list.
func genBrewfile(env *dotEnv) ([]byte, error) {
	out, err := env.Run("brew", "bundle", "dump", "--file=-", "--no-vscode", "--no-go", "--no-cargo", "--no-npm", "--no-uv", "--no-krew", "--no-flatpak", "--no-winget")
	if err != nil {
		// Older brew without the --no-* flags.
		out, err = env.Run("brew", "bundle", "dump", "--file=-")
	}
	if err != nil {
		return nil, fmt.Errorf("brew bundle dump failed: %v", err)
	}
	return out, nil
}

func genVSCodeExtensions(env *dotEnv) ([]byte, error) {
	out, err := env.Run("code", "--list-extensions")
	if err != nil {
		return nil, nil // VS Code CLI not installed: nothing to save
	}
	lines := strings.Fields(string(out))
	sort.Strings(lines)
	if len(lines) == 0 {
		return nil, nil
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

// itermPrefsPath follows iTerm2's "load preferences from a custom folder" setting.
func itermPrefsPath(env *dotEnv) string {
	if out, err := env.Run("defaults", "read", "com.googlecode.iterm2", "LoadPrefsFromCustomFolder"); err == nil && strings.TrimSpace(string(out)) == "1" {
		if dir, err := env.Run("defaults", "read", "com.googlecode.iterm2", "PrefsCustomFolder"); err == nil {
			d := expandHomeIn(env.Home, strings.TrimSpace(string(dir)))
			if d != "" {
				p := filepath.Join(d, "com.googlecode.iterm2.plist")
				if _, err := os.Stat(p); err == nil {
					return p
				}
			}
		}
	}
	return filepath.Join(env.Home, "Library/Preferences/com.googlecode.iterm2.plist")
}

func expandHomeIn(home, p string) string {
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}
