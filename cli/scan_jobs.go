package main

// Jobs: every user LaunchAgent on this Mac, whether its last run worked, and backups that are stale.
//
// Sources, per job:
//   - the plist in ~/Library/LaunchAgents (label, schedule, command, log paths)
//   - `launchctl print gui/<uid>/<label>` (loaded, running pid, runs since load, last exit code)
//   - its logs (last modified, last lines, error markers)
//   - an optional status file the job writes itself: ~/Library/Application Support/jobs/<label>.json
//   - optional per-job settings in ~/.basecamp/jobs-config.json (extra log glob, backup snapshots)
//
// Everything here is pure where it can be (parse*, derive*, slot math) so it can be tested
// without launchd; buildJobsReport does the I/O.

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---------- output ----------

type JobsReport struct {
	GeneratedAt string       `json:"generatedAt"`
	Host        string       `json:"host"`
	Summary     JobsSummary  `json:"summary"`
	Jobs        []Job        `json:"jobs"`
	ThirdParty  []ThirdParty `json:"thirdParty"`
	Offsite     *OffsiteInfo `json:"offsite,omitempty"`
	Findings    []Note       `json:"findings"`
	StatusDir   string       `json:"statusDir"`
	// Labels of jobs that would benefit from writing a status file (see README, "Job status files").
	WantsStatusFile []string `json:"wantsStatusFile"`
}

type JobsSummary struct {
	OK       bool           `json:"ok"`
	Total    int            `json:"total"`
	Red      int            `json:"red"`
	Warn     int            `json:"warn"`
	ByStatus map[string]int `json:"byStatus"`
}

type Job struct {
	Label       string   `json:"label"`
	Kind        string   `json:"kind"` // calendar, interval, service, watch, at-load, on-demand
	Schedule    string   `json:"schedule"`
	Slots       []string `json:"slots,omitempty"` // daily "HH:MM" times, for the night timeline
	IntervalSec int      `json:"intervalSec,omitempty"`
	Command     string   `json:"command"`
	Plist       string   `json:"plist"`
	Logs        []string `json:"logs"`

	Loaded   bool   `json:"loaded"`
	State    string `json:"state,omitempty"`
	PID      int    `json:"pid,omitempty"`
	Runs     int    `json:"runs"`
	LastExit *int   `json:"lastExit,omitempty"`

	LastRun       string `json:"lastRun,omitempty"`
	LastRunSource string `json:"lastRunSource,omitempty"` // status-file, log, launchd
	RunStart      string `json:"runStart,omitempty"`      // first/last timestamp of the last run, from the log
	RunEnd        string `json:"runEnd,omitempty"`
	DueSlot       string `json:"dueSlot,omitempty"` // latest slot it should already have run for
	NextRun       string `json:"nextRun,omitempty"`

	Status  string   `json:"status"`
	Red     bool     `json:"red"`
	Reasons []string `json:"reasons"`
	Notes   []Note   `json:"notes"`

	Report    *JobStatusFile `json:"report,omitempty"`
	Log       *LogInfo       `json:"log,omitempty"`
	Errors    []string       `json:"errors"`
	Warnings  []string       `json:"warnings"`
	Snapshots []Snapshot     `json:"snapshots,omitempty"`
	IsOffsite bool           `json:"isOffsite,omitempty"`
}

type Note struct {
	Level string `json:"level"` // info, warn, error
	Label string `json:"label,omitempty"`
	Text  string `json:"text"`
}

type LogInfo struct {
	Path    string   `json:"path"`
	ModTime string   `json:"modTime,omitempty"`
	Size    int64    `json:"size"`
	Tail    []string `json:"tail"`
	Error   string   `json:"error,omitempty"`
}

// JobStatusFile is the optional convention: a job writes
// ~/Library/Application Support/jobs/<label>.json when it finishes.
type JobStatusFile struct {
	OK       *bool  `json:"ok"`
	Started  string `json:"started,omitempty"`
	Finished string `json:"finished,omitempty"`
	Summary  string `json:"summary,omitempty"`
}

type Snapshot struct {
	Name     string  `json:"name"`
	File     string  `json:"file,omitempty"`
	At       string  `json:"at,omitempty"`
	AgeHours float64 `json:"ageHours,omitempty"`
	Stale    bool    `json:"stale"`
	Source   string  `json:"source,omitempty"` // dir, log
	Error    string  `json:"error,omitempty"`
	// Offsite: "in-image" (newest image has it), "next-image" (included, but newer than the newest image),
	// "not-included" (the offsite job doesn't pick it up), "unknown".
	Offsite      string `json:"offsite,omitempty"`
	OffsiteImage string `json:"offsiteImage,omitempty"`
}

type OffsiteInfo struct {
	Label    string  `json:"label"`
	Image    string  `json:"image,omitempty"`
	At       string  `json:"at,omitempty"`
	AgeHours float64 `json:"ageHours,omitempty"`
	Source   string  `json:"source,omitempty"`
	Error    string  `json:"error,omitempty"`
}

type ThirdParty struct {
	Label  string `json:"label"`
	Kind   string `json:"kind"`
	Plist  string `json:"plist"`
	Loaded bool   `json:"loaded"`
}

// ---------- config ----------

type JobsConfig struct {
	StaleAfterHours float64              `json:"staleAfterHours"`
	ThirdParty      []string             `json:"thirdParty"` // label prefixes
	Offsite         *OffsiteConfig       `json:"offsite"`
	Jobs            map[string]JobConfig `json:"jobs"`
}

