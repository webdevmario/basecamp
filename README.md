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
| `basecamp jobs` | Check scheduled jobs and backups into `~/.basecamp/jobs.json` ([docs/jobs.md](docs/jobs.md)) |

## Server API

| Route | Description |
|---|---|
| `GET /scan.json` | The latest scan |
| `POST /api/scan` | Scan now, save it, return it (the dashboard's Run Scan button) |
| `GET /api/health` | `{"ok": true, "scanUpdated": ...}` |
| `GET` / `PUT /api/notes` | Per-item notes, stored in `~/.basecamp/notes.json` |
| `GET /jobs.json` | The latest jobs report (Jobs tab) |
| `GET /api/jobs/status` | One green/red answer for Springboard, CORS open |

## Dashboard

- **Health**: items marked outdated (a newer version exists) or stale (probably a leftover) lower
  the score. "Show issues only" filters to them.
- **Notes**: the ＋ on any row. Saved on the server, so every device sees the same notes.
- **Runtimes**: click a runtime with several versions (nvm, pyenv) to compare their global packages
  and get the command that copies missing ones over.
- **Jobs** (`/#jobs`): every scheduled LaunchAgent, its last run and result, a timeline of the
  night, log tails, and whether each backup is fresh and reaches the offsite image. See
  [docs/jobs.md](docs/jobs.md).
- **Export Playbook**: a bash script that reinstalls your brew packages, VS Code extensions, macOS
  defaults and runtimes. Copy it or download it as a `.sh`.
