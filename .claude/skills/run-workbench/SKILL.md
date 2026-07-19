---
name: run-workbench
description: Build, run, screenshot, and stop the workbench Gio apps (launcher, mindchat, todos, feeds, watchlist, iconbrowser, sitedocs) on macOS. Use for "run the app", "launch mindchat", "take a screenshot of the launcher", or capturing app images for docs/READMEs.
---

# Run the workbench apps

Every app in this repo is a native Gio window binary with no CLI surface:
you run it, a window opens on the real display, and you drive it by
screenshot (there is no headless mode). The driver at
`.claude/skills/run-workbench/driver.sh` handles the whole loop: build,
launch, wait for the window, capture, stop. All paths below are relative
to the workbench root; artifacts land in `/tmp/workbench-run/`.

## Prerequisites

macOS with Xcode CLT (`go`, `swiftc`, `screencapture` — all present
here). The terminal needs the Screen Recording permission; it was
already granted in this environment.

## Run + screenshot (agent path)

```sh
.claude/skills/run-workbench/driver.sh launch launcher
.claude/skills/run-workbench/driver.sh shoot launcher /tmp/workbench-run/launcher.png
.claude/skills/run-workbench/driver.sh stop launcher
```

`launch <app>` builds the app (go.work resolves the sibling library
repos), starts it detached, and polls until the window exists — it
prints `window <id>` on success, or the log tail from
`/tmp/workbench-run/<app>.log` on failure. `shoot` captures the window
(shadowless, silent) at window point size — 1100×788 for the launcher on
this display. **Always Read the PNG afterwards** — a capture of a blank
frame means the app crashed after the window appeared.

`<app>` is any app directory name: `launcher`, `mindchat`, `todos`,
`feeds`, `watchlist`, `iconbrowser`, `sitedocs`.

## MindChat: never capture real user state

MindChat keeps chats, config, and **real API keys** under
`~/Library/Application Support/nl.simpleapps/mindchat`. For any capture
that leaves the machine, run it against seeded demo state in a scratch
HOME:

```sh
.claude/skills/run-workbench/driver.sh seed-mindchat /tmp/workbench-run/home
.claude/skills/run-workbench/driver.sh launch mindchat /tmp/workbench-run/home
.claude/skills/run-workbench/driver.sh shoot mindchat /tmp/workbench-run/mindchat.png
.claude/skills/run-workbench/driver.sh stop mindchat
```

The seed writes a config with two placeholder-key providers (header chip
reads "Default · OpenAI · gpt-5.5"), four sidebar chats, and one
conversation whose last answer carries citations, so the web-search
Sources rendering shows. The second `launch` argument sets HOME and
clears `OPENAI_API_KEY` (a fresh config would otherwise seed itself with
the real key from the environment).

## Run (human path)

`go run ./launcher` from the repo root — a window opens; quit with the
window close button. The launcher's cards themselves run
`go run ./<app>/` relative to its cwd, which is why the driver starts
apps from the repo root.

## Test

Per app: `go test ./...` inside the app directory (state-only reducer
tests; no window needed).

## Gotchas

- Window lookup is by process name (`<app>-bin`, via
  `windowid.swift` → `CGWindowListCopyWindowInfo`). Neither
  `GetWindowID` (brew) nor Python Quartz bindings are installed here —
  the Swift helper exists because of that; the driver compiles it once
  to `/tmp/workbench-run/windowid` (~1s, cached).
- If two copies of an app run, `shoot` captures an arbitrary one —
  `stop` strays first.
- Apps keep running after the driver exits (detached); every `launch`
  needs a matching `stop` or windows accumulate.
- The apps render in the **current OS appearance** (spectrum live
  theming) — captures come out light or dark depending on the system
  setting at capture time, and two apps captured minutes apart can
  differ if auto-switching crosses the boundary.
- Chat file names are display names: the sidebar shows them with the
  `.jsonl` extension stripped, so seeded files must be named like
  `reactive layouts.jsonl`, not `chat-01.jsonl`.
- `stop` may print a harmless `Terminated: 15` job-control line when
  `launch` and `stop` for the same app ran in one shell chain; the
  capture and exit codes are unaffected.

## Troubleshooting

- `no window appeared for <app>-bin` with an empty log: the build
  succeeded but the window server refused the connection — check the
  app isn't already running under the same name (`pkill -f <app>-bin`)
  and retry.
- Capture shows the desktop instead of the app: Screen Recording
  permission is missing for the terminal — grant it in System
  Settings → Privacy & Security → Screen Recording, then restart the
  terminal.