type OffsiteConfig struct {
	Label   string `json:"label"`
	Dir     string `json:"dir"`
	Pattern string `json:"pattern"`
	Script  string `json:"script"` // searched for snapshot names to tell what the image includes
}

type JobConfig struct {
	Log       string           `json:"log"` // glob; its newest match is the main log
	Snapshots []SnapshotConfig `json:"snapshots"`
	Ignore    bool             `json:"ignore"`
}

type SnapshotConfig struct {
	Name    string `json:"name"`
	Dir     string `json:"dir"`
	Pattern string `json:"pattern"` // matched against file names
	Offsite *bool  `json:"offsite"` // nil: look for Name in the offsite script
}

var defaultThirdParty = []string{
	"com.google.", "com.valvesoftware.", "homebrew.mxcl.", "com.1password.", "com.agilebits.",
	"com.microsoft.", "com.adobe.", "com.dropbox.", "com.spotify.", "us.zoom.", "com.docker.",
	"com.jetbrains.", "org.virtualbox.", "com.github.", "keybase.", "com.openai.", "com.anthropic.",
}

func loadJobsConfig(path string) (JobsConfig, error) {
	cfg := JobsConfig{}
	data, err := os.ReadFile(expandHome(path))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return cfg, err
	}
	if err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("%s: %w", path, err)
		}
	}
	if cfg.StaleAfterHours <= 0 {
		cfg.StaleAfterHours = 48 // the offsite job's own rule: snapshots older than 2 days
	}
	cfg.ThirdParty = append(cfg.ThirdParty, defaultThirdParty...)
	if cfg.Jobs == nil {
		cfg.Jobs = map[string]JobConfig{}
	}
	return cfg, nil
}

// ---------- plist ----------

type plistAgent struct {
	Label                 string          `json:"Label"`
	Program               string          `json:"Program"`
	ProgramArguments      []string        `json:"ProgramArguments"`
	StartCalendarInterval json.RawMessage `json:"StartCalendarInterval"`
	StartInterval         int             `json:"StartInterval"`
	KeepAlive             json.RawMessage `json:"KeepAlive"`
	RunAtLoad             bool            `json:"RunAtLoad"`
	WatchPaths            []string        `json:"WatchPaths"`
	StandardOutPath       string          `json:"StandardOutPath"`
	StandardErrorPath     string          `json:"StandardErrorPath"`
	Disabled              bool            `json:"Disabled"`
}

// CalSpec is one StartCalendarInterval entry; nil fields are wildcards.
type CalSpec struct {
	Minute  *int `json:"Minute"`
	Hour    *int `json:"Hour"`
	Day     *int `json:"Day"`
	Weekday *int `json:"Weekday"`
	Month   *int `json:"Month"`
}

// readPlistJSON converts any plist (XML or binary) to JSON with plutil.
func readPlistJSON(path string) ([]byte, error) {
	out, err := exec.Command("plutil", "-convert", "json", "-o", "-", path).Output()
	if err != nil {
		lint, _ := exec.Command("plutil", "-lint", path).CombinedOutput()
		msg := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(lint)), path+":"))
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("not a valid plist: %s", strings.Trim(msg, "()"))
	}
	return out, nil
}

func parsePlistJSON(data []byte) (plistAgent, error) {
	var p plistAgent
	err := json.Unmarshal(data, &p)
	return p, err
}

func (p plistAgent) calendar() ([]CalSpec, error) {
	raw := strings.TrimSpace(string(p.StartCalendarInterval))
	if raw == "" || raw == "null" {
		return nil, nil
	}
	if strings.HasPrefix(raw, "[") {
		var specs []CalSpec
		err := json.Unmarshal(p.StartCalendarInterval, &specs)
		return specs, err
	}
	var one CalSpec
	err := json.Unmarshal(p.StartCalendarInterval, &one)
	return []CalSpec{one}, err
}

// keepAlive: true, or a non-empty conditions dict (also a long-running service).
func (p plistAgent) keepAlive() bool {
	raw := strings.TrimSpace(string(p.KeepAlive))
	return raw == "true" || (strings.HasPrefix(raw, "{") && raw != "{}")
}

func (p plistAgent) kind() string {
	if specs, _ := p.calendar(); len(specs) > 0 {
		return "calendar"
	}
	switch {
	case p.StartInterval > 0:
		return "interval"
	case p.keepAlive():
		return "service"
	case len(p.WatchPaths) > 0:
		return "watch"
	case p.RunAtLoad:
		return "at-load"
	}
	return "on-demand"
}

// command shows what runs: the -c string for `sh -c`, else the joined arguments.
func (p plistAgent) command() string {
	args := p.ProgramArguments
	if len(args) == 0 && p.Program != "" {
		return p.Program
	}
	for i, a := range args {
		if a == "-c" && i+1 < len(args) && i <= 2 {
			return args[i+1]
		}
	}
	quoted := make([]string, len(args))
	for i, a := range args {
		if strings.ContainsAny(a, " \t") {
			a = `"` + a + `"`
		}
		quoted[i] = a
	}
	return strings.Join(quoted, " ")
}

func (p plistAgent) program() string {
	if p.Program != "" {
		return p.Program
	}
	if len(p.ProgramArguments) > 0 {
		return p.ProgramArguments[0]
	}
	return ""
}

func isThirdParty(label string, p plistAgent, prefixes []string) bool {
	for _, pre := range prefixes {
		if strings.HasPrefix(label, pre) {
			return true
		}
	}
	prog := p.program()
	return strings.HasPrefix(prog, "/Applications/") || strings.Contains(prog, ".app/Contents/")
}

