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
  `playbook.js` (bash setup script), `notes.js` (per-item notes, saved through `/api/notes`),
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
- `run()` in `helpers.go` kills any command after 60s (`commandTimeout`), so a hung tool (an
  `osascript` permission prompt) leaves that category short instead of hanging the scan.
- Notes live in `~/.basecamp/notes.json` (`notes_http.go`); `useNotes()` in `web/src/notes.js`
  moves a browser's old localStorage notes up on first load, and falls back to localStorage
  when no server answers.
- The dashboard is served over plain http, so `navigator.clipboard` may be missing or refuse;
  copy buttons need the textarea fallback (see `PlaybookActions` in `components.jsx`).
- The Claude app's browser pane blocks scripts from the tailnet host; preview at
  http://localhost:3050 instead.
- Status values the UI understands: `active`, `current`, `running` (good), `outdated`, `stale`
  (count as issues and lower the score).

## Open work (as of 2026-11-24)

1. **Public history (Mario's call).** Pass 1 untracked `web/public/scan.json`, `cli/basecamp`,
   `web/node_modules` and `.DS_Store`, but the March 2026 scan and the binary are still in git
   history on the public repo (no secrets: hostname, software inventory, launch agent names,
   SSH key age, `/Users/Mario` build paths). Rewriting history or making the repo private is
   Mario's decision. The main checkout has local edits to both files: move them aside before
   `git pull` there, since upstream now deletes them.
2. **`basecamp diff`** is not built (the old README called it "coming soon").
3. **Parallel branches:** a Jobs view (`basecamp-jobs`) and config tracking (`basecamp-dotfiles`)
   build on `basecamp serve`. Each registers its routes with one line in `runServe`.
4. **Cleanup the scan points at** (Mario's machine, not code): Node 15/16/18/20/23 under nvm are
   end-of-life, 22 brew packages outdated, mongodb-community installed but not running, a leftover
   "VPN by Google One" launch agent, Remote WSL and three Docker extensions in VS Code, a
   5.4-year-old SSH key.
5. Not verified in a browser: the playbook's "Download .sh" button (the Claude app's pane blocks
   downloads). Copy, notes, Run Scan and the version compare were checked on 2026-11-24.
