# basecamp: notes for Claude sessions

Mac inventory and health dashboard for Mario's Mac mini: a Go CLI scans the machine (dotfiles,
VS Code, runtimes, global packages, fonts, macOS defaults, SSH keys, services) into JSON, and a
React + Vite dashboard shows it with health scores, per-item notes and a setup playbook. It also
checks scheduled jobs and backups (Jobs tab) and keeps configs in a private repo (Configs tab).

**The GitHub repo is public.** Never commit a scan, the compiled binary, or anything read from this
Mac (paths, hostnames, keys, dotfile contents).

## Layout

- `cli/`: Go, no dependencies. `main.go` (commands), `scan_*.go` (one scanner per category, each
  returns a `Category`), `types.go` (JSON shape), `helpers.go` (`run`, `runLines`, file helpers),
  `serve.go` (HTTP server). Jobs: `scan_jobs.go` (plists, launchctl, logs, snapshots, status
  rules), `jobs_cmd.go` (`basecamp jobs`, notify-once state), `jobs_http.go` (routes).
- `web/`: React 19 + Vite. `App.jsx` (everything on screen), `health.js` (scores and observations),
  `playbook.js` (bash setup script), `notes.js` (per-item notes, saved through `/api/notes`),
  `VersionDetail.jsx` (nvm/pyenv versions compared), `NoteModal.jsx`, `JobsView.jsx` (Jobs tab).
- `docs/jobs.md`: how job status is decided, the status file convention, `jobs-config.json`.
- `scripts/install-service.sh`: builds and installs the launchd jobs.

## Running

- Live: http://shady-mac-mini.taila545c3.ts.net:3050 (Springboard tile "basecamp").
- launchd `com.basecamp`: `basecamp serve` on :3050, KeepAlive. Serves `~/.basecamp/app/web`,
  `GET /scan.json` from `~/.basecamp/scan.json`, `POST /api/scan` (rescan now), `GET /api/health`.
- launchd `com.basecamp-scan`: `basecamp scan -o ~/.basecamp/scan.json` at load and every hour.
- launchd `com.basecamp-jobs`: `basecamp jobs -launchd` at load and every 15 min into
  `~/.basecamp/jobs.json`; one macOS notification per new failure (state in `jobs-state.json`).
  Backup folders and the offsite job are configured in `~/.basecamp/jobs-config.json` (local only).
  `GET /jobs.json` feeds the Jobs tab (`/#jobs`); `GET /api/jobs/status` feeds Springboard's Jobs tile.
- Logs: `~/.basecamp/logs/<label>.log`.
- Tailnet only: `tailnetOnly()` in `serve.go` answers 100.64.0.0/10, fd7a:115c:a1e0::/48 and loopback;
  anything else (home LAN) gets 403. Keep it: config diffs will be served here. Never use Funnel.
- New API areas plug in as `registerXRoutes(mux, ...)` in their own `cli/*_http.go`, one line in `runServe`.
- Deploy: `scripts/install-service.sh` from any checkout. It builds the CLI and `web/dist`, copies
  them to `~/.basecamp/app` (so removing a worktree never breaks the service) and reloads both jobs.
  `--remove` uninstalls and keeps `~/.basecamp`.
- Dev: `cd cli && go build -o basecamp . && ./basecamp scan --pretty`; `cd web && npm run dev`.

## Gotchas

- launchd jobs get a bare environment. Both plists source `~/.nvm/nvm.sh` and set PATH (Homebrew,
  `/usr/local/bin` for `code`) and `SHELL`, or the scan misses nvm Node, brew and VS Code.
- `run()` in `helpers.go` kills any command after 60s (`commandTimeout`), so a hung tool (an
  `osascript` permission prompt) leaves that category short instead of hanging the scan.
- Notes live in `~/.basecamp/notes.json` (`notes_http.go`); `useNotes()` in `web/src/notes.js`
  moves a browser's old localStorage notes up on first load, and falls back to localStorage
  when no server answers.
- The dashboard is served over plain http, so `navigator.clipboard` may be missing or refuse;
  copy buttons need the textarea fallback (see `PlaybookActions` in `components.jsx`).