// ---------- schedule ----------

func (c CalSpec) matches(t time.Time) bool {
	eq := func(f *int, v int) bool { return f == nil || *f == v }
	if !eq(c.Minute, t.Minute()) || !eq(c.Hour, t.Hour()) || !eq(c.Month, int(t.Month())) {
		return false
	}
	dayOK := eq(c.Day, t.Day())
	wdOK := c.Weekday == nil || *c.Weekday%7 == int(t.Weekday())
	return dayOK && wdOK
}

const slotSearchMinutes = 32 * 24 * 60

// prevSlot returns the latest slot at or before t (minute precision), or zero if none in 32 days.
func prevSlot(specs []CalSpec, t time.Time) time.Time {
	m := t.Truncate(time.Minute)
	for i := 0; i < slotSearchMinutes; i++ {
		for _, s := range specs {
			if s.matches(m) {
				return m
			}
		}
		m = m.Add(-time.Minute)
	}
	return time.Time{}
}

// nextSlot returns the first slot strictly after t.
func nextSlot(specs []CalSpec, t time.Time) time.Time {
	m := t.Truncate(time.Minute).Add(time.Minute)
	for i := 0; i < slotSearchMinutes; i++ {
		for _, s := range specs {
			if s.matches(m) {
				return m
			}
		}
		m = m.Add(time.Minute)
	}
	return time.Time{}
}

// dueSlot is the latest slot whose run should be visible by now. A run gets `grace` to show up;
// launchd runs a slot missed during sleep at wake, so the latest slot before the last wake gets
// grace after the wake instead. Until then the slot before it is the one that counts.
func dueSlot(specs []CalSpec, now, lastWake time.Time, grace time.Duration) time.Time {
	s := prevSlot(specs, now)
	if s.IsZero() {
		return s
	}
	deadline := s.Add(grace)
	if lastWake.After(s) {
		deadline = lastWake.Add(grace)
	}
	if now.Before(deadline) {
		return prevSlot(specs, s.Add(-time.Minute))
	}
	return s
}

func dailySlots(specs []CalSpec) []string {
	var out []string
	for _, s := range specs {
		if s.Hour != nil && s.Minute != nil && s.Day == nil && s.Weekday == nil && s.Month == nil {
			out = append(out, fmt.Sprintf("%02d:%02d", *s.Hour, *s.Minute))
		}
	}
	sort.Strings(out)
	return out
}

var weekdayNames = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

func describeCalendar(specs []CalSpec) string {
	parts := make([]string, 0, len(specs))
	for _, s := range specs {
		var b strings.Builder
		switch {
		case s.Hour != nil && s.Minute != nil:
			fmt.Fprintf(&b, "%02d:%02d", *s.Hour, *s.Minute)
		case s.Hour != nil:
			fmt.Fprintf(&b, "every minute of %02d:00", *s.Hour)
		case s.Minute != nil:
			fmt.Fprintf(&b, "hourly at :%02d", *s.Minute)
		default:
			b.WriteString("every minute")
		}
		var when []string
		if s.Weekday != nil {
			when = append(when, weekdayNames[*s.Weekday%7]+"s")
		}
		if s.Day != nil {
			when = append(when, fmt.Sprintf("day %d", *s.Day))
		}
		if s.Month != nil {
			when = append(when, time.Month(*s.Month).String())
		}
		if len(when) == 0 && s.Hour != nil {
			when = append(when, "daily")
		}
		if len(when) > 0 {
			b.WriteString(" " + strings.Join(when, ", "))
		}
		parts = append(parts, b.String())
	}
	return strings.Join(parts, "; ")
}

func describeInterval(sec int) string {
	switch {
	case sec%3600 == 0:
		if sec == 3600 {
			return "every hour"
		}
		return fmt.Sprintf("every %d h", sec/3600)
	case sec%60 == 0:
		return fmt.Sprintf("every %d min", sec/60)
	}
	return fmt.Sprintf("every %d s", sec)
}

// ---------- launchctl ----------

type launchdInfo struct {
	Loaded     bool
	Properties string
	State      string
	PID      int
	Runs     int
	LastExit *int
}

// parseLaunchctlPrint reads the top-level fields of `launchctl print` (one tab deep; nested
// blocks reuse names like "state").
func parseLaunchctlPrint(out string) launchdInfo {
	info := launchdInfo{Loaded: strings.TrimSpace(out) != ""}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "\t\t") {
			continue
		}
		k, v, ok := strings.Cut(strings.TrimSpace(line), " = ")
		if !ok {
			continue
		}
		switch k {
		case "state":
			info.State = v
		case "properties":
			info.Properties = v
		case "pid":
			info.PID, _ = strconv.Atoi(v)
		case "runs":
			info.Runs, _ = strconv.Atoi(v)
		case "last exit code":
			// "0", "1: Operation not permitted", "(never exited)"
			num, _, _ := strings.Cut(v, ":")
			if n, err := strconv.Atoi(strings.TrimSpace(num)); err == nil {
				info.LastExit = &n
			}
		}
	}
	return info
}

// ---------- logs ----------

var lineTimeRe = regexp.MustCompile(`^[\[=\s]*(\d{4}-\d{2}-\d{2})[T ](\d{2}:\d{2}(?::\d{2})?)(\.\d+)?(Z|[+-]\d{2}:?\d{2})?`)

