# basecamp: notes for Claude sessions

Mac inventory and health dashboard for Mario's Mac mini: a Go CLI scans the machine (dotfiles,
Homebrew, VS Code, runtimes, global packages, fonts, macOS defaults, SSH keys, services) into JSON,
and a React + Vite dashboard shows it with health scores, per-item notes and a setup playbook.

**The GitHub repo is public.** Never commit a scan, the compiled binary, or anything read from this
Mac (paths, hostnames, keys, dotfile contents).

## Layout

- `cli/`: Go, no dependencies. `main.go` (commands), `scan_*.go` (one scanner per category, each
  returns a `Category`), `types.go` (JSON shape), `helpers.go` (`run`, `runLines`, file helpers),
  `serve.go` (HTTP server).
- `web/`: React 19 + Vite. `App.jsx` (everything on screen), `health.js` (scores and observations),
  `playbook.js` (bash setup script), `notes.js` (per-item notes in `localStorage`),
  `VersionDetail.jsx` (nvm/pyenv versions compared), `NoteModal.jsx`.
- `scripts/install-service.sh`: builds and installs the launchd jobs.

## Running

- Live: http://shady-mac-mini.taila545c3.ts.net:3050 (Springboard tile "basecamp").
- launchd `com.basecamp`: `basecamp serve` on :3050, KeepAlive. Serves `~/.basecamp/app/web`,
  `GET /scan.json` from `~/.basecamp/scan.json`, `POST /api/scan` (rescan now), `GET /api/health`.
- launchd `com.basecamp-scan`: `basecamp scan -o ~/.basecamp/scan.json` at load and every hour.
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
- `run()` in `helpers.go` has no timeout; a hung command (an `osascript` permission prompt, a slow
  `brew outdated`) hangs the whole scan.
- `web/node_modules` is committed, so `npm ci` shows deleted files in git. `git checkout --
  web/node_modules` undoes it until it is untracked.
- The Claude app's browser pane blocks scripts from the tailnet host; preview at
  http://localhost:3050 instead.
- Status values the UI understands: `active`, `current`, `running` (good), `outdated`, `stale`
  (count as issues and lower the score).

## Open work (as of 2026-11-24)

1. **Privacy (Mario's call).** `web/public/scan.json` (a March 2026 scan of this Mac) and
   `cli/basecamp` (binary with `/Users/Mario/...` build paths) have been public since the first
   commit; `web/node_modules` and `.DS_Store` are tracked too. Proposed: untrack them and add a
   `.gitignore`. Rewriting history or making the repo private is Mario's decision. The main
   checkout has uncommitted changes to both files; untracking upstream will make `git pull` there
   complain until those local copies are moved aside.
2. **Pass 1 fixes:** waiting on Mario's picks from the 2026-11-24 review (bug list in the PR).
3. **`basecamp diff`** is in the README as "coming soon" but does not exist.
4. Parallel work on other branches: a Jobs view (`basecamp-jobs`) and config tracking
   (`basecamp-dotfiles`). They build on `basecamp serve`; deploy with the install script.
