package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testEnv is a fake home and an empty dotfiles repo, with commands stubbed out.
func testEnv(t *testing.T, manifest []configEntry) *dotEnv {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	env := &dotEnv{
		Home:     filepath.Join(root, "home"),
		RepoDir:  filepath.Join(root, "home", ".basecamp", "dotfiles"),
		Manifest: manifest,
		Run: func(name string, args ...string) ([]byte, error) {
			return nil, errors.New("not available in tests")
		},
		Now: func() time.Time { return time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC) },
	}
	os.MkdirAll(env.Home, 0o755)
	if err := initRepo(env, ""); err != nil {
		t.Fatal(err)
	}
	git(env.RepoDir, "config", "user.email", "test@example.com")
	git(env.RepoDir, "config", "user.name", "test")
	git(env.RepoDir, "config", "commit.gpgsign", "false")
	return env
}

func writeHome(t *testing.T, env *dotEnv, rel, content string) {
	t.Helper()
	p := filepath.Join(env.Home, rel)
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func statusOf(files []*configFile, repoPath string) string {
	for _, f := range files {
		if f.RepoPath == repoPath {
			return f.Status
		}
	}
	return ""
}

var simpleManifest = []configEntry{
	{ID: "zshrc", Label: ".zshrc", Group: "Shell", Source: ".zshrc", RepoPath: "shell/.zshrc"},
	{ID: "gitconfig", Label: ".gitconfig", Group: "Git", Source: ".gitconfig", RepoPath: "git/.gitconfig"},
	{ID: "npmrc", Label: ".npmrc", Group: "Tools", Source: ".npmrc", RepoPath: "tools/.npmrc"},
	{ID: "absent", Label: ".absent", Group: "Tools", Source: ".absent", RepoPath: "tools/.absent"},
	{ID: "snippets", Label: "snippets", Group: "Editors", Source: "snips", RepoPath: "vscode/snippets", Dir: true},
}

func TestStatusSaveAndChange(t *testing.T) {
	env := testEnv(t, simpleManifest)
	writeHome(t, env, ".zshrc", "alias ll='ls -la'\n")
	writeHome(t, env, ".gitconfig", "[user]\n\tname = Test\n")
	writeHome(t, env, ".npmrc", "//registry.npmjs.org/:_authToken=npm_abcdefghijklmnop1234\n")
	writeHome(t, env, "snips/go.json", "{}\n")

	files := collectConfigs(env)
	if got := statusOf(files, "shell/.zshrc"); got != statusNew {
		t.Fatalf(".zshrc before save = %q, want new", got)
	}
	if got := statusOf(files, "tools/.npmrc"); got != statusBlocked {
		t.Fatalf(".npmrc with a token = %q, want blocked", got)
	}
	if got := statusOf(files, "tools/.absent"); got != "" {
		t.Fatalf("a file on neither side should not be listed, got %q", got)
	}

	res, err := saveConfigs(env, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Committed || len(res.Saved) != 3 {
		t.Fatalf("save = %+v, want 3 files committed", res)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].RepoPath != "tools/.npmrc" || res.Skipped[0].Secrets[0].Line != 1 {
		t.Fatalf("skipped = %+v, want .npmrc refused at line 1", res.Skipped)
	}
	if _, err := os.Stat(filepath.Join(env.RepoDir, "tools/.npmrc")); err == nil {
		t.Fatal(".npmrc with a token was copied into the repo")
	}
	if b, _ := os.ReadFile(filepath.Join(env.RepoDir, "README.md")); !bytes.Contains(b, []byte("`shell/.zshrc` | ~/.zshrc")) {
		t.Fatalf("README missing the .zshrc row:\n%s", b)
	}

	st := buildStatus(env, true)
	if st.Counts[statusSynced] != 3 || st.LastSave == "" {
		t.Fatalf("after save: counts %v lastSave %q", st.Counts, st.LastSave)
	}

	writeHome(t, env, ".zshrc", "alias ll='ls -la'\nalias gs='git status'\n")
	os.Remove(filepath.Join(env.Home, "snips/go.json"))
	st = buildStatus(env, true)
	var zsh *configFile
	for _, f := range st.Files {
		if f.RepoPath == "shell/.zshrc" {
			zsh = f
		}
	}
	if zsh.Status != statusChanged || !strings.Contains(zsh.Diff, "+alias gs='git status'") {
		t.Fatalf(".zshrc after edit: status %q diff %q", zsh.Status, zsh.Diff)
	}
	if got := statusOf(st.Files, "vscode/snippets/go.json"); got != statusMissing {
		t.Fatalf("deleted snippet = %q, want missing", got)
	}

	// A second save commits only the change and keeps the deleted file in the repo.
	res, err = saveConfigs(env, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Saved) != 1 || res.Saved[0] != "shell/.zshrc" {
		t.Fatalf("second save = %v", res.Saved)
	}
	if _, err := os.Stat(filepath.Join(env.RepoDir, "vscode/snippets/go.json")); err != nil {
		t.Fatal("save deleted a file that is only gone from the Mac")
	}

	// Nothing changed: no new commit.
	before := gitOut(env.RepoDir, "rev-parse", "HEAD")
	res, _ = saveConfigs(env, false)
	if res.Committed || gitOut(env.RepoDir, "rev-parse", "HEAD") != before {
		t.Fatal("a save with no changes made a commit")
	}
}

func TestSaveNeedsRepo(t *testing.T) {
	env := testEnv(t, simpleManifest)
	os.RemoveAll(env.RepoDir)
	if _, err := saveConfigs(env, false); err == nil {
		t.Fatal("save without a repo should fail")
	}
}

func TestSaveAndPush(t *testing.T) {
	env := testEnv(t, simpleManifest)
	remote := filepath.Join(t.TempDir(), "remote.git")
	if out, err := exec.Command("git", "init", "-q", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	if err := initRepo(env, remote); err != nil { // existing repo: adds the remote
		t.Fatal(err)
	}
	writeHome(t, env, ".zshrc", "export EDITOR=vim\n")
	res, err := saveConfigs(env, true)
	if err != nil || !res.Pushed {
		t.Fatalf("push: %v %+v", err, res)
	}
	if st := buildStatus(env, false); st.Unpushed != 0 || st.Remote != remote {
		t.Fatalf("after push: %+v", st)
	}
	// A fresh clone, as on a new Mac, gets the file.
	env2 := testEnv(t, simpleManifest)
	os.RemoveAll(env2.RepoDir)
	if err := initRepo(env2, remote); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(env2.RepoDir, "shell/.zshrc")); string(b) != "export EDITOR=vim\n" {
		t.Fatalf("clone has %q", b)
	}
}

func TestRestore(t *testing.T) {
	env := testEnv(t, simpleManifest)
	writeHome(t, env, ".zshrc", "saved version\n")
	writeHome(t, env, ".gitconfig", "[user]\n")
	if _, err := saveConfigs(env, false); err != nil {
		t.Fatal(err)
	}
	writeHome(t, env, ".zshrc", "local edit\n")
	os.Remove(filepath.Join(env.Home, ".gitconfig"))

	plan, err := restoreConfigs(env, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Write) != 2 {
		t.Fatalf("dry run plan = %+v", plan.Write)
	}
	if b, _ := os.ReadFile(filepath.Join(env.Home, ".zshrc")); string(b) != "local edit\n" {
		t.Fatal("dry run wrote a file")
	}

	plan, err = restoreConfigs(env, true)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(env.Home, ".zshrc")); string(b) != "saved version\n" {
		t.Fatalf(".zshrc after restore = %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(env.Home, ".gitconfig")); string(b) != "[user]\n" {
		t.Fatalf(".gitconfig after restore = %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(plan.Backup, "shell/.zshrc")); string(b) != "local edit\n" {
		t.Fatalf("backup of replaced .zshrc = %q (backup dir %q)", b, plan.Backup)
	}
}

func TestForbiddenFilesNeverCopied(t *testing.T) {
	env := testEnv(t, []configEntry{
		{ID: "ssh", Label: "ssh", Group: "SSH", Source: ".ssh", RepoPath: "ssh", Dir: true},
		{ID: "gh-hosts", Label: "gh hosts", Group: "Tools", Source: ".config/gh/hosts.yml", RepoPath: "gh/hosts.yml"},
	})
	writeHome(t, env, ".ssh/config", "Host *\n  UseKeychain yes\n")
	writeHome(t, env, ".ssh/id_ed25519", "not really a key\n")
	writeHome(t, env, ".ssh/known_hosts", "github.com ssh-ed25519 AAAA\n")
	writeHome(t, env, ".config/gh/hosts.yml", "github.com:\n  user: x\n")
	res, err := saveConfigs(env, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Saved) != 1 || res.Saved[0] != "ssh/config" {
		t.Fatalf("saved %v, want only ssh/config", res.Saved)
	}
	for _, p := range []string{"ssh/id_ed25519", "ssh/known_hosts", "gh/hosts.yml"} {
		if _, err := os.Stat(filepath.Join(env.RepoDir, p)); err == nil {
			t.Fatalf("%s was copied", p)
		}
	}
	if len(res.Skipped) != 3 {
		t.Fatalf("skipped %+v, want the key, known_hosts and hosts.yml reported", res.Skipped)
	}
}

func TestGeneratedAndPlistEntries(t *testing.T) {
	env := testEnv(t, []configEntry{
		{ID: "brewfile", Label: "Brewfile", Group: "Packages", RepoPath: "brew/Brewfile", Generate: genBrewfile, RestoreHint: "brew bundle --file=brew/Brewfile"},
		{ID: "ext", Label: "VS Code extensions", Group: "Editors", RepoPath: "vscode/extensions.txt", Generate: genVSCodeExtensions},
		{ID: "iterm2", Label: "iTerm2 preferences", Group: "Terminal", Resolve: itermPrefsPath, RepoPath: "iterm2/prefs.plist", PlistXML: true},
	})
	custom := filepath.Join(env.Home, "iCloud", "iterm2")
	writeHome(t, env, "iCloud/iterm2/com.googlecode.iterm2.plist", "binary plist")
	env.Run = func(name string, args ...string) ([]byte, error) {
		cmd := name + " " + strings.Join(args, " ")
		switch {
		case strings.HasPrefix(cmd, "brew bundle dump"):
			return []byte("brew \"git\"\n"), nil
		case cmd == "code --list-extensions":
			return []byte("ms-python.python\ngolang.go\n"), nil
		case strings.HasSuffix(cmd, "LoadPrefsFromCustomFolder"):
			return []byte("1\n"), nil
		case strings.HasSuffix(cmd, "PrefsCustomFolder"):
			return []byte(custom + "\n"), nil
		case strings.HasPrefix(cmd, "plutil -convert xml1"):
			return []byte("<plist>xml</plist>\n"), nil
		}
		return nil, errors.New("unexpected: " + cmd)
	}
	files := collectConfigs(env)
	got := map[string]*configFile{}
	for _, f := range files {
		got[f.RepoPath] = f
	}
	if f := got["vscode/extensions.txt"]; f == nil || string(f.current) != "golang.go\nms-python.python\n" {
		t.Fatalf("extensions = %+v", f)
	}
	if f := got["iterm2/prefs.plist"]; f == nil || f.Source != "~/iCloud/iterm2/com.googlecode.iterm2.plist" || string(f.current) != "<plist>xml</plist>\n" {
		t.Fatalf("iterm2 = %+v", f)
	}
	if f := got["brew/Brewfile"]; f == nil || f.Status != statusNew {
		t.Fatalf("Brewfile = %+v", f)
	}
	plan, _ := restoreConfigs(env, false)
	for _, w := range plan.Write {
		if w.RepoPath == "brew/Brewfile" {
			t.Fatal("restore would overwrite a generated file")
		}
	}
}

func TestDotfilesHTTP(t *testing.T) {
	env := testEnv(t, simpleManifest)
	writeHome(t, env, ".zshrc", "alias a=b\n")
	mux := http.NewServeMux()
	registerDotfilesRoutesWith(mux, func() *dotEnv { return env })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/dotfiles")
	if err != nil {
		t.Fatal(err)
	}
	var st DotfilesStatus
	json.NewDecoder(resp.Body).Decode(&st)
	resp.Body.Close()
	if st.Counts[statusNew] != 1 || !strings.Contains(st.Files[0].Diff, "+alias a=b") {
		t.Fatalf("GET status = %+v", st)
	}

	resp, _ = http.Post(srv.URL+"/api/dotfiles/save", "application/json", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("save without the header = %d, want 403", resp.StatusCode)
	}
	req, _ := http.NewRequest("POST", srv.URL+"/api/dotfiles/save?push=0", nil)
	req.Header.Set("X-Basecamp", "1")
	resp, err = http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("save = %v %v", err, resp.StatusCode)
	}
	var res SaveResult
	json.NewDecoder(resp.Body).Decode(&res)
	if !res.Committed || res.Saved[0] != "shell/.zshrc" {
		t.Fatalf("save result = %+v", res)
	}
}

func TestNightlyPlist(t *testing.T) {
	p := nightlyPlist("/Users/x/.basecamp/app/basecamp", "/Users/x/.basecamp/dotfiles", "/Users/x/log & more.log", 3, 0)
	for _, want := range []string{
		"<string>com.basecamp-dotfiles</string>",
		"<string>save</string><string>--push</string>",
		"<key>Hour</key><integer>3</integer>",
		"log &amp; more.log",
	} {
		if !strings.Contains(p, want) {
			t.Fatalf("plist missing %q:\n%s", want, p)
		}
	}
	if _, err := exec.LookPath("plutil"); err == nil {
		f := filepath.Join(t.TempDir(), "job.plist")
		os.WriteFile(f, []byte(p), 0o644)
		if out, err := exec.Command("plutil", "-lint", f).CombinedOutput(); err != nil {
			t.Fatalf("plutil -lint: %s", out)
		}
	}
}