// lineTime parses a timestamp at the start of a log line ("2026-11-24T09:10:06.214Z",
// "2026-11-24 03:30:06", "[2026-11-24 03:30]", "=== 2026-11-24 05:00:05 ===").
func lineTime(line string, loc *time.Location) (time.Time, bool) {
	m := lineTimeRe.FindStringSubmatch(line)
	if m == nil {
		return time.Time{}, false
	}
	clock := m[2]
	if len(clock) == 5 {
		clock += ":00"
	}
	s := m[1] + "T" + clock
	zone := m[4]
	if zone == "" {
		t, err := time.ParseInLocation("2006-01-02T15:04:05", s, loc)
		return t, err == nil
	}
	if zone != "Z" && !strings.Contains(zone, ":") {
		zone = zone[:3] + ":" + zone[3:]
	}
	t, err := time.Parse(time.RFC3339, s+zone)
	return t, err == nil
}

func isErrorLine(l string) bool {
	t := strings.TrimSpace(l)
	if m := lineTimeRe.FindString(t); m != "" {
		t = strings.TrimSpace(t[len(m):])
	}
	return strings.HasPrefix(t, "!!") || strings.Contains(l, "ERROR") || strings.Contains(l, "FAILED") ||
		strings.HasPrefix(t, "Error:") || strings.Contains(l, "Traceback (most recent call last)")
}

func isWarningLine(l string) bool {
	return strings.Contains(l, "WARNING") || strings.Contains(l, "WARN ")
}

// lastRunLines picks the lines written by the last run: those stamped at or after `since`, plus
// unstamped lines that follow one. A log with no timestamps at all counts whole when the file
// was modified after `since`.
func lastRunLines(lines []string, since, modTime time.Time, loc *time.Location) (out []string, first, last time.Time) {
	stamped := false
	cur := time.Time{}
	for _, l := range lines {
		if t, ok := lineTime(l, loc); ok {
			stamped = true
			cur = t
			if !t.Before(since) {
				if first.IsZero() {
					first = t
				}
				last = t
			}
		}
		if !cur.IsZero() && !cur.Before(since) {
			out = append(out, l)
		}
	}
	if !stamped && !modTime.Before(since) {
		return lines, time.Time{}, time.Time{}
	}
	return out, first, last
}

func classifyLines(lines []string) (errs, warns []string) {
	for _, l := range lines {
		switch {
		case isErrorLine(l):
			errs = append(errs, clip(l, 300))
		case isWarningLine(l):
			warns = append(warns, clip(l, 300))
		}
	}
	return limitTail(errs, 8), limitTail(warns, 8)
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func limitTail(s []string, n int) []string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}

// tailFile returns up to n lines from the last 128 KB of a file.
func tailFile(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	const window = 128 * 1024
	off := int64(0)
	if st.Size() > window {
		off = st.Size() - window
	}
	buf := make([]byte, st.Size()-off)
	if _, err := f.ReadAt(buf, off); err != nil && len(buf) > 0 {
		return nil, err
	}
	text := strings.TrimRight(string(buf), "\n")
	if text == "" {
		return nil, nil
	}
	lines := strings.Split(text, "\n")
	if off > 0 && len(lines) > 1 {
		lines = lines[1:] // first line is likely partial
	}
	return limitTail(lines, n), nil
}

// ---------- snapshots ----------

var fileDateRe = regexp.MustCompile(`(\d{8})`)

// snapshotName is the literal prefix of a pattern ("^stacks-\d{8}..." -> "stacks").
func snapshotName(sc SnapshotConfig) string {
	if sc.Name != "" {
		return sc.Name
	}
	p := strings.TrimPrefix(sc.Pattern, "^")
	end := strings.IndexAny(p, `\.[(*+?{|$-`)
	if end > 0 {
		return p[:end]
	}
	return p
}

type snapHit struct {
	file string
	at   time.Time
	src  string
}

func newestInDir(dir string, re *regexp.Regexp) (snapHit, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return snapHit{}, err
	}
	var best snapHit
	for _, e := range entries {
		if e.IsDir() || !re.MatchString(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(best.at) {
			best = snapHit{file: e.Name(), at: info.ModTime(), src: "dir"}
		}
	}
	return best, nil
}

// newestInLog finds the latest log line naming a matching file. Used when the directory isn't
// readable (launchd jobs can't read ~/Documents) or the job only reports paths in its log.
func newestInLog(lines []string, re *regexp.Regexp, loc *time.Location) snapHit {
	var best snapHit
	for _, l := range lines {
		for _, tok := range strings.Fields(l) {
			name := filepath.Base(strings.Trim(tok, `"'(),`))
			if !re.MatchString(name) {
				continue
			}
			at, ok := lineTime(l, loc)
			if !ok {
				if m := fileDateRe.FindString(name); m != "" {
					at, _ = time.ParseInLocation("20060102", m, loc)
				}
			}
			if !at.IsZero() && !at.Before(best.at) {
				best = snapHit{file: name, at: at, src: "log"}
			}
		}
	}
	return best
}

func findSnapshot(dir string, re *regexp.Regexp, logLines []string, loc *time.Location) (snapHit, error) {
	hit, dirErr := newestInDir(expandHome(dir), re)
	if fromLog := newestInLog(logLines, re, loc); fromLog.at.After(hit.at) {
		hit = fromLog
	}
	if hit.file == "" {
		return hit, dirErr
	}
	return hit, nil
}

func describeReadError(err error) string {
	if errors.Is(err, fs.ErrPermission) {
		return "no permission to read the folder (background jobs can't read ~/Documents without Full Disk Access)"
	}
	if errors.Is(err, fs.ErrNotExist) {
		return "folder not found"
	}
	if err != nil {
		return err.Error()
	}
	return "no matching file"
}