- The Claude app's browser pane blocks scripts from the tailnet host; preview at
  http://localhost:3050 instead.
- `-launchd` makes `basecamp jobs` skip `~/Documents` and iCloud Drive (privacy prompts would
  hang it); it finds snapshots from the paths the job's log names. Run it by hand to read folders.
- Global Packages lists npm globals for the current (default) nvm Node only, plus pip (skipped for
  Apple's `/usr/bin/pip3` and with pyenv) and cargo. npm and corepack are left out because they ship
  with Node. An empty tab can be accurate: on 2026-11-24 the manual and launchd scans both found none.
- Status values the UI understands: `active`, `current`, `running` (good), `outdated`, `stale`
  (count as issues and lower the score).

## UI conventions (2026-11-24, matched to borrowd, Stacks, Financial Insider, Papers)

- Size everything in rem, never px, so it grows with the root: 16px on phones and laptops, 17px
  from 1280px wide, 18px from 1600px (`styles.css`, the same rule as borrowd). Only 1-2px hairlines
  stay px.
- Text uses the type scale in `styles.css`: `fontSize: 'var(--text-sm)'` (xs 12, sm 14, base 16,
  lg 18, xl 20, 2xl 24, Tailwind's steps). Body and nav are `base`, secondary lines `sm`,
  uppercase labels and badges `xs`, page titles `2xl`. Nothing smaller than `xs`.
- Styles stay inline (no Tailwind here); anything that needs a breakpoint is a class in
  `styles.css`: `.shell/.sidebar/.main/.topbar` (phones get a top bar and the sidebar as a drawer),
  `.page` (padding, 112rem cap), `.page-head`, `.btn`/`.btn-primary`/`.btn-quiet`, `.icon-btn`,
  `.nav-btn`, `.modal-scrim`/`.modal` (bottom sheet on phones), and the row grids `.item-grid`,
  `.file-grid`, `.job-grid` with `a-*` areas that restack under 768px. Inline styles win over
  classes, so don't set a class's property inline too.
- Phones (under 768px): tap targets at least `--tap` (44px), table headers hidden, rows stack.
- Colors: Financial Insider and Papers neutrals (`--bg #0f1117`, `--card #181a24`,
  `--border #2a2d3e`) with basecamp's indigo accent. Dark only.
- Nothing moves after a tap (borrowd's rule): busy buttons keep their label, dimmed with
  `aria-busy`; a label that changes reserves the longer width (Copy/Copied).
- Check a UI change at phone (375), laptop (1440) and wide (2560) widths. Headless Chrome won't
  go under 500px wide, so use the browser pane's mobile size for phones.

## Open work (as of 2026-11-24, evening)

Everything from the first passes is merged and deployed (PRs #1 to #4): `basecamp serve` on :3050,
hourly scan, Jobs check every 15 minutes, dotfiles saved and pushed at 03:00. The first Jobs scan's
findings are all fixed in the other repos (logs out of `/tmp`, diecast-scout in the offsite image,
Springboard's plist); on 2026-11-24 it reported all 23 jobs fine.

1. **Cleanup the scan points at** (Mario's machine, not code): Node 15/16/18/20/23 under nvm are
   end-of-life, mongodb-community installed but not running, a leftover "VPN by Google One" launch
   agent, Remote WSL and three Docker extensions in VS Code, a 5.4-year-old SSH key.
2. **`basecamp diff`** (compare two scans) is not built.
3. **Status files:** no job writes one yet (`docs/jobs.md` shows how), so the Jobs tab reads logs.
   Optional; the nightly backups would benefit most.
4. Not verified in a browser: the playbook's "Download .sh" button (the Claude app's pane blocks
   downloads).

UI pass (2026-11-24, branch `claude/ui-pass-b919b3`): bigger rem-based sizing, phone layout, see
UI conventions above.

Decided, so don't redo: no Homebrew section (Mario, 2026-11-24: the Homebrew app,
`/Applications/Homebrew.app`, manages and updates packages). The Brewfile stays in the dotfiles
backup and the playbook installs from it. Public history: left as is (2026-11-24, no secrets in it).
