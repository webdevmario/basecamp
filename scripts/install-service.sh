#!/bin/zsh
# Installs (or reinstalls) basecamp as two launchd jobs, the same way the other
# Mac mini apps run:
#   com.basecamp        `basecamp serve` on port 3050, restarted if it stops
#   com.basecamp-scan   `basecamp scan` every hour (and at load) into ~/.basecamp/scan.json
#
# It builds the CLI and the dashboard from this checkout and copies them to
# ~/.basecamp/app, so the service keeps running when a worktree is removed.
# Re-run it after pulling changes.
#
#   scripts/install-service.sh            # build, install, start
#   scripts/install-service.sh --remove   # stop and uninstall (keeps ~/.basecamp data)
set -eu
PORT=3050
SERVE_LABEL=com.basecamp
SCAN_LABEL=com.basecamp-scan
SCAN_EVERY=3600
ROOT="${0:A:h:h}"
DATA="$HOME/.basecamp"
APP="$DATA/app"
LOGS="$DATA/logs"
AGENTS="$HOME/Library/LaunchAgents"
DOMAIN="gui/$(id -u)"

stop() {
  launchctl bootout "$DOMAIN/$1" 2>/dev/null || true
  # bootout returns before the old job is gone; bootstrapping too soon fails with "5: Input/output error".
  for i in {1..20}; do
    launchctl print "$DOMAIN/$1" >/dev/null 2>&1 || break
    sleep 0.5
  done
}

start() {
  for i in {1..5}; do
    launchctl bootstrap "$DOMAIN" "$AGENTS/$1.plist" 2>/dev/null && return 0
    (( i == 5 )) && { launchctl bootstrap "$DOMAIN" "$AGENTS/$1.plist"; exit 1; }
    sleep 1
  done
}

stop "$SERVE_LABEL"
stop "$SCAN_LABEL"
if [[ "${1:-}" == "--remove" ]]; then
  rm -f "$AGENTS/$SERVE_LABEL.plist" "$AGENTS/$SCAN_LABEL.plist"
  echo "Removed $SERVE_LABEL and $SCAN_LABEL (data left in $DATA)"
  exit 0
fi

# Build
source "$HOME/.nvm/nvm.sh" >/dev/null
mkdir -p "$APP" "$LOGS"
(cd "$ROOT/cli" && go build -o "$APP/basecamp.new" . && mv "$APP/basecamp.new" "$APP/basecamp")
(cd "$ROOT/web" && npm ci --no-audit --no-fund --silent && npm run build --silent)
rm -rf "$APP/web.new"
cp -R "$ROOT/web/dist" "$APP/web.new"
rm -f "$APP/web.new/scan.json"   # scans are served from $DATA, never from the build
rm -rf "$APP/web" && mv "$APP/web.new" "$APP/web"

# launchd starts jobs with a bare PATH; give the scanner the same tools a login shell has.
ENV_PATH="/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
PRELUDE='source "$HOME/.nvm/nvm.sh" >/dev/null 2&gt;&amp;1; '

plist() { # label, command, extra keys
  cat >"$AGENTS/$1.plist" <<PL
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>$1</string>
  <key>ProgramArguments</key>
  <array>
    <string>/bin/bash</string><string>-c</string>
    <string>${PRELUDE}exec $2</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>PATH</key><string>$ENV_PATH</string>
    <key>SHELL</key><string>/bin/zsh</string>
  </dict>
  <key>RunAtLoad</key><true/>
$3
  <key>StandardOutPath</key><string>$LOGS/$1.log</string>
  <key>StandardErrorPath</key><string>$LOGS/$1.log</string>
</dict>
</plist>
PL
}

plist "$SERVE_LABEL" "\"$APP/basecamp\" serve -addr :$PORT -web \"$APP/web\" -data \"$DATA\"" \
  "  <key>KeepAlive</key><true/>"
plist "$SCAN_LABEL" "\"$APP/basecamp\" scan -o \"$DATA/scan.json\"" \
  "  <key>StartInterval</key><integer>$SCAN_EVERY</integer>"

start "$SCAN_LABEL"
start "$SERVE_LABEL"
echo "Installed $SERVE_LABEL on port $PORT: http://$(hostname -s | tr '[:upper:]' '[:lower:]').taila545c3.ts.net:$PORT"
echo "Installed $SCAN_LABEL: scans every $((SCAN_EVERY / 60)) min into $DATA/scan.json (logs in $LOGS)"
