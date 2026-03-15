# basecamp

A developer environment inventory, health dashboard, and setup playbook generator for macOS.

## Structure

```
basecamp/
├── cli/          # Go CLI scanner — scans your machine, outputs JSON
├── web/          # React + Vite dashboard — visualizes scan data
└── README.md
```

## Quick Start

### 1. Run the scanner

```bash
cd cli
go build -o basecamp .
./basecamp scan > ../web/public/scan.json
```

### 2. Start the dashboard

```bash
cd web
npm install
npm run dev
```

Open `http://localhost:5173` — the dashboard reads `scan.json` on load.

### Workflow

1. Run `basecamp scan` whenever you want a fresh snapshot
2. The dashboard picks up the new data on refresh
3. Your personal notes are stored in `~/.basecamp/notes.json` (persisted across scans)

## CLI Commands

| Command | Description |
|---------|-------------|
| `basecamp scan` | Full environment scan, outputs JSON to stdout |
| `basecamp scan --pretty` | Pretty-printed JSON output |
| `basecamp scan -o file.json` | Write directly to a file |
| `basecamp diff <old.json> <new.json>` | Compare two scans (coming soon) |
