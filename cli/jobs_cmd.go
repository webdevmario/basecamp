package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// jobsState persists between runs (~/.basecamp/jobs-state.json): launchd's run counters, to
// notice runs that left no log, and which red states were already notified.
type jobsState struct {
	Runs     map[string]runCount `json:"runs"`
	Notified map[string]string   `json:"notified"`
}

type runCount struct {
	Runs      int    `json:"runs"`
	ChangedAt string `json:"changedAt,omitempty"`
}

func runJobs(args []string) {
	fsFlags := flag.NewFlagSet("jobs", flag.ExitOnError)
	out := fsFlags.String("o", filepath.Join(defaultDataDir(), "jobs.json"), `Where to write the report ("-" for stdout)`)
	pretty := fsFlags.Bool("pretty", false, "Pretty-print JSON")
	cfgPath := fsFlags.String("config", "~/.basecamp/jobs-config.json", "Per-job settings (log globs, backup snapshots, offsite job)")
	statePath := fsFlags.String("state", "~/.basecamp/jobs-state.json", "State kept between runs")
	notify := fsFlags.Bool("notify", true, "Send a macOS notification when a job turns red (once per failure)")
	background := fsFlags.Bool("launchd", false, "Running from launchd: skip privacy-protected folders (~/Documents, iCloud Drive), read logs instead")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: basecamp jobs [flags]")
		fmt.Fprintln(os.Stderr, "Checks every user LaunchAgent and the backups it makes, writes jobs.json, and notifies")
		fmt.Fprintln(os.Stderr, "once when a job turns red. scripts/install-service.sh runs it every 15 minutes.")
		fsFlags.PrintDefaults()
	}
	fsFlags.Parse(args)

	cfg, err := loadJobsConfig(*cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	env := defaultJobsEnv(cfg)
	env.SkipProtected = *background
	state := loadJobsState(expandHome(*statePath))
	env.Observed = observeRuns(&state, env)

	rep := buildJobsReport(env)

	if *notify {
		if msg := pendingNotification(&state, rep); msg != nil {
			if err := sendNotification(msg.title, msg.body); err != nil {
				fmt.Fprintln(os.Stderr, "notification failed:", err)
			}
		}
	}
	if err := saveJSON(expandHome(*statePath), state, true); err != nil {
		fmt.Fprintln(os.Stderr, "Error saving state:", err)
	}

	if *out == "-" {
		enc := json.NewEncoder(os.Stdout)
		if *pretty {
			enc.SetIndent("", "  ")
		}
		enc.Encode(rep)
		return
	}
	if err := saveJSON(*out, rep, *pretty); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "jobs: %d scheduled/services, %d red, %d warning -> %s\n",
		rep.Summary.Total, rep.Summary.Red, rep.Summary.Warn, *out)
}

func loadJobsState(path string) jobsState {
	s := jobsState{}
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, &s)
	}
	if s.Runs == nil {
		s.Runs = map[string]runCount{}
	}
	if s.Notified == nil {
		s.Notified = map[string]string{}
	}
	return s
}

// observeRuns records when launchd's run counter went up since the last scan. That time is an
// upper bound for the run (within one scan interval), used when a job writes no log.
func observeRuns(state *jobsState, env jobsEnv) map[string]time.Time {
	observed := map[string]time.Time{}
	entries, _ := os.ReadDir(env.AgentsDir)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".plist") {
			continue
		}
		data, err := env.ReadPlist(filepath.Join(env.AgentsDir, e.Name()))
		if err != nil {
			continue
		}
		p, err := parsePlistJSON(data)
		if err != nil || p.Label == "" {
			continue
		}
		info := parseLaunchctlPrint(env.Launchctl(p.Label))
		prev, seen := state.Runs[p.Label]
		next := runCount{Runs: info.Runs, ChangedAt: prev.ChangedAt}
		if seen && info.Runs > prev.Runs {
			next.ChangedAt = fmtTime(env.Now)
		}
		state.Runs[p.Label] = next
		if t := parseAnyTime(next.ChangedAt, env.Loc); !t.IsZero() {
			observed[p.Label] = t
		}
	}
	return observed
}

type notification struct{ title, body string }

// notifyKey changes when there is a new failure: a new failed run (lastRun moves), a new missed
// slot (dueSlot moves), or a different status. Staying red with the same key stays quiet.
func notifyKey(j Job) string {
	return j.Status + "|" + j.LastRun + "|" + j.DueSlot
}

// pendingNotification updates state.Notified and returns one combined notification for jobs
// that newly turned red, or nil.
func pendingNotification(state *jobsState, rep JobsReport) *notification {
	var fresh []Job
	current := map[string]bool{}
	for _, j := range rep.Jobs {
		if !j.Red {
			continue
		}
		current[j.Label] = true
		key := notifyKey(j)
		if state.Notified[j.Label] != key {
			state.Notified[j.Label] = key
			fresh = append(fresh, j)
		}
	}
	for label := range state.Notified {
		if !current[label] {
			delete(state.Notified, label) // recovered: the next failure notifies again
		}
	}
	if len(fresh) == 0 {
		return nil
	}
	sort.Slice(fresh, func(a, b int) bool { return fresh[a].Label < fresh[b].Label })
	if len(fresh) == 1 {
		j := fresh[0]
		body := j.Status
		if len(j.Reasons) > 0 {
			body = j.Reasons[0]
		}
		return &notification{title: fmt.Sprintf("Job %s: %s", j.Label, j.Status), body: body}
	}
	names := make([]string, len(fresh))
	for i, j := range fresh {
		names[i] = fmt.Sprintf("%s (%s)", j.Label, j.Status)
	}
	return &notification{title: fmt.Sprintf("%d jobs need attention", len(fresh)), body: strings.Join(names, ", ")}
}

func sendNotification(title, body string) error {
	// argv keeps quotes in the text from breaking the AppleScript
	return exec.Command("osascript",
		"-e", "on run argv",
		"-e", `display notification (item 2 of argv) with title "basecamp" subtitle (item 1 of argv) sound name "Basso"`,
		"-e", "end run",
		title, body).Run()
}

func saveJSON(path string, v any, pretty bool) error {
	var data []byte
	var err error
	if pretty {
		data, err = json.MarshalIndent(v, "", "  ")
	} else {
		data, err = json.Marshal(v)
	}
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data)
}
