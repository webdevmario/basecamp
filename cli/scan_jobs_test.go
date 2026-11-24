package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

var chicago = mustLoc("America/Chicago")

func mustLoc(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return l
}

func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, chicago)
	if err != nil {
		panic(err)
	}
	return t
}

func intp(n int) *int { return &n }

// ---------- plist ----------

func TestParsePlistKinds(t *testing.T) {
	cases := []struct {
		name, json, kind, schedule, command string
	}{
		{"daily backup",
			`{"Label":"com.x-backup","ProgramArguments":["/bin/bash","-c","source ~/.nvm/nvm.sh && node backup.mjs"],"StartCalendarInterval":{"Hour":3,"Minute":10}}`,
			"calendar", "03:10 daily", "source ~/.nvm/nvm.sh && node backup.mjs"},
		{"several slots",
			`{"Label":"a","ProgramArguments":["/usr/bin/true"],"StartCalendarInterval":[{"Hour":9,"Minute":0,"Weekday":1},{"Minute":5}]}`,
			"calendar", "09:00 Mons; hourly at :05", "/usr/bin/true"},
		{"interval", `{"Label":"b","Program":"/usr/local/bin/thing","StartInterval":900}`, "interval", "", "/usr/local/bin/thing"},
		{"keepalive", `{"Label":"c","ProgramArguments":["/bin/zsh","/path with space/run.sh"],"KeepAlive":true,"RunAtLoad":true}`,
			"service", "", `/bin/zsh "/path with space/run.sh"`},
		{"keepalive dict", `{"Label":"d","ProgramArguments":["x"],"KeepAlive":{"SuccessfulExit":false}}`, "service", "", "x"},
		{"keepalive false", `{"Label":"e","ProgramArguments":["x"],"KeepAlive":false,"RunAtLoad":true}`, "at-load", "", "x"},
		{"watch", `{"Label":"f","ProgramArguments":["x"],"WatchPaths":["/tmp/a"]}`, "watch", "", "x"},
		{"on demand", `{"Label":"g","ProgramArguments":["x"]}`, "on-demand", "", "x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := parsePlistJSON([]byte(c.json))
			if err != nil {
				t.Fatal(err)
			}
			if got := p.kind(); got != c.kind {
				t.Errorf("kind = %q, want %q", got, c.kind)
			}
			if got := p.command(); got != c.command {
				t.Errorf("command = %q, want %q", got, c.command)
			}
			if c.schedule != "" {
				specs, err := p.calendar()
				if err != nil {
					t.Fatal(err)
				}
				if got := describeCalendar(specs); got != c.schedule {
					t.Errorf("schedule = %q, want %q", got, c.schedule)
				}
			}
		})
	}
}

