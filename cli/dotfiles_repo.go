package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// DotfilesStatus is what `basecamp dotfiles status --json` and GET /api/dotfiles return.
type DotfilesStatus struct {
	Repo        string         `json:"repo"`
	Initialized bool           `json:"initialized"`
	Remote      string         `json:"remote,omitempty"`
	LastSave    string         `json:"lastSave,omitempty"`
	Unpushed    int            `json:"unpushed"`
	Counts      map[string]int `json:"counts"`
	Files       []*configFile  `json:"files"`
}

func buildStatus(env *dotEnv, withDiffs bool) DotfilesStatus {
	files := collectConfigs(env)
	st := DotfilesStatus{Repo: env.RepoDir, Counts: map[string]int{}, Files: files}
	if isGitRepo(env.RepoDir) {
		st.Initialized = true
		st.Remote = strings.TrimSpace(gitOut(env.RepoDir, "remote", "get-url", "origin"))
		st.LastSave = strings.TrimSpace(gitOut(env.RepoDir, "log", "-1", "--format=%cI"))
		if st.Remote != "" {
			fmt.Sscanf(gitOut(env.RepoDir, "rev-list", "--count", "@{u}..HEAD"), "%d", &st.Unpushed)
		}
	}
	for _, f := range files {
		st.Counts[f.Status]++
		if withDiffs && (f.Status == statusChanged || f.Status == statusNew || f.Status == statusMissing) {
			f.Diff = unifiedDiff(f.saved, f.current, f.RepoPath)
		}
	}
	return st
}

// SaveResult says what a save did, including everything it refused to copy.
type SaveResult struct {
	Saved     []string        `json:"saved"`
	Skipped   []SkippedConfig `json:"skipped"`
	Committed bool            `json:"committed"`
	Commit    string          `json:"commit,omitempty"`
	Pushed    bool            `json:"pushed"`
	PushError string          `json:"pushError,omitempty"`
}

type SkippedConfig struct {
	RepoPath string          `json:"repoPath"`
	Reason   string          `json:"reason"`
	Secrets  []SecretFinding `json:"secrets,omitempty"`
}

// saveConfigs copies every changed or new config into the repo and commits.
// Files with secrets are refused, never stripped: a half-edited copy would restore wrong.
// Files that are gone from the Mac stay in the repo (restore can bring them back).
func saveConfigs(env *dotEnv, push bool) (SaveResult, error) {
	res := SaveResult{Saved: []string{}, Skipped: []SkippedConfig{}}
	if !isGitRepo(env.RepoDir) {
		return res, fmt.Errorf("no dotfiles repo at %s: run `basecamp dotfiles init <git-url>` first", env.RepoDir)
	}
	files := collectConfigs(env)
	for _, f := range files {
		switch f.Status {
		case statusChanged, statusNew:
			dst := filepath.Join(env.RepoDir, f.RepoPath)
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return res, err
			}
			if err := os.WriteFile(dst, f.current, 0o644); err != nil {
				return res, err
			}
			res.Saved = append(res.Saved, f.RepoPath)
		case statusBlocked:
			res.Skipped = append(res.Skipped, SkippedConfig{RepoPath: f.RepoPath, Reason: "looks like it holds a secret", Secrets: f.Secrets})
		case statusError:
			res.Skipped = append(res.Skipped, SkippedConfig{RepoPath: f.RepoPath, Reason: f.Error})
		}
	}
	if err := writeRepoReadme(env, files); err != nil {
		return res, err
	}

	if _, err := git(env.RepoDir, "add", "-A"); err != nil {
		return res, err
	}
	if _, err := git(env.RepoDir, "diff", "--cached", "--quiet"); err != nil {
		msg := commitMessage(res.Saved)
		if out, err := git(env.RepoDir, "commit", "-q", "-m", msg); err != nil {
			return res, fmt.Errorf("git commit: %v: %s", err, out)
		}
		res.Committed = true
		res.Commit = strings.TrimSpace(gitOut(env.RepoDir, "rev-parse", "--short", "HEAD"))
	}

	if push && strings.TrimSpace(gitOut(env.RepoDir, "remote")) != "" {
		if out, err := git(env.RepoDir, "push", "-q", "-u", "origin", "HEAD"); err != nil {
			res.PushError = strings.TrimSpace(out)
			if res.PushError == "" {
				res.PushError = err.Error()
			}
		} else {
			res.Pushed = true
		}
	}
	return res, nil
}

