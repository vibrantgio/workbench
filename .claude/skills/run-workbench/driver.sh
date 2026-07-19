#!/bin/bash
# driver.sh — build, launch, screenshot, and stop the workbench Gio apps
# on macOS. Windows are located by process name (windowid.swift) and
# captured with screencapture(1). Binaries, logs, and the compiled
# window-id helper live under /tmp/workbench-run.
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SKILL_DIR/../../.." && pwd)"
WORK=/tmp/workbench-run
mkdir -p "$WORK"

usage() {
  echo "usage: driver.sh launch <app> [scratch-home]   # build + start, waits for the window" >&2
  echo "       driver.sh shoot <app> <out.png>         # screenshot the app's window" >&2
  echo "       driver.sh stop <app>                    # kill the app" >&2
  echo "       driver.sh seed-mindchat <scratch-home>  # demo config + chats for clean captures" >&2
  exit 64
}

# Compile the window-id helper once; ~1s, cached across runs.
WID="$WORK/windowid"
if [ ! -x "$WID" ] || [ "$SKILL_DIR/windowid.swift" -nt "$WID" ]; then
  swiftc -O -o "$WID" "$SKILL_DIR/windowid.swift"
fi

window_id() { "$WID" "$1-bin" | head -1 | cut -d' ' -f1; }

cmd=${1:-}
[ -n "$cmd" ] || usage
shift
case "$cmd" in
launch)
  app=${1:?app name required}
  home=${2:-}
  (cd "$ROOT/$app" && go build -o "$WORK/$app-bin" .)
  if [ -n "$home" ]; then
    # Scratch HOME isolates per-user state (mindchat: config, chats,
    # API keys). OPENAI_API_KEY is cleared so a fresh config cannot
    # seed itself with the real key; cwd avoids any .env autoload.
    mkdir -p "$home"
    (cd "$home" && HOME="$home" OPENAI_API_KEY= "$WORK/$app-bin" >"$WORK/$app.log" 2>&1 &)
  else
    # The launcher runs `go run ./<app>/` relative to its cwd, so all
    # apps start from the workbench root.
    (cd "$ROOT" && "$WORK/$app-bin" >"$WORK/$app.log" 2>&1 &)
  fi
  for _ in $(seq 1 30); do
    id=$(window_id "$app")
    if [ -n "$id" ]; then
      echo "window $id"
      exit 0
    fi
    sleep 0.5
  done
  echo "no window appeared for $app-bin; log tail:" >&2
  tail -5 "$WORK/$app.log" >&2
  exit 1
  ;;
shoot)
  app=${1:?app name required}
  out=${2:?output .png path required}
  id=$(window_id "$app")
  [ -n "$id" ] || { echo "no window for $app-bin (launch it first)" >&2; exit 1; }
  screencapture -o -x -l "$id" "$out"
  echo "captured $out"
  ;;
stop)
  pkill -f "${1:?app name required}-bin" || true
  ;;
seed-mindchat)
  home=${1:?scratch-home path required}
  D="$home/Library/Application Support/nl.simpleapps/mindchat"
  mkdir -p "$D/chats"
  cat >"$D/config.json" <<'EOF'
{
  "LastChat": "reactive layouts.jsonl",
  "SidebarRatio": 0,
  "SidebarCollapsed": false,
  "Providers": [
    {"Name": "OpenAI", "BaseURL": "", "APIKey": "sk-demo-not-a-real-key", "WebSearch": true, "Models": ["gpt-5.5", "gpt-5.5-codex", "gpt-5.5-mini"]},
    {"Name": "xAI", "BaseURL": "https://api.x.ai/v1", "APIKey": "xai-demo-not-a-real-key", "WebSearch": true, "Models": ["grok-4", "grok-4-fast"]}
  ],
  "DefaultProvider": "OpenAI",
  "DefaultModel": "gpt-5.5"
}
EOF
  cat >"$D/chats/reactive layouts.jsonl" <<'EOF'
{"time":"2026-07-19T09:41:03+02:00","type":"user","text":"In MVU, how does a button click reach the update function?"}
{"time":"2026-07-19T09:41:07+02:00","type":"assistant","text":"The widget never calls back into your code — it records a message. The clickable's layout writes a MessageOp carrying your message into the frame's operation list; after the frame, the mvu window collects every MessageOp and feeds them to the loop, where Update reduces each one against the current Model. The next frame simply renders the new Model. One direction, no callbacks: view → ops → message → update → model → view."}
{"time":"2026-07-19T09:42:31+02:00","type":"user","text":"So state never lives in the widget?"}
{"time":"2026-07-19T09:42:36+02:00","type":"assistant","text":"Only ephemeral gesture state — press tracking, scroll position, an editor's cursor. Everything the app must remember reduces into the Model: the sidebar split, the open popover, even an in-flight completion stream is a Model field whose id routes the arriving deltas.","citations":[{"url":"https://gioui.org/doc/architecture","title":"Gio — Architecture"},{"url":"https://github.com/vibrantgio/mvu","title":"vibrantgio/mvu — Model-View-Update runtime for Gio"}]}
EOF
  printf '%s\n' '{"time":"2026-07-18T14:02:11+02:00","type":"user","text":"When is a constraint solver worth it over flexbox?"}' >"$D/chats/cassowary layout.jsonl"
  printf '%s\n' '{"time":"2026-07-17T11:20:45+02:00","type":"user","text":"Why does simplex noise look less gridded than Perlin?"}' >"$D/chats/simplex noise.jsonl"
  printf '%s\n' '{"time":"2026-07-16T16:33:29+02:00","type":"user","text":"Should clicking outside a popover always dismiss it?"}' >"$D/chats/popover dismissal.jsonl"
  echo "seeded $D"
  ;;
*)
  usage
  ;;
esac
