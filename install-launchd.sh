#!/usr/bin/env bash
set -euo pipefail

CONFIG=""
LABEL=""
LOG_PATH=""
BINARY="${HOME}/bin/backlog-sync"
LOAD=0

usage() {
  cat <<'USAGE'
Usage: install-launchd.sh --config <path> --label <label> --log <path> [--binary <path>] [--load]

Renders ~/Library/LaunchAgents/<label>.plist for one backlog-sync config.
It does not run launchctl unless --load is passed.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --config) CONFIG="$2"; shift 2 ;;
    --label) LABEL="$2"; shift 2 ;;
    --log) LOG_PATH="$2"; shift 2 ;;
    --binary) BINARY="$2"; shift 2 ;;
    --load) LOAD=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if [[ -z "$CONFIG" || -z "$LABEL" || -z "$LOG_PATH" ]]; then
  usage >&2
  exit 2
fi
if [[ "$CONFIG" != /* || "$LOG_PATH" != /* || "$BINARY" != /* ]]; then
  echo "--config, --log, and --binary must be absolute paths" >&2
  exit 2
fi

ROOT="$(cd "$(dirname "$CONFIG")" && pwd -P)"
PLIST_DIR="${HOME}/Library/LaunchAgents"
PLIST="${PLIST_DIR}/${LABEL}.plist"
mkdir -p "$PLIST_DIR" "$(dirname "$LOG_PATH")"

xml_escape() {
  python3 -c 'import html,sys; print(html.escape(sys.stdin.read().strip(), quote=True))'
}

BINARY_XML="$(printf '%s' "$BINARY" | xml_escape)"
CONFIG_XML="$(printf '%s' "$CONFIG" | xml_escape)"
ROOT_XML="$(printf '%s' "$ROOT" | xml_escape)"
LABEL_XML="$(printf '%s' "$LABEL" | xml_escape)"
LOG_XML="$(printf '%s' "$LOG_PATH" | xml_escape)"

cat > "$PLIST" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>${LABEL_XML}</string>
  <key>ProgramArguments</key>
  <array>
    <string>${BINARY_XML}</string>
    <string>--config</string>
    <string>${CONFIG_XML}</string>
  </array>
  <key>WorkingDirectory</key>
  <string>${ROOT_XML}</string>
  <key>StartInterval</key>
  <integer>300</integer>
  <key>RunAtLoad</key>
  <true/>
  <key>StandardOutPath</key>
  <string>${LOG_XML}</string>
  <key>StandardErrorPath</key>
  <string>${LOG_XML}</string>
  <key>EnvironmentVariables</key>
  <dict>
    <key>PATH</key>
    <string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    <key>HOME</key>
    <string>${HOME}</string>
  </dict>
</dict>
</plist>
PLIST

echo "wrote $PLIST"
if [[ "$LOAD" -eq 1 ]]; then
  launchctl bootstrap "gui/$(id -u)" "$PLIST" 2>/dev/null || true
  launchctl kickstart -k "gui/$(id -u)/${LABEL}"
fi