// ---------- status ----------

var redStatuses = map[string]bool{"failed": true, "missed": true, "stale": true, "unloaded": true, "down": true}

type statusInput struct {
	Kind          string
	Loaded        bool
	PID           int
	Runs          int
	LastExit      *int
	ReportOK      *bool
	ReportSummary string
	Errors        []string
	Warnings      []string
	LastRun       time.Time
	Due           time.Time // calendar: latest due slot; zero when none since the job was added
	IntervalSec   int
	Now           time.Time
	Grace         time.Duration
	StaleSnaps    []string
	WarnNotes     int
}

// deriveStatus returns the job's status and every reason that applies, worst first.
// Order: unloaded, running, failed, missed/down, stale, warning, pending/unknown, ok.
func deriveStatus(in statusInput) (string, []string) {
	var reasons []string
	if !in.Loaded {
		return "unloaded", []string{"plist is in ~/Library/LaunchAgents but not loaded, so it never runs"}
	}

	failed := false
	if in.LastExit != nil && *in.LastExit != 0 {
		failed = true
		reasons = append(reasons, fmt.Sprintf("last run exited with code %d", *in.LastExit))
	}
	if in.ReportOK != nil && !*in.ReportOK {
		failed = true
		r := "status file says the last run failed"
		if in.ReportSummary != "" {
			r += ": " + in.ReportSummary
		}
		reasons = append(reasons, r)
	}
	if len(in.Errors) > 0 {
		failed = true
		reasons = append(reasons, fmt.Sprintf("%d error line(s) in the last run's log", len(in.Errors)))
	}

	switch in.Kind {
	case "service":
		if in.PID > 0 {
			if failed {
				return "running", reasons // a crash that launchd restarted from: keep it visible
			}
			return "running", nil
		}
		if failed {
			return "failed", reasons
		}
		return "down", append(reasons, "KeepAlive service is not running")
	case "calendar", "interval":
	default:
		if failed {
			return "failed", reasons
		}
		return "idle", nil
	}

	if in.PID > 0 {
		return "running", reasons
	}
	if failed {
		return "failed", reasons
	}

	missed, pending, unknown := false, false, false
	switch in.Kind {
	case "calendar":
		switch {
		case in.Due.IsZero():
			pending = true
		case in.LastRun.IsZero() && in.Runs > 0:
			unknown = true
		case in.LastRun.Before(in.Due.Add(-2 * time.Minute)):
			missed = true
			if in.LastRun.IsZero() {
				reasons = append(reasons, fmt.Sprintf("no run seen for the %s slot", in.Due.Format("Mon 15:04")))
			} else {
				reasons = append(reasons, fmt.Sprintf("no run since %s; the %s slot was missed",
					in.LastRun.Format("Mon 15:04"), in.Due.Format("Mon 15:04")))
			}
		}
	case "interval":
		limit := time.Duration(in.IntervalSec)*time.Second + in.Grace
		switch {
		case in.LastRun.IsZero() && in.Runs > 0:
			unknown = true
		case in.LastRun.IsZero():
			pending = true
		case in.Now.Sub(in.LastRun) > limit:
			missed = true
			reasons = append(reasons, fmt.Sprintf("no run since %s (runs %s)",
				in.LastRun.Format("Mon 15:04"), describeInterval(in.IntervalSec)))
		}
	}
	if missed {
		return "missed", reasons
	}
	if len(in.StaleSnaps) > 0 {
		return "stale", append(reasons, in.StaleSnaps...)
	}
	if len(in.Warnings) > 0 || in.WarnNotes > 0 {
		if len(in.Warnings) > 0 {
			reasons = append(reasons, fmt.Sprintf("%d warning line(s) in the last run's log", len(in.Warnings)))
		}
		return "warning", reasons
	}
	if pending {
		return "pending", append(reasons, "no scheduled slot has come up since it was added")
	}
	if unknown {
		return "unknown", append(reasons, fmt.Sprintf("launchd counts %d run(s) since load, but no log or status file says when", in.Runs))
	}
	return "ok", reasons
}

// ---------- build ----------

type jobsEnv struct {
	Now        time.Time
	Loc        *time.Location
	AgentsDir  string
	StatusDir  string
	UID        int
	LastWake   time.Time
	Grace      time.Duration
	Config     JobsConfig
	Launchctl  func(label string) string
	Observed   map[string]time.Time // label -> when basecamp saw launchd's run count go up
	ReadPlist  func(path string) ([]byte, error)
	OffsiteSrc string // text of the offsite script
}

func defaultJobsEnv(cfg JobsConfig) jobsEnv {
	home, _ := os.UserHomeDir()
	uid := os.Getuid()
	env := jobsEnv{
		Now:       time.Now(),
		Loc:       time.Local,
		AgentsDir: filepath.Join(home, "Library", "LaunchAgents"),
		StatusDir: filepath.Join(home, "Library", "Application Support", "jobs"),
		UID:       uid,
		LastWake:  sysctlTime("kern.waketime"),
		Grace:     30 * time.Minute,
		Config:    cfg,
		ReadPlist: readPlistJSON,
	}
	env.Launchctl = func(label string) string {
		return run("launchctl", "print", fmt.Sprintf("gui/%d/%s", uid, label))
	}
	if cfg.Offsite != nil && cfg.Offsite.Script != "" {
		if b, err := os.ReadFile(expandHome(cfg.Offsite.Script)); err == nil {
			env.OffsiteSrc = string(b)
		}
	}
	return env
}

