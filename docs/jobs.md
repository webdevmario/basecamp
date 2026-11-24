# Jobs

`basecamp jobs` checks every scheduled job on this Mac (user LaunchAgents in
`~/Library/LaunchAgents`), says whether its last run worked, and flags backups that are stale or
never reach the offsite image. The dashboard shows it in the **Jobs** tab (`/#jobs`).

## What runs

| Piece | What it does |
|---|---|
| `basecamp jobs` | Writes `~/.basecamp/jobs.json`. Sends one macOS notification when a job turns red. |
| launchd `com.basecamp-jobs` | Runs `basecamp jobs -launchd` at load and every 15 minutes. Installed by `scripts/install-service.sh`. |
| `GET /jobs.json` | The report, for the Jobs tab (`basecamp serve`). |
| `GET /api/jobs/status` | `{ ok, red, warn, total, redJobs, generatedAt, message }`, CORS open, for Springboard's status dot. A missing report, or one over an hour old, is `ok: false`. |

`jobs.json`, `jobs-state.json` and `jobs-config.json` describe this Mac. They live in
`~/.basecamp`, never in the repo.

## How a job's status is decided

For each LaunchAgent (third-party ones such as `com.google.*`, `homebrew.mxcl.*` or anything
inside an `.app` are listed apart and never turn red):

- **Plist**: label, schedule (`StartCalendarInterval`, `StartInterval`, `KeepAlive`), command,
  `StandardOutPath` / `StandardErrorPath`.
- **launchd** (`launchctl print gui/<uid>/<label>`): loaded or not, running pid, runs since load,
  last exit code.
- **Logs**: last modified time, last lines, and error markers in the lines from the last run
  (`ERROR`, `FAILED`, `Error:`, a Python traceback, lines starting `!!`; `WARNING` counts as a
  warning). Lines belong to the last run when they are timestamped at or after the run's slot.
- **Status file**, when the job writes one (below).

| Status | Meaning | Red |
|---|---|---|
| `failed` | Non-zero exit, status file `ok: false`, or error lines in the last run | yes |
| `missed` | No run since its latest slot (30 minutes of grace; a slot missed during sleep gets 30 minutes after wake, since launchd runs it then) | yes |
| `stale` | A backup's newest snapshot is older than 2 days (the offsite job's own rule), or there is none | yes |
| `unloaded` | The plist is there but launchd hasn't loaded it, so it never runs | yes |
| `down` | A KeepAlive service isn't running | yes |
| `warning` | Warning lines, logs in `/tmp` (lost on reboot), or backups the offsite image doesn't include | no |
| `pending` | Added after its latest slot, so it hasn't had a chance to run | no |
| `ok` / `running` / `idle` | Fine | no |

The notification fires once per failure: a new failed run, a newly missed slot, or a new status.
The same failure seen again 15 minutes later stays quiet; once the job recovers, the next failure
notifies again.

## Job status files (optional)

A job can say what it did by writing `~/Library/Application Support/jobs/<label>.json` when it
finishes:

```json
{
  "ok": true,
  "started": "2026-11-24T03:30:00-06:00",
  "finished": "2026-11-24T03:30:06-06:00",
  "summary": "stacks 1.7 MB, rankd 8 KB"
}
```

`ok: false` makes the job red with the summary as the reason; `summary` is the result line in the
Jobs tab; `started` / `finished` give the run's exact times. A shell job can write it with:

```bash
mkdir -p "$HOME/Library/Application Support/jobs"
printf '{"ok":%s,"started":"%s","finished":"%s","summary":"%s"}\n' \
  "$ok" "$started" "$(date +%Y-%m-%dT%H:%M:%S%z)" "$summary" \
  > "$HOME/Library/Application Support/jobs/com.example-backup.json"
```

Without one, basecamp falls back to the exit code and the log. The Jobs tab lists the scheduled
jobs that don't write one yet.

## Per-job settings: `~/.basecamp/jobs-config.json`

Optional. Tells basecamp where each backup keeps its snapshots and which job builds the offsite
image. See [`jobs-config.example.json`](jobs-config.example.json).

- `jobs.<label>.snapshots`: `dir` and `pattern` (a regex on file names) for each kind of snapshot
  the job writes. The newest is checked for age. `name` defaults to the pattern's literal prefix.
- `jobs.<label>.log`: a glob for a log the plist doesn't name (say a dated
  `nightly-YYYY-MM-DD.log`); its newest match is the main log.
- `jobs.<label>.ignore`: leave the job out.
- `offsite`: the job that bundles snapshots offsite, where its images go, and its `script`.
  A snapshot counts as included when its `name` appears in the script (override with
  `"offsite": true|false` on the snapshot), and as reaching the offsite image when the newest image
  is newer than the snapshot.
- `staleAfterHours` (default 48), `thirdParty` (extra label prefixes to list apart).

The 15-minute check runs from launchd, which can't read privacy-protected folders (`~/Documents`,
iCloud Drive, ...) without Full Disk Access. With `-launchd` it doesn't try: it finds the newest
snapshot from the paths the job's log names instead. Run `basecamp jobs` by hand to read the
folders themselves.
