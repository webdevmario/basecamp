package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const dotfilesUsage = `Usage: basecamp dotfiles <command> [flags]

Keeps your configs saved in a private git repo (default ~/.basecamp/dotfiles,
or $BASECAMP_DOTFILES_DIR). Files that look like they hold secrets are refused,
and SSH keys are never copied.

Commands:
  status [--json]           In sync, changed since last save, or not saved yet
  diff [name...]            What changed since the last save
  save [--push]             Copy changed configs into the repo and commit
  init [git-url]            Clone the private repo (or start an empty one)
  restore [--apply]         Copy saved configs back (dry run without --apply)
  schedule [--remove]       Save and push every night at 03:00 (launchd)
`

func runDotfiles(args []string) {
	if len(args) == 0 {
		args = []string{"status"}
	}
	env := defaultDotEnv()
	cmd, rest := args[0], args[1:]
	fs := flag.NewFlagSet("dotfiles "+cmd, flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "JSON output")
	push := fs.Bool("push", false, "Push to the remote after committing")
	apply := fs.Bool("apply", false, "Write files (restore)")
	remove := fs.Bool("remove", false, "Remove the nightly job (schedule)")
	at := fs.String("at", "03:00", "Time of day for the nightly save (schedule)")
	fs.Usage = func() { fmt.Fprint(os.Stderr, dotfilesUsage) }
	fs.Parse(rest)

	switch cmd {
	case "status":
		st := buildStatus(env, *jsonOut)
		if *jsonOut {
			dotPrintJSON(st)
			return
		}
		printStatus(st)
	case "diff":
		st := buildStatus(env, true)
		want := map[string]bool{}
		for _, a := range fs.Args() {
			want[a] = true
		}
		shown := 0
		for _, f := range st.Files {
			if f.Diff == "" || (len(want) > 0 && !want[f.EntryID] && !want[f.Label] && !want[f.RepoPath]) {
				continue
			}
			fmt.Printf("── %s (%s) %s\n%s\n", f.RepoPath, f.Status, f.Source, f.Diff)
			shown++
		}
		if shown == 0 {
			fmt.Println("No changes since the last save.")
		}
	case "save":
		res, err := saveConfigs(env, *push)
		if *jsonOut {
			dotPrintJSON(res)
		} else {
			printSave(res)
		}
		if err != nil {
			dotFatal(err)
		}
		if res.PushError != "" {
			os.Exit(2)
		}
	case "init":
		url := ""
		if fs.NArg() > 0 {
			url = fs.Arg(0)
		}
		if err := initRepo(env, url); err != nil {
			dotFatal(err)
		}
		fmt.Printf("Dotfiles repo ready at %s\n", env.RepoDir)
	case "restore":
		plan, err := restoreConfigs(env, *apply)
		if err != nil {
			dotFatal(err)
		}
		printRestore(plan, *apply)
	case "schedule":
		if err := scheduleNightly(env, *remove, *at); err != nil {
			dotFatal(err)
		}
	case "help", "-h", "--help":
		fmt.Print(dotfilesUsage)
	default:
		fmt.Fprintf(os.Stderr, "Unknown dotfiles command: %s\n\n%s", cmd, dotfilesUsage)
		os.Exit(1)
	}
}

func dotPrintJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func dotFatal(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}

var statusWords = map[string]string{
	statusSynced:  "in sync",
	statusChanged: "changed since last save",
	statusNew:     "not saved yet",
	statusBlocked: "refused: looks like a secret",
	statusMissing: "saved, but gone from this Mac",
	statusError:   "skipped",
}

func printStatus(st DotfilesStatus) {
	if !st.Initialized {
		fmt.Printf("No dotfiles repo yet at %s. Run `basecamp dotfiles init <git-url>`.\n\n", st.Repo)
	} else {
		last := st.LastSave
		if last == "" {
			last = "never"
		}
		fmt.Printf("Repo %s (%s), last save %s", st.Repo, orNone(st.Remote), last)
		if st.Unpushed > 0 {
			fmt.Printf(", %d commit(s) not pushed", st.Unpushed)
		}
		fmt.Print("\n\n")
	}
	group := ""
	for _, f := range st.Files {
		if f.Group != group {
			group = f.Group
			fmt.Printf("%s\n", group)
		}
		line := fmt.Sprintf("  %-38s %s", f.RepoPath, statusWords[f.Status])
		if len(f.Secrets) > 0 {
			line += " (" + describeSecrets(f.Secrets) + ")"
		}
		if f.Error != "" {
			line += " (" + f.Error + ")"
		}
		fmt.Println(line)
	}
	fmt.Println()
	fmt.Printf("%d in sync, %d changed, %d not saved, %d refused, %d gone\n",
		st.Counts[statusSynced], st.Counts[statusChanged], st.Counts[statusNew], st.Counts[statusBlocked], st.Counts[statusMissing])
}