func commitMessage(saved []string) string {
	switch {
	case len(saved) == 0:
		return "Update README"
	case len(saved) <= 4:
		return "Save " + strings.Join(saved, ", ")
	default:
		return fmt.Sprintf("Save %d configs: %s and more", len(saved), strings.Join(saved[:3], ", "))
	}
}

// RestorePlan lists what restore would write, and what it leaves to the user.
type RestorePlan struct {
	Write  []RestoreStep `json:"write"`
	Manual []string      `json:"manual"`
	Backup string        `json:"backup,omitempty"`
}

type RestoreStep struct {
	RepoPath string `json:"repoPath"`
	Target   string `json:"target"`
	Replaces bool   `json:"replaces"`
}

// restoreConfigs copies saved configs back to where they belong. Without apply it
// only reports. Anything it would overwrite is first copied to ~/.basecamp/backups.
func restoreConfigs(env *dotEnv, apply bool) (RestorePlan, error) {
	plan := RestorePlan{}
	if !isGitRepo(env.RepoDir) {
		return plan, fmt.Errorf("no dotfiles repo at %s", env.RepoDir)
	}
	files := collectConfigs(env)
	type job struct {
		f     *configFile
		plist bool
	}
	plistByID := map[string]bool{}
	for _, e := range env.Manifest {
		plistByID[e.ID] = e.PlistXML
	}
	var jobs []job
	for _, f := range files {
		if f.saved == nil || f.Status == statusSynced {
			continue
		}
		if f.Generated {
			continue
		}
		if f.absSource == "" || forbiddenPath(f.absSource) {
			continue
		}
		jobs = append(jobs, job{f, plistByID[f.EntryID]})
		plan.Write = append(plan.Write, RestoreStep{RepoPath: f.RepoPath, Target: f.Source, Replaces: f.current != nil})
	}
	for _, e := range env.Manifest {
		if e.Generate != nil && e.RestoreHint != "" {
			if _, err := os.Stat(filepath.Join(env.RepoDir, e.RepoPath)); err == nil {
				plan.Manual = append(plan.Manual, fmt.Sprintf("cd %s && %s", env.RepoDir, e.RestoreHint))
			}
		}
		if e.PlistXML {
			plan.Manual = append(plan.Manual, "Quit "+strings.TrimSuffix(e.Label, " preferences")+" before restoring its preferences, or it will overwrite them on quit.")
		}
	}
	if !apply {
		return plan, nil
	}

	backup := filepath.Join(env.Home, ".basecamp", "backups", env.Now().Format("20060102-150405"))
	for _, j := range jobs {
		if j.f.current != nil {
			rel := strings.TrimPrefix(j.f.RepoPath, "/")
			if err := copyFile(j.f.absSource, filepath.Join(backup, rel)); err != nil {
				return plan, fmt.Errorf("backup %s: %v", j.f.Source, err)
			}
			plan.Backup = backup
		}
		if err := os.MkdirAll(filepath.Dir(j.f.absSource), 0o755); err != nil {
			return plan, err
		}
		data := j.f.saved
		mode := os.FileMode(0o644)
		if strings.Contains(j.f.absSource, "/.ssh/") {
			mode = 0o600
		}
		if err := os.WriteFile(j.f.absSource, data, mode); err != nil {
			return plan, err
		}
		if j.plist {
			// Stored as XML; macOS reads either, but keep the original binary form.
			env.Run("plutil", "-convert", "binary1", j.f.absSource)
		}
	}
	return plan, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}