func TestReadPlistXML(t *testing.T) {
	if _, err := exec.LookPath("plutil"); err != nil {
		t.Skip("plutil not available")
	}
	dir := t.TempDir()
	good := filepath.Join(dir, "good.plist")
	os.WriteFile(good, []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.example.nightly</string>
  <key>ProgramArguments</key><array><string>/bin/zsh</string><string>/opt/nightly.sh</string></array>
  <key>StartCalendarInterval</key><dict><key>Hour</key><integer>5</integer><key>Minute</key><integer>0</integer></dict>
  <key>StandardOutPath</key><string>/tmp/nightly.log</string>
</dict></plist>`), 0o644)
	data, err := readPlistJSON(good)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := parsePlistJSON(data)
	if p.Label != "com.example.nightly" || p.kind() != "calendar" || p.StandardOutPath != "/tmp/nightly.log" {
		t.Errorf("got %+v", p)
	}

	bad := filepath.Join(dir, "bad.plist")
	os.WriteFile(bad, []byte(`<?xml version="1.0"?><plist version="1.0"><dict><key>Label</key><string>a && b</string></dict></plist>`), 0o644)
	if _, err := readPlistJSON(bad); err == nil || !strings.Contains(err.Error(), "not a valid plist") {
		t.Errorf("want a parse error, got %v", err)
	}
}

func TestThirdParty(t *testing.T) {
	p := plistAgent{ProgramArguments: []string{"/Applications/VPN by Google One.app/Contents/MacOS/VPN by Google One"}}
	if !isThirdParty("VPN by Google One", p, defaultThirdParty) {
		t.Error("app bundle agent should be third-party")
	}
	if !isThirdParty("com.google.keystone.agent", plistAgent{}, defaultThirdParty) {
		t.Error("com.google.* should be third-party")
	}
	mine := plistAgent{ProgramArguments: []string{"/bin/bash", "/Users/me/Library/Application Support/stacks-backup/backup-db.sh"}}
	if isThirdParty("com.stacks-backup", mine, defaultThirdParty) {
		t.Error("own script should not be third-party")
	}
}

// ---------- launchctl ----------

const launchctlScheduled = `gui/501/com.stacks-backup = {
	active count = 0
	path = /Users/me/Library/LaunchAgents/com.stacks-backup.plist
	type = LaunchAgent
	state = not running

	program = /bin/bash
	stdout path = /tmp/stacks-backup.log
	runs = 29
	last exit code = 0

	event triggers = {
		com.stacks-backup.268435483 => {
			keepalive = 0
			service = com.stacks-backup
			stream = com.apple.launchd.calendarinterval.system
			descriptor = {
				"Minute" => 30
				"Hour" => 3
			}
		}
	}

	event channels = {
		state = active
	}
	job state = exited
	properties = inferred program | managed LWCR | has LWCR
}
`

func TestParseLaunchctlPrint(t *testing.T) {
	info := parseLaunchctlPrint(launchctlScheduled)
	if !info.Loaded || info.State != "not running" || info.Runs != 29 || info.LastExit == nil || *info.LastExit != 0 || info.PID != 0 {
		t.Errorf("scheduled: %+v", info)
	}

	svc := parseLaunchctlPrint("gui/501/com.borrowd = {\n\tstate = running\n\truns = 1\n\tpid = 85084\n\tlast exit code = (never exited)\n\tproperties = keepalive | runatload | inferred program\n}\n")
	if svc.PID != 85084 || svc.LastExit != nil || !strings.Contains(svc.Properties, "keepalive") {
		t.Errorf("service: %+v", svc)
	}

	failed := parseLaunchctlPrint("x = {\n\tlast exit code = 78: Function not implemented\n}\n")
	if failed.LastExit == nil || *failed.LastExit != 78 {
		t.Errorf("failed: %+v", failed)
	}

	if parseLaunchctlPrint("").Loaded {
		t.Error("empty output means not loaded")
	}
}

// ---------- schedule ----------

func TestSlots(t *testing.T) {
	daily := []CalSpec{{Hour: intp(3), Minute: intp(30)}}
	now := at("2026-11-24 11:00")

	if got := prevSlot(daily, now); !got.Equal(at("2026-11-24 03:30")) {
		t.Errorf("prevSlot = %v", got)
	}
	if got := nextSlot(daily, now); !got.Equal(at("2026-11-25 03:30")) {
		t.Errorf("nextSlot = %v", got)
	}
	// 03:40, still inside the grace window: today's slot isn't due yet, yesterday's is.
	if got := dueSlot(daily, at("2026-11-24 03:40"), time.Time{}, 30*time.Minute); !got.Equal(at("2026-11-23 03:30")) {
		t.Errorf("dueSlot in grace = %v", got)
	}
	if got := dueSlot(daily, at("2026-11-24 04:01"), time.Time{}, 30*time.Minute); !got.Equal(at("2026-11-24 03:30")) {
		t.Errorf("dueSlot after grace = %v", got)
	}
	// Asleep through 03:30, woke at 07:50: launchd runs it at wake, so give it until 08:20.
	wake := at("2026-11-24 07:50")
	if got := dueSlot(daily, at("2026-11-24 08:00"), wake, 30*time.Minute); !got.Equal(at("2026-11-23 03:30")) {
		t.Errorf("dueSlot right after wake = %v", got)
	}
	if got := dueSlot(daily, at("2026-11-24 08:30"), wake, 30*time.Minute); !got.Equal(at("2026-11-24 03:30")) {
		t.Errorf("dueSlot after wake grace = %v", got)
	}

	monday := []CalSpec{{Hour: intp(9), Minute: intp(0), Weekday: intp(1)}}
	if got := prevSlot(monday, now); !got.Equal(at("2026-11-23 09:00")) { // the 24th is a Tuesday
		t.Errorf("weekly prevSlot = %v", got)
	}
	sunday7 := []CalSpec{{Hour: intp(9), Minute: intp(0), Weekday: intp(7)}}
	if got := prevSlot(sunday7, now); !got.Equal(at("2026-11-22 09:00")) {
		t.Errorf("Weekday 7 is Sunday, got %v", got)
	}
	if got := dailySlots([]CalSpec{{Hour: intp(4), Minute: intp(0)}, {Hour: intp(3), Minute: intp(5)}, {Minute: intp(5)}}); strings.Join(got, ",") != "03:05,04:00" {
		t.Errorf("dailySlots = %v", got)
	}
	if got := describeInterval(900); got != "every 15 min" {
		t.Errorf("describeInterval = %q", got)
	}
}

// ---------- logs ----------

func TestLineTime(t *testing.T) {
	cases := map[string]string{
		"2026-11-24T09:10:06.214Z Backup written: x":   "2026-11-24 03:10",
		"2026-11-24 03:30:06 wrote /x/stacks.dump":      "2026-11-24 03:30",
		"=== 2026-11-24 05:00:05 ===":                   "2026-11-24 05:00",
		"[2026-11-24 03:30] start":                      "2026-11-24 03:30",
		"2026-11-24T03:30:00-06:00 offset with colon":   "2026-11-24 03:30",
		"2026-11-24T09:30:00+0000 offset without colon": "2026-11-24 03:30",
	}
	for line, want := range cases {
		got, ok := lineTime(line, chicago)
		if !ok || !got.Truncate(time.Minute).Equal(at(want)) {
			t.Errorf("%q -> %v %v, want %s", line, got, ok, want)
		}
	}
	if _, ok := lineTime("  ready: a book, pick up by 2026-11-28 17:00", chicago); ok {
		t.Error("a date mid-line is not a timestamp")
	}
}

func TestLastRunLinesAndMarkers(t *testing.T) {
	// borrowd style: one header with a date, unstamped lines after, failures start with !!
	borrowd := []string{
		"=== 2026-11-23 05:00:04 ===",
		"!! enrich failed (1)",
		"=== done 06:12:00 ===",
		"=== 2026-11-24 05:00:05 ===",
		"card A: 10 out",
		"!! libby check failed (2)",
		"WARNING: slow answer from the library",
		"=== done 06:36:49 ===",
	}
	lines, first, _ := lastRunLines(borrowd, at("2026-11-24 04:58"), at("2026-11-24 06:36"), chicago)
	errs, warns := classifyLines(lines)
	if len(errs) != 1 || !strings.Contains(errs[0], "libby") {
		t.Errorf("errors = %v (yesterday's enrich failure must not count)", errs)
	}
	if len(warns) != 1 {
		t.Errorf("warnings = %v", warns)
	}
	if !first.Equal(time.Date(2026, 11, 24, 5, 0, 5, 0, chicago)) {
		t.Errorf("run start = %v", first)
	}

	// No timestamps at all: the whole tail counts only if the file changed during this run.
	plain := []string{"Error: connect ECONNREFUSED 127.0.0.1:5432"}
	lines, _, _ = lastRunLines(plain, at("2026-11-24 03:28"), at("2026-11-24 03:30"), chicago)
	if errs, _ := classifyLines(lines); len(errs) != 1 {
		t.Errorf("fresh unstamped error should count, got %v", errs)
	}
	lines, _, _ = lastRunLines(plain, at("2026-11-24 03:28"), at("2026-11-20 03:30"), chicago)
	if len(lines) != 0 {
		t.Errorf("old unstamped log should not count, got %v", lines)
	}

	// Offsite style warning, stacks style FAILED
	_, w := classifyLines([]string{"2026-11-24T10:00:01.000Z WARNING: vault: newest snapshot is 3.1 days old"})
	e, _ := classifyLines([]string{"2026-11-24 03:30:06 FAILED to back up rankd"})
	if len(w) != 1 || len(e) != 1 {
		t.Errorf("warnings %v errors %v", w, e)
	}
}

func TestSnapshotsFromLog(t *testing.T) {
	re := regexp.MustCompile(`^stacks-\d{8}-\d{6}-nightly\.dump$`)
	lines := []string{
		"2026-11-23 03:30:05 wrote /Users/me/Backups/postgres/stacks-20261123-033005-nightly.dump (1.7M)",
		"2026-11-24 03:30:06 wrote /Users/me/Backups/postgres/stacks-20261124-033005-nightly.dump (1.7M)",
		"/Users/me/Backups/postgres/stacks-20261109-033005-nightly.dump", // pruned, no timestamp: older by its name
	}
	hit := newestInLog(lines, re, chicago)
	if hit.file != "stacks-20261124-033005-nightly.dump" || hit.src != "log" {
		t.Errorf("got %+v", hit)
	}
	offsite := newestInLog([]string{
		"2026-11-24T10:00:39.508Z Wrote /x/Offsite/offsite-20261124.dmg (300.0 MB)",
		"2026-11-24T10:00:39.572Z Pruned: offsite-20261117.dmg",
	}, regexp.MustCompile(`^offsite-\d{8}\.dmg$`), chicago)
	if offsite.file != "offsite-20261124.dmg" || offsite.at.In(chicago).Hour() != 4 {
		t.Errorf("a pruned older image must not win: %+v", offsite)
	}
	if got := snapshotName(SnapshotConfig{Pattern: `^diecast_scout-\d{8}-\d{6}-nightly\.dump$`}); got != "diecast_scout" {
		t.Errorf("snapshotName = %q", got)
	}
	if got := snapshotName(SnapshotConfig{Pattern: `^tft-\d{8}\.db$`}); got != "tft" {
		t.Errorf("snapshotName = %q", got)
	}
}

// ---------- status ----------

func TestDeriveStatus(t *testing.T) {
	now := at("2026-11-24 11:00")
	due := at("2026-11-24 03:30")
	base := func() statusInput {
		return statusInput{Kind: "calendar", Loaded: true, Runs: 3, LastExit: intp(0),
			LastRun: at("2026-11-24 03:31"), Due: due, Now: now, Grace: 30 * time.Minute}
	}
	f := false
	cases := []struct {
		name   string
		mod    func(*statusInput)
		status string
	}{
		{"ok", func(*statusInput) {}, "ok"},
		{"unloaded", func(in *statusInput) { in.Loaded = false }, "unloaded"},
		{"exit code", func(in *statusInput) { in.LastExit = intp(1) }, "failed"},
		{"status file says failed", func(in *statusInput) { in.ReportOK = &f; in.ReportSummary = "disk full" }, "failed"},
		{"error lines", func(in *statusInput) { in.Errors = []string{"!! sync failed"} }, "failed"},
		{"missed", func(in *statusInput) { in.LastRun = at("2026-11-23 03:31") }, "missed"},
		{"never ran but due", func(in *statusInput) { in.LastRun = time.Time{}; in.Runs = 0 }, "missed"},
		{"ran, no evidence when", func(in *statusInput) { in.LastRun = time.Time{} }, "unknown"},
		{"added after last slot", func(in *statusInput) { in.Due = time.Time{}; in.LastRun = time.Time{}; in.Runs = 0 }, "pending"},
		{"stale backup", func(in *statusInput) { in.StaleSnaps = []string{"x: newest snapshot is 3.0 days old"} }, "stale"},
		{"warning lines", func(in *statusInput) { in.Warnings = []string{"WARNING: x"} }, "warning"},
		{"warn note", func(in *statusInput) { in.WarnNotes = 1 }, "warning"},
		{"running now", func(in *statusInput) { in.PID = 42; in.LastRun = at("2026-11-23 03:31") }, "running"},
		{"failed beats missed", func(in *statusInput) { in.LastExit = intp(2); in.LastRun = at("2026-11-22 03:31") }, "failed"},
		{"interval ok", func(in *statusInput) { in.Kind = "interval"; in.IntervalSec = 900; in.LastRun = at("2026-11-24 10:50") }, "ok"},
		{"interval missed", func(in *statusInput) { in.Kind = "interval"; in.IntervalSec = 900; in.LastRun = at("2026-11-24 09:00") }, "missed"},
		{"service up", func(in *statusInput) { in.Kind = "service"; in.PID = 7; in.LastExit = nil }, "running"},
		{"service down", func(in *statusInput) { in.Kind = "service"; in.LastExit = nil }, "down"},
		{"service crashed", func(in *statusInput) { in.Kind = "service"; in.LastExit = intp(1) }, "failed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := base()
			c.mod(&in)
			got, reasons := deriveStatus(in)
			if got != c.status {
				t.Errorf("status = %q (%v), want %q", got, reasons, c.status)
			}
			if redStatuses[got] && len(reasons) == 0 {
				t.Errorf("red status %q needs a reason", got)
			}
		})
	}
}

// ---------- notifications and status endpoint ----------

func TestNotifyOncePerFailure(t *testing.T) {
	state := jobsState{Runs: map[string]runCount{}, Notified: map[string]string{}}
	red := Job{Label: "com.x-backup", Status: "missed", Red: true, LastRun: "2026-11-23T03:31:00-06:00", DueSlot: "2026-11-24T03:30:00-06:00", Reasons: []string{"no run"}}
	rep := JobsReport{Jobs: []Job{red, {Label: "ok", Status: "ok"}}}

	if n := pendingNotification(&state, rep); n == nil || !strings.Contains(n.title, "com.x-backup") {
		t.Fatalf("first failure should notify, got %+v", n)
	}
	if n := pendingNotification(&state, rep); n != nil {
		t.Errorf("same failure 15 minutes later should stay quiet, got %+v", n)
	}
	next := red
	next.DueSlot = "2026-11-25T03:30:00-06:00" // missed again the next night
	if n := pendingNotification(&state, JobsReport{Jobs: []Job{next}}); n == nil {
		t.Error("a new missed slot should notify")
	}
	pendingNotification(&state, JobsReport{Jobs: []Job{{Label: "com.x-backup", Status: "ok"}}})
	if _, ok := state.Notified["com.x-backup"]; ok {
		t.Error("recovery should clear the notified mark")
	}
	two := JobsReport{Jobs: []Job{red, {Label: "b", Status: "failed", Red: true}}}
	if n := pendingNotification(&state, two); n == nil || !strings.HasPrefix(n.title, "2 jobs") {
		t.Errorf("two new failures should be one notification, got %+v", n)
	}
}

func TestJobsStatus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.json")
	now := time.Now()
	if st := jobsStatus(path, now); st.OK {
		t.Error("missing report must not be green")
	}
	write := func(gen time.Time, red int) {
		rep := JobsReport{GeneratedAt: fmtTime(gen), Summary: JobsSummary{Total: 5, Red: red}}
		if red > 0 {
			rep.Jobs = []Job{{Label: "com.a", Red: true}}
		}
		b, _ := json.Marshal(rep)
		os.WriteFile(path, b, 0o644)
	}
	write(now.Add(-5*time.Minute), 0)
	if st := jobsStatus(path, now); !st.OK {
		t.Errorf("fresh clean report should be green: %+v", st)
	}
	write(now.Add(-5*time.Minute), 1)
	if st := jobsStatus(path, now); st.OK || st.RedJobs[0] != "com.a" {
		t.Errorf("red job should be red: %+v", st)
	}
	write(now.Add(-2*time.Hour), 0)
	if st := jobsStatus(path, now); st.OK {
		t.Errorf("stale report should be red: %+v", st)
	}
}

// ---------- the whole report against a fake Mac ----------

func TestBuildJobsReport(t *testing.T) {
	root := t.TempDir()
	agents := filepath.Join(root, "LaunchAgents")
	logs := filepath.Join(root, "Logs")
	snaps := filepath.Join(root, "backups")
	images := filepath.Join(root, "Offsite")
	status := filepath.Join(root, "jobs")
	for _, d := range []string{agents, logs, snaps, images, status} {
		os.MkdirAll(d, 0o755)
	}
	now := at("2026-11-24 11:00")
	old := at("2026-11-01 12:00")

	writeAgent := func(label, body string) {
		p := filepath.Join(agents, label+".plist")
		os.WriteFile(p, []byte(body), 0o644)
		os.Chtimes(p, old, old)
	}
	writeFile := func(path, text string, mod time.Time) {
		os.WriteFile(path, []byte(text), 0o644)
		os.Chtimes(path, mod, mod)
	}

	// health: ran at 03:10, snapshot fresh, in the 04:00 image
	writeAgent("com.health-backup", `{"Label":"com.health-backup","ProgramArguments":["node","backup.mjs"],"StartCalendarInterval":{"Hour":3,"Minute":10},"StandardOutPath":"`+logs+`/health.log"}`)
	writeFile(logs+"/health.log", "2026-11-24T09:10:06.214Z Backup written: "+snaps+"/health-20261124.db (540 KB)\n", at("2026-11-24 03:10"))
	writeFile(snaps+"/health-20261124.db", "x", at("2026-11-24 03:10"))
	// diecast: ran, but the offsite script doesn't know it
	writeAgent("com.diecast-backup", `{"Label":"com.diecast-backup","ProgramArguments":["backup.sh"],"StartCalendarInterval":{"Hour":3,"Minute":40},"StandardOutPath":"`+logs+`/diecast.log"}`)
	writeFile(logs+"/diecast.log", "2026-11-24 03:40:00 wrote "+snaps+"/diecast-20261124.dump\n", at("2026-11-24 03:40"))
	writeFile(snaps+"/diecast-20261124.dump", "x", at("2026-11-24 03:40"))
	// finance: last ran three days ago (missed), and its snapshot is stale
	writeAgent("com.finance-backup", `{"Label":"com.finance-backup","ProgramArguments":["x"],"StartCalendarInterval":{"Hour":3,"Minute":0},"StandardOutPath":"`+logs+`/finance.log"}`)
	writeFile(logs+"/finance.log", "2026-11-21T09:00:05.000Z Backup written: finance-20261121.db\n", at("2026-11-21 03:00"))
	writeFile(snaps+"/finance-20261121.db", "x", at("2026-11-21 03:00"))
	// offsite: wrote today's image
	writeAgent("com.offsite-backup", `{"Label":"com.offsite-backup","ProgramArguments":["node","offsite.mjs"],"StartCalendarInterval":{"Hour":4,"Minute":0},"StandardOutPath":"`+logs+`/offsite.log","StandardErrorPath":"`+logs+`/offsite-error.log"}`)
	writeFile(logs+"/offsite.log", "2026-11-24T10:00:39.508Z Wrote "+images+"/offsite-20261124.dmg (300.0 MB)\n", at("2026-11-24 04:00"))
	writeFile(images+"/offsite-20261124.dmg", "x", at("2026-11-24 04:00"))
	offsiteScript := filepath.Join(root, "offsite.mjs")
	writeFile(offsiteScript, `newest(dir, /^health-\d{8}\.db$/, "health"); newest(dir, /^finance-\d{8}\.db$/, "finance")`, old)
	// nightly: status file says it failed
	writeAgent("local.nightly", `{"Label":"local.nightly","ProgramArguments":["/bin/zsh","nightly.sh"],"StartCalendarInterval":{"Hour":5,"Minute":0}}`)
	writeFile(status+"/local.nightly.json", `{"ok":false,"started":"2026-11-24T05:00:00-06:00","finished":"2026-11-24T05:20:00-06:00","summary":"catalog sync failed"}`, at("2026-11-24 05:20"))
	// a service and a third-party agent
	writeAgent("com.web", `{"Label":"com.web","ProgramArguments":["node","server.js"],"KeepAlive":true,"StandardOutPath":"/tmp/web.log"}`)
	writeAgent("com.google.keystone.agent", `{"Label":"com.google.keystone.agent","ProgramArguments":["/x"],"StartInterval":3600}`)

	launchctl := map[string]string{
		"com.health-backup":  "x = {\n\tstate = not running\n\truns = 5\n\tlast exit code = 0\n}\n",
		"com.diecast-backup": "x = {\n\tstate = not running\n\truns = 5\n\tlast exit code = 0\n}\n",
		"com.finance-backup": "x = {\n\tstate = not running\n\truns = 2\n\tlast exit code = 0\n}\n",
		"com.offsite-backup": "x = {\n\tstate = not running\n\truns = 5\n\tlast exit code = 0\n}\n",
		"local.nightly":      "x = {\n\tstate = not running\n\truns = 1\n\tlast exit code = 0\n}\n",
		"com.web":            "x = {\n\tstate = running\n\tpid = 99\n\truns = 1\n\tproperties = keepalive\n}\n",
	}
	cfg := JobsConfig{
		StaleAfterHours: 48,
		ThirdParty:      defaultThirdParty,
		Offsite:         &OffsiteConfig{Label: "com.offsite-backup", Dir: images, Pattern: `^offsite-\d{8}\.dmg$`, Script: offsiteScript},
		Jobs: map[string]JobConfig{
			"com.health-backup":  {Snapshots: []SnapshotConfig{{Dir: snaps, Pattern: `^health-\d{8}\.db$`}}},
			"com.diecast-backup": {Snapshots: []SnapshotConfig{{Dir: snaps, Pattern: `^diecast-\d{8}\.dump$`}}},
			"com.finance-backup": {Snapshots: []SnapshotConfig{{Dir: snaps, Pattern: `^finance-\d{8}\.db$`}}},
		},
	}
	src, _ := os.ReadFile(offsiteScript)
	env := jobsEnv{
		Now: now, Loc: chicago, AgentsDir: agents, StatusDir: status, Grace: 30 * time.Minute, Config: cfg,
		Launchctl:  func(l string) string { return launchctl[l] },
		ReadPlist:  os.ReadFile, // the fixtures are already JSON
		OffsiteSrc: string(src),
	}
	rep := buildJobsReport(env)

	byLabel := map[string]Job{}
	for _, j := range rep.Jobs {
		byLabel[j.Label] = j
	}
	expect := map[string]string{
		"com.health-backup":  "ok",
		"com.diecast-backup": "warning",
		"com.finance-backup": "missed",
		"com.offsite-backup": "ok",
		"local.nightly":      "failed",
		"com.web":            "running",
	}
	for label, want := range expect {
		j, ok := byLabel[label]
		if !ok {
			t.Errorf("%s missing from report", label)
			continue
		}
		if j.Status != want {
			t.Errorf("%s: status %q (%v), want %q", label, j.Status, j.Reasons, want)
		}
	}
	if len(rep.ThirdParty) != 1 || rep.ThirdParty[0].Label != "com.google.keystone.agent" {
		t.Errorf("third party = %+v", rep.ThirdParty)
	}
	if _, ok := byLabel["com.google.keystone.agent"]; ok {
		t.Error("third-party agent listed with own jobs")
	}

	h := byLabel["com.health-backup"].Snapshots[0]
	if h.Offsite != "in-image" || h.OffsiteImage != "offsite-20261124.dmg" || h.Stale {
		t.Errorf("health snapshot = %+v", h)
	}
	d := byLabel["com.diecast-backup"].Snapshots[0]
	if d.Offsite != "not-included" {
		t.Errorf("diecast snapshot = %+v", d)
	}
	fin := byLabel["com.finance-backup"]
	if !fin.Snapshots[0].Stale || fin.DueSlot == "" {
		t.Errorf("finance = %+v", fin)
	}
	if n := byLabel["local.nightly"]; n.LastRunSource != "status-file" || !strings.Contains(strings.Join(n.Reasons, " "), "catalog sync failed") {
		t.Errorf("nightly = %+v", n)
	}
	if web := byLabel["com.web"]; len(web.Notes) == 0 || !strings.Contains(web.Notes[0].Text, "/tmp") {
		t.Errorf("service logging to /tmp should get a note: %+v", web.Notes)
	}
	if rep.Summary.OK || rep.Summary.Red != 2 {
		t.Errorf("summary = %+v", rep.Summary)
	}
	if rep.Offsite == nil || rep.Offsite.Image != "offsite-20261124.dmg" {
		t.Errorf("offsite = %+v", rep.Offsite)
	}
	// Jobs come out in night order: 03:00, 03:10, 03:40, 04:00, 05:00, then services
	var order []string
	for _, j := range rep.Jobs {
		order = append(order, j.Label)
	}
	if strings.Join(order, ",") != "com.finance-backup,com.health-backup,com.diecast-backup,com.offsite-backup,local.nightly,com.web" {
		t.Errorf("order = %v", order)
	}
}