// sysctlTime reads "{ sec = 1793061410, usec = 87585 } ..." (zero when it never happened).
func sysctlTime(name string) time.Time {
	out := run("sysctl", "-n", name)
	var sec, usec int64
	if _, err := fmt.Sscanf(out, "{ sec = %d, usec = %d }", &sec, &usec); err != nil || sec == 0 {
		return time.Time{}
	}
	return time.Unix(sec, usec*1000)
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func parseAnyTime(s string, loc *time.Location) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t
		}
	}
	return time.Time{}
}

func underTmp(p string) bool {
	return strings.HasPrefix(p, "/tmp/") || strings.HasPrefix(p, "/private/tmp/")
}

type logFile struct {
	path    string
	modTime time.Time
	size    int64
	lines   []string
	err     error
}

func readLog(path string) logFile {
	lf := logFile{path: path}
	st, err := os.Stat(path)
	if err != nil {
		lf.err = err
		return lf
	}
	lf.modTime, lf.size = st.ModTime(), st.Size()
	lf.lines, lf.err = tailFile(path, 400)
	return lf
}

func newestGlob(pattern string) string {
	matches, _ := filepath.Glob(expandHome(pattern))
	best, bestT := "", time.Time{}
	for _, m := range matches {
		if t := fileModTime(m); t.After(bestT) {
			best, bestT = m, t
		}
	}
	return best
}