// initRepo clones the private repo, or starts an empty one when no URL is given.
func initRepo(env *dotEnv, url string) error {
	if isGitRepo(env.RepoDir) {
		if url != "" && strings.TrimSpace(gitOut(env.RepoDir, "remote")) == "" {
			_, err := git(env.RepoDir, "remote", "add", "origin", url)
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(env.RepoDir), 0o755); err != nil {
		return err
	}
	if url != "" {
		if out, err := git(filepath.Dir(env.RepoDir), "clone", "-q", url, env.RepoDir); err != nil {
			return fmt.Errorf("git clone: %v: %s", err, out)
		}
		return nil
	}
	if err := os.MkdirAll(env.RepoDir, 0o755); err != nil {
		return err
	}
	_, err := git(env.RepoDir, "init", "-q", "-b", "main")
	return err
}

// writeRepoReadme keeps a map of the repo: what each file is and where it goes back.
func writeRepoReadme(env *dotEnv, files []*configFile) error {
	var b bytes.Buffer
	b.WriteString("# dotfiles\n\n")
	b.WriteString("Configs saved by [basecamp](https://github.com/webdevmario/basecamp) (`basecamp dotfiles save`).\n")
	b.WriteString("Keep this repo private. basecamp refuses files that look like they hold secrets, and never copies SSH keys.\n\n")
	b.WriteString("## Restore on a new Mac\n\n")
	b.WriteString("```bash\nbasecamp dotfiles init <this repo's git URL>\nbasecamp dotfiles restore          # shows what would change\nbasecamp dotfiles restore --apply  # writes files, backing up what it replaces\n```\n\n")
	b.WriteString("Then install packages and extensions:\n\n```bash\n")
	for _, e := range env.Manifest {
		if e.RestoreHint != "" {
			b.WriteString(e.RestoreHint + "\n")
		}
	}
	b.WriteString("```\n\n## Files\n\n| File | Goes to |\n|---|---|\n")
	rows := map[string]string{}
	for _, f := range files {
		if f.saved == nil && f.Status != statusNew && f.Status != statusChanged {
			continue
		}
		if f.Status == statusBlocked && f.saved == nil {
			continue
		}
		target := f.Source
		if f.Generated {
			target = "(generated)"
		}
		rows[f.RepoPath] = target
	}
	keys := make([]string, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "| `%s` | %s |\n", k, strings.ReplaceAll(rows[k], "|", "\\|"))
	}
	return os.WriteFile(filepath.Join(env.RepoDir, "README.md"), b.Bytes(), 0o644)
}

// ---- git helpers ----

func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func gitOut(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, _ := cmd.Output()
	return string(out)
}

// unifiedDiff shows saved -> current as hunks, via git (no extra dependency).
func unifiedDiff(saved, current []byte, name string) string {
	tmp, err := os.MkdirTemp("", "basecamp-diff-")
	if err != nil {
		return ""
	}
	defer os.RemoveAll(tmp)
	a, b := filepath.Join(tmp, "saved"), filepath.Join(tmp, "current")
	os.WriteFile(a, saved, 0o600)
	os.WriteFile(b, current, 0o600)
	if saved == nil {
		a = os.DevNull
	}
	if current == nil {
		b = os.DevNull
	}
	cmd := exec.Command("git", "diff", "--no-index", "--no-color", "-U3", a, b)
	out, _ := cmd.Output() // exit 1 means "differs"
	s := string(out)
	if i := strings.Index(s, "\n@@"); i >= 0 {
		s = s[i+1:]
	} else if strings.HasPrefix(s, "@@") {
		// already starts at a hunk
	} else {
		s = ""
	}
	const max = 200_000
	if len(s) > max {
		s = s[:max] + "\n... diff truncated\n"
	}
	return s
}