func describeSecrets(s []SecretFinding) string {
	parts := make([]string, 0, len(s))
	for _, f := range s {
		parts = append(parts, fmt.Sprintf("line %d: %s", f.Line, f.Rule))
	}
	return strings.Join(parts, "; ")
}

func printSave(res SaveResult) {
	if len(res.Saved) == 0 {
		fmt.Println("Nothing changed since the last save.")
	} else {
		fmt.Printf("Saved %d: %s\n", len(res.Saved), strings.Join(res.Saved, ", "))
	}
	for _, s := range res.Skipped {
		msg := s.Reason
		if len(s.Secrets) > 0 {
			msg += " (" + describeSecrets(s.Secrets) + "; add `basecamp:allow` to a line if it is a false positive)"
		}
		fmt.Printf("Skipped %s: %s\n", s.RepoPath, msg)
	}
	if res.Committed {
		fmt.Printf("Committed %s\n", res.Commit)
	}
	if res.Pushed {
		fmt.Println("Pushed")
	}
	if res.PushError != "" {
		fmt.Printf("Push failed: %s\n", res.PushError)
	}
}

func printRestore(plan RestorePlan, applied bool) {
	verb := "Would write"
	if applied {
		verb = "Wrote"
	}
	if len(plan.Write) == 0 {
		fmt.Println("Every saved config already matches this Mac.")
	}
	for _, s := range plan.Write {
		note := ""
		if s.Replaces {
			note = " (replaces the current file)"
		}
		fmt.Printf("%s %s -> %s%s\n", verb, s.RepoPath, s.Target, note)
	}
	if plan.Backup != "" {
		fmt.Printf("Backed up replaced files to %s\n", plan.Backup)
	}
	if len(plan.Manual) > 0 {
		fmt.Println("\nBy hand:")
		for _, m := range plan.Manual {
			fmt.Println("  " + m)
		}
	}
	if !applied && len(plan.Write) > 0 {
		fmt.Println("\nRun with --apply to write these files.")
	}
}

func orNone(s string) string {
	if s == "" {
		return "no remote"
	}
	return s
}

// ---- nightly job ----

const dotfilesJobLabel = "com.basecamp-dotfiles"

// scheduleNightly installs a LaunchAgent that runs `basecamp dotfiles save --push`.
// It points at this binary, so run it from a stable install (~/.basecamp/app/basecamp),
// not from a worktree build.
func scheduleNightly(env *dotEnv, remove bool, at string) error {
	agents := filepath.Join(env.Home, "Library", "LaunchAgents")
	plistPath := filepath.Join(agents, dotfilesJobLabel+".plist")
	domain := fmt.Sprintf("gui/%d", os.Getuid())
	exec.Command("launchctl", "bootout", domain+"/"+dotfilesJobLabel).Run()
	if remove {
		os.Remove(plistPath)
		fmt.Printf("Removed %s\n", dotfilesJobLabel)
		return nil
	}

	var hour, minute int
	if _, err := fmt.Sscanf(at, "%d:%d", &hour, &minute); err != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return fmt.Errorf("--at wants HH:MM, got %q", at)
	}
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	bin, _ = filepath.EvalSymlinks(bin)
	if strings.Contains(bin, "/.claude/worktrees/") || strings.Contains(bin, "/go-build") {
		return fmt.Errorf("%s is a temporary build; install basecamp somewhere stable (e.g. ~/.basecamp/app/basecamp) and run schedule from there", bin)
	}
	logs := filepath.Join(env.Home, ".basecamp", "logs")
	os.MkdirAll(logs, 0o755)
	os.MkdirAll(agents, 0o755)
	plist := nightlyPlist(bin, env.RepoDir, filepath.Join(logs, dotfilesJobLabel+".log"), hour, minute)
	if err := os.WriteFile(plistPath, []byte(plist), 0o644); err != nil {
		return err
	}
	if out, err := exec.Command("launchctl", "bootstrap", domain, plistPath).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootstrap: %v: %s", err, out)
	}
	fmt.Printf("Installed %s: saves and pushes every day at %02d:%02d (log in %s)\n", dotfilesJobLabel, hour, minute, logs)
	return nil
}

func nightlyPlist(bin, repo, logPath string, hour, minute int) string {
	esc := func(s string) string {
		r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
		return r.Replace(s)
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string><string>dotfiles</string><string>save</string><string>--push</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>PATH</key><string>/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    <key>BASECAMP_DOTFILES_DIR</key><string>%s</string>
  </dict>
  <key>StartCalendarInterval</key>
  <dict>
    <key>Hour</key><integer>%d</integer>
    <key>Minute</key><integer>%d</integer>
  </dict>
  <key>StandardOutPath</key><string>%s</string>
  <key>StandardErrorPath</key><string>%s</string>
</dict>
</plist>
`, dotfilesJobLabel, esc(bin), esc(repo), hour, minute, esc(logPath), esc(logPath))
}
