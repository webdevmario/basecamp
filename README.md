# basecamp

A developer environment inventory, health dashboard, and setup playbook generator for macOS.

It scans dotfiles, Homebrew, VS Code extensions, runtimes (nvm, pyenv, uv, rbenv, Go, Rust, Bun,
Deno), global packages, fonts, macOS defaults, SSH keys and background services, then shows what
is outdated or likely left over, keeps your notes per item, and writes a bash script that would
set up a new Mac the same way.

## Structure

```
basecamp/
├── cli/        # Go: the scanner (basecamp scan) and the server (basecamp serve)
├── web/        # React + Vite dashboard
└── scripts/    # install-service.sh: run it as launchd jobs
```

## Quick start

```bash
cd cli
go build -o basecamp .
./basecamp scan -o ~/.basecamp/scan.json
cd ../web && npm install && npm run build
../cli/basecamp serve -web dist
```

Open http://localhost:3050.

Scans contain an inventory of your machine. They live in `~/.basecamp/`, outside the repo, and
`.gitignore` keeps them out of commits.

## Run it all the time (launchd)

```bash
scripts/install-service.sh
```

This builds the CLI and the dashboard into `~/.basecamp/app` and installs three launchd jobs:

| Job | What it does |
|---|---|
| `com.basecamp` | `basecamp serve` on port 3050, restarted if it stops |
| `com.basecamp-scan` | `basecamp scan` at login and every hour, into `~/.basecamp/scan.json` |
| `com.basecamp-jobs` | `basecamp jobs` at login and every 15 minutes, into `~/.basecamp/jobs.json`; notifies when a job turns red |

The server only answers tailnet (Tailscale) and localhost clients. Re-run the script after pulling
changes; `scripts/install-service.sh --remove` uninstalls (your data in `~/.basecamp` stays).

## Develop

```bash
cd web && npm run dev
```

Vite proxies `/scan.json` and `/api` to `basecamp serve` on port 3050, so the dev page shows the
live scan and notes.

## CLI

| Command | Description |
|---|---|
| `basecamp scan` | Full environment scan, JSON to stdout |
| `basecamp scan -pretty` | Pretty-printed JSON |
| `basecamp scan -o file.json` | Write to a file (atomically) |
| `basecamp serve` | Serve the dashboard and API (`-addr :3050`, `-web dir`, `-data ~/.basecamp`) |
| `basecamp dotfiles status` | Which configs are in sync, changed since the last save, or not saved yet |
| `basecamp dotfiles diff [name]` | What changed since the last save |
| `basecamp dotfiles save [--push]` | Copy changed configs into the private repo and commit |
| `basecamp dotfiles restore [--apply]` | Copy saved configs back (dry run without `--apply`) |
| `basecamp dotfiles schedule` | Save and push every night at 03:00 (launchd) |
| `basecamp jobs` | Check scheduled jobs and backups into `~/.basecamp/jobs.json` ([docs/jobs.md](docs/jobs.md)) |

## Server API

| Route | Description |
|---|---|
| `GET /scan.json` | The latest scan |
| `POST /api/scan` | Scan now, save it, return it (the dashboard's Run Scan button) |
| `GET /api/health` | `{"ok": true, "scanUpdated": ...}` |
| `GET` / `PUT /api/notes` | Per-item notes, stored in `~/.basecamp/notes.json` |
| `GET /api/dotfiles` | Status of every saved config, with diffs |
| `POST /api/dotfiles/save` | Save changed configs, commit and push (needs an `X-Basecamp` header) |
| `GET /jobs.json` | The latest jobs report (Jobs tab) |
| `GET /api/jobs/status` | One green/red answer for Springboard, CORS open |

## Dashboard

- **Health**: items marked outdated (a newer version exists) or stale (probably a leftover) lower
  the score. "Show issues only" filters to them.
- **Notes**: the ＋ on any row. Saved on the server, so every device sees the same notes.
- **Runtimes**: click a runtime with several versions (nvm, pyenv) to compare their global packages
  and get the command that copies missing ones over.
- **Configs**: what `basecamp dotfiles` keeps saved, with diffs and a Save button.
- **Jobs** (`/#jobs`): every scheduled LaunchAgent, its last run and result, a timeline of the
  night, log tails, and whether each backup is fresh and reaches the offsite image. See
  [docs/jobs.md](docs/jobs.md).
- **Export Playbook**: a bash script that reinstalls your brew packages, VS Code extensions, macOS
  defaults and runtimes. Copy it or download it as a `.sh`.

## Configs (dotfiles)

basecamp keeps copies of your configs in a **private** git repo cloned at
`~/.basecamp/dotfiles` (override with `BASECAMP_DOTFILES_DIR`): shell rc files,
git and SSH config, iTerm2 preferences (as XML, following its custom prefs folder),
VS Code and Zed settings, the gh and Claude Code config, a generated `Brewfile`
and the VS Code extension list. The list lives in `cli/dotfiles.go`.

- Contents never go into this repo; only into the private one.
- A file with anything that looks like a secret (tokens, `_authToken`, private keys,
  `FOO_PASSWORD=...`) is refused, not stripped, and the save says which line and why.
  Add `basecamp:allow` to a line that is a false positive. SSH keys, `known_hosts`,
  `gh/hosts.yml`, `.netrc` and similar are never copied.
- Files are copied, not symlinked, so apps that rewrite their configs keep working.

```bash
basecamp dotfiles init git@github.com:<you>/dotfiles.git   # once
basecamp dotfiles save --push
```

On a new Mac: `basecamp dotfiles init <url>`, then `basecamp dotfiles restore --apply`
(anything replaced is backed up to `~/.basecamp/backups/`), then the `brew bundle` and
`code --install-extension` lines it prints.