func buildJobsReport(env jobsEnv) JobsReport {
	hostname, _ := os.Hostname()
	rep := JobsReport{
		GeneratedAt: fmtTime(env.Now),
		Host:        hostname,
		StatusDir:   env.StatusDir,
		Jobs:        []Job{},
		ThirdParty:  []ThirdParty{},
		Findings:    []Note{},
	}

	entries, err := os.ReadDir(env.AgentsDir)
	if err != nil {
		rep.Findings = append(rep.Findings, Note{Level: "error", Text: "can't read LaunchAgents: " + err.Error()})
	}

	type pending struct {
		job   Job
		agent plistAgent
		specs []CalSpec
		added time.Time
	}
	var all []pending
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".plist") || strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(env.AgentsDir, name)
		data, err := env.ReadPlist(path)
		if err != nil {
			// launchd is more forgiving than plutil (an unescaped & in a string, say), so the job may
			// still be loaded: list it from what launchctl knows, with the parse error as a warning.
			label := strings.TrimSuffix(name, ".plist")
			ld := parseLaunchctlPrint(env.Launchctl(label))
			kind := "unknown"
			if strings.Contains(ld.Properties, "keepalive") {
				kind = "service"
			}
			j := Job{Label: label, Kind: kind, Schedule: "unknown (plist unreadable)", Plist: path,
				Loaded: ld.Loaded, State: ld.State, PID: ld.PID, Runs: ld.Runs, LastExit: ld.LastExit,
				Logs: []string{}, Reasons: []string{}, Errors: []string{}, Warnings: []string{},
				Notes: []Note{{Level: "warn", Text: name + " is " + err.Error() + " (launchd may still load it, but plutil and other tools can't read it)"}}}
			all = append(all, pending{job: j, added: fileModTime(path)})
			continue
		}
		agent, err := parsePlistJSON(data)
		if err != nil {
			rep.Findings = append(rep.Findings, Note{Level: "warn", Label: name, Text: "unreadable plist: " + err.Error()})
			continue
		}
		label := agent.Label
		if label == "" {
			label = strings.TrimSuffix(name, ".plist")
		}
		if env.Config.Jobs[label].Ignore {
			continue
		}
		ld := parseLaunchctlPrint(env.Launchctl(label))
		if isThirdParty(label, agent, env.Config.ThirdParty) {
			rep.ThirdParty = append(rep.ThirdParty, ThirdParty{Label: label, Kind: agent.kind(), Plist: path, Loaded: ld.Loaded})
			continue
		}
		specs, _ := agent.calendar()
		j := Job{
			Label:    label,
			Kind:     agent.kind(),
			Command:  agent.command(),
			Plist:    path,
			Loaded:   ld.Loaded,
			State:    ld.State,
			PID:      ld.PID,
			Runs:     ld.Runs,
			LastExit: ld.LastExit,
			Logs:     []string{},
			Reasons:  []string{},
			Notes:    []Note{},
			Errors:   []string{},
			Warnings: []string{},
		}
		switch j.Kind {
		case "calendar":
			j.Schedule = describeCalendar(specs)
			j.Slots = dailySlots(specs)
		case "interval":
			j.IntervalSec = agent.StartInterval
			j.Schedule = describeInterval(agent.StartInterval)
		case "service":
			j.Schedule = "always on (KeepAlive)"
		case "watch":
			j.Schedule = "when watched paths change"
		case "at-load":
			j.Schedule = "once at login"
		default:
			j.Schedule = "on demand"
		}
		for _, p := range []string{agent.StandardOutPath, agent.StandardErrorPath} {
			if p != "" && !contains(j.Logs, p) {
				j.Logs = append(j.Logs, p)
			}
		}
		all = append(all, pending{job: j, agent: agent, specs: specs, added: fileModTime(path)})
	}

	var offsiteImage snapHit
	var offsiteJob string
	if oc := env.Config.Offsite; oc != nil {
		offsiteJob = oc.Label
		rep.Offsite = &OffsiteInfo{Label: oc.Label}
	}

	// First pass: logs, last run, snapshots. The offsite job goes first so others can compare.
	sort.SliceStable(all, func(a, b int) bool { return all[a].job.Label == offsiteJob && all[b].job.Label != offsiteJob })
	for i := range all {
		p := &all[i]
		j := &p.job
		jc := env.Config.Jobs[j.Label]
		logPaths := append([]string{}, j.Logs...)
		if jc.Log != "" {
			if g := newestGlob(jc.Log); g != "" {
				logPaths = append([]string{g}, logPaths...)
				j.Logs = append([]string{g}, j.Logs...)
			} else {
				j.Notes = append(j.Notes, Note{Level: "info", Text: "no file matches " + jc.Log})
			}
		}

		// Status file convention
		statusPath := filepath.Join(env.StatusDir, j.Label+".json")
		if b, err := os.ReadFile(statusPath); err == nil {
			var sf JobStatusFile
			if err := json.Unmarshal(b, &sf); err == nil {
				j.Report = &sf
			} else {
				j.Notes = append(j.Notes, Note{Level: "warn", Text: "status file isn't valid JSON: " + err.Error()})
			}
		}

		var logs []logFile
		var lastRun time.Time
		src := ""
		for _, lp := range logPaths {
			lf := readLog(expandHome(lp))
			logs = append(logs, lf)
			if lf.modTime.After(lastRun) && lf.size > 0 {
				lastRun, src = lf.modTime, "log"
			}
		}
		if j.Report != nil {
			t := parseAnyTime(j.Report.Finished, env.Loc)
			if t.IsZero() {
				t = parseAnyTime(j.Report.Started, env.Loc)
			}
			if !t.IsZero() && !t.Before(lastRun.Add(-time.Minute)) {
				lastRun, src = t, "status-file"
			}
		}
		if t, ok := env.Observed[j.Label]; ok && t.After(lastRun) {
			lastRun, src = t, "launchd"
		}
		if j.Kind == "service" || j.Kind == "at-load" {
			src = ""
			lastRun = time.Time{}
		}
		j.LastRun, j.LastRunSource = fmtTime(lastRun), src

		// Which log lines belong to the last run
		since := env.Now.Add(-26 * time.Hour)
		switch {
		case j.Report != nil && !parseAnyTime(j.Report.Started, env.Loc).IsZero():
			since = parseAnyTime(j.Report.Started, env.Loc).Add(-time.Minute)
		case j.Kind == "calendar" && !lastRun.IsZero():
			if s := prevSlot(p.specs, lastRun); !s.IsZero() && lastRun.Sub(s) < 20*time.Hour {
				since = s.Add(-2 * time.Minute)
			} else {
				since = lastRun.Add(-3 * time.Hour)
			}
		case j.Kind == "interval" && !lastRun.IsZero():
			since = lastRun.Add(-time.Duration(j.IntervalSec) * time.Second)
		case j.Kind == "service" || j.Kind == "at-load":
			since = env.Now.Add(-24 * time.Hour)
		}

		var allLines []string
		var main *logFile
		var runStart, runEnd time.Time
		for k := range logs {
			lf := &logs[k]
			allLines = append(allLines, lf.lines...)
			if underTmp(lf.path) {
				addNote(j, "warn", fmt.Sprintf("logs to %s, which is wiped on reboot", lf.path))
			}
			if lf.err != nil {
				continue
			}
			lines, first, last := lastRunLines(lf.lines, since, lf.modTime, env.Loc)
			errs, warns := classifyLines(lines)
			j.Errors = append(j.Errors, errs...)
			j.Warnings = append(j.Warnings, warns...)
			if !first.IsZero() && (runStart.IsZero() || first.Before(runStart)) {
				runStart = first
			}
			if last.After(runEnd) {
				runEnd = last
			}
			if lf.size > 0 && (main == nil || lf.modTime.After(main.modTime)) {
				main = lf
			}
		}
		if main == nil && len(logs) > 0 {
			main = &logs[0]
		}
		if j.Kind == "calendar" || j.Kind == "interval" {
			// Lines like "=== done 06:36:49 ===" carry no date; the log's last write marks the end.
			if main != nil && !runStart.IsZero() && main.modTime.After(runEnd) && main.modTime.Sub(runStart) < 12*time.Hour {
				runEnd = main.modTime
			}
			j.RunStart, j.RunEnd = fmtTime(runStart), fmtTime(runEnd)
		}
		if main != nil {
			li := &LogInfo{Path: main.path, ModTime: fmtTime(main.modTime), Size: main.size, Tail: limitTail(main.lines, 15)}
			if main.err != nil {
				li.Error = describeReadError(main.err)
			}
			if li.Tail == nil {
				li.Tail = []string{}
			}
			j.Log = li
		}
		if len(logPaths) == 0 && j.Kind != "unknown" && !strings.HasPrefix(j.Schedule, "unknown") {
			addNote(j, "info", "no log file configured (StandardOutPath / StandardErrorPath)")
		}

		// Backups
		if j.Label == offsiteJob && env.Config.Offsite != nil {
			j.IsOffsite = true
			oc := env.Config.Offsite
			re, err := regexp.Compile(oc.Pattern)
			if err == nil {
				hit, herr := findSnapshot(oc.Dir, re, allLines, env.Loc)
				offsiteImage = hit
				rep.Offsite.Image, rep.Offsite.At, rep.Offsite.Source = hit.file, fmtTime(hit.at), hit.src
				snap := Snapshot{Name: "offsite image", File: hit.file, At: fmtTime(hit.at), Source: hit.src}
				if hit.file == "" {
					snap.Error = describeReadError(herr)
					rep.Offsite.Error = snap.Error
				} else {
					age := env.Now.Sub(hit.at).Hours()
					snap.AgeHours, rep.Offsite.AgeHours = round1(age), round1(age)
					snap.Stale = age > env.Config.StaleAfterHours
				}
				j.Snapshots = append(j.Snapshots, snap)
			}
		}
		for _, sc := range jc.Snapshots {
			re, err := regexp.Compile(sc.Pattern)
			if err != nil {
				addNote(j, "warn", fmt.Sprintf("bad snapshot pattern %q: %v", sc.Pattern, err))
				continue
			}
			name := snapshotName(sc)
			hit, herr := findSnapshot(sc.Dir, re, allLines, env.Loc)
			snap := Snapshot{Name: name, File: hit.file, At: fmtTime(hit.at), Source: hit.src}
			if hit.file == "" {
				snap.Error = describeReadError(herr) + ": " + sc.Dir
				snap.Stale = true
			} else {
				age := env.Now.Sub(hit.at).Hours()
				snap.AgeHours = round1(age)
				snap.Stale = age > env.Config.StaleAfterHours
			}
			if env.Config.Offsite != nil {
				included := false
				if sc.Offsite != nil {
					included = *sc.Offsite
				} else if env.OffsiteSrc != "" {
					included = strings.Contains(env.OffsiteSrc, name)
				} else {
					snap.Offsite = "unknown"
				}
				if snap.Offsite == "" {
					switch {
					case !included:
						snap.Offsite = "not-included"
						addNote(j, "warn", fmt.Sprintf("%s snapshots are not in the offsite image (%s doesn't pick them up)", name, env.Config.Offsite.Label))
					case offsiteImage.file != "" && !hit.at.IsZero() && !offsiteImage.at.Before(hit.at):
						snap.Offsite, snap.OffsiteImage = "in-image", offsiteImage.file
					case hit.file != "":
						snap.Offsite = "next-image"
					default:
						snap.Offsite = "unknown"
					}
				}
			}
			j.Snapshots = append(j.Snapshots, snap)
		}
	}

	// Second pass: status
	rep.Summary.ByStatus = map[string]int{}
	for i := range all {
		p := &all[i]
		j := &p.job
		in := statusInput{
			Kind: j.Kind, Loaded: j.Loaded, PID: j.PID, Runs: j.Runs, LastExit: j.LastExit,
			Errors: j.Errors, Warnings: j.Warnings, LastRun: parseAnyTime(j.LastRun, env.Loc),
			IntervalSec: j.IntervalSec, Now: env.Now, Grace: env.Grace,
		}
		if j.Report != nil {
			in.ReportOK, in.ReportSummary = j.Report.OK, j.Report.Summary
		}
		if j.Kind == "calendar" {
			due := dueSlot(p.specs, env.Now, env.LastWake, env.Grace)
			if !due.IsZero() && due.After(p.added) {
				in.Due = due
				j.DueSlot = fmtTime(due)
			}
			j.NextRun = fmtTime(nextSlot(p.specs, env.Now))
		}
		if j.Kind == "interval" {
			if lr := parseAnyTime(j.LastRun, env.Loc); !lr.IsZero() {
				j.NextRun = fmtTime(lr.Add(time.Duration(j.IntervalSec) * time.Second))
			}
		}
		for _, s := range j.Snapshots {
			if s.Stale {
				if s.Error != "" {
					in.StaleSnaps = append(in.StaleSnaps, fmt.Sprintf("%s: no snapshot found (%s)", s.Name, s.Error))
				} else {
					in.StaleSnaps = append(in.StaleSnaps, fmt.Sprintf("%s: newest snapshot is %.1f days old", s.Name, s.AgeHours/24))
				}
			}
		}
		for _, n := range j.Notes {
			if n.Level == "warn" {
				in.WarnNotes++
			}
		}
		j.Status, j.Reasons = deriveStatus(in)
		if j.Reasons == nil {
			j.Reasons = []string{}
		}
		j.Red = redStatuses[j.Status]
		rep.Summary.Total++
		rep.Summary.ByStatus[j.Status]++
		if j.Red {
			rep.Summary.Red++
		} else if j.Status == "warning" {
			rep.Summary.Warn++
		}
		for _, n := range j.Notes {
			if n.Level != "info" {
				rep.Findings = append(rep.Findings, Note{Level: n.Level, Label: j.Label, Text: n.Text})
			}
		}
		if (j.Kind == "calendar" || j.Kind == "interval") && j.Report == nil {
			rep.WantsStatusFile = append(rep.WantsStatusFile, j.Label)
		}
		rep.Jobs = append(rep.Jobs, *j)
	}
	rep.Summary.OK = rep.Summary.Red == 0

	sort.SliceStable(rep.Jobs, func(a, b int) bool { return jobSortKey(rep.Jobs[a]) < jobSortKey(rep.Jobs[b]) })
	sort.Slice(rep.ThirdParty, func(a, b int) bool { return rep.ThirdParty[a].Label < rep.ThirdParty[b].Label })
	return rep
}

func jobSortKey(j Job) string {
	order := map[string]string{"calendar": "0", "interval": "1", "service": "2"}
	k, ok := order[j.Kind]
	if !ok {
		k = "3"
	}
	first := "99:99"
	if len(j.Slots) > 0 {
		first = j.Slots[0]
	}
	return k + first + j.Label
}

func addNote(j *Job, level, text string) {
	for _, n := range j.Notes {
		if n.Text == text {
			return
		}
	}
	j.Notes = append(j.Notes, Note{Level: level, Text: text})
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func round1(f float64) float64 { return float64(int(f*10+0.5)) / 10 }
