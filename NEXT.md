# NEXT — Cursor for the active milestone

**Active goal:** `G−1.5d` in [PLAN.md](./PLAN.md) (Phase −1 — Gio Migration; sub-goal of `G−1.5`, target: `traer/gio/*`).

---

## Prompt for Claude Sonnet 4.6

You are working a single SMART goal from `PLAN.md` in the current repository. Your job: discharge **goal `G−1.1`** to its acceptance criterion, and nothing more.

### Step 1 — Load only what you need (token discipline)

`PLAN.md` and `DESIGN.md` are large. Do **not** `Read` them whole. Use `mdedit` to fetch only the slices you need:

```bash
# The active goal — read this first.
mdedit read --section "G−1.1" --scope tree PLAN.md

# The SMART contract and anti-goals (one-time refresher).
mdedit read --section "SMART contract for every goal" --scope tree PLAN.md

# DESIGN.md sections cited by G−1.1's "Relevant:" line.
# Read each only if the goal text is ambiguous on intent.
mdedit outline DESIGN.md                              # to find heading names
mdedit read --section "Phase −1" --scope tree DESIGN.md
mdedit read --section "Known Fragilities" --scope tree DESIGN.md
```

If the goal references neighbouring goals (e.g., "Depends on G−1.2"), read those headings the same way — never the whole plan.

### Step 2 — Honour the SMART contract

The goal's own `Specific / Measurable / Achievable / Relevant / Budget` lines are the contract. In particular:

- **Measurable** is the only definition of done. Do not declare success until that exact criterion is met.
- **Budget** caps your session at ≤100 K Sonnet 4.6 tokens total. If you are approaching the cap with the goal incomplete, stop and split per the `Split:` note (or propose one) — do not blow the budget.
- **Achievable** rules out scope expansion. Do not refactor adjacent code, rename modules, or "while I'm here" cleanups. If you find unrelated issues, note them at the bottom of this file under `## Followups` and move on.

### Step 3 — Execute

1. Read the goal via `mdedit` (Step 1).
2. Reproduce, in one or two lines at the top of your reply, the **Measurable** criterion verbatim — this anchors you.
3. Do the work. Prefer focused tool calls; avoid full-tree greps when a targeted `grep -rn <pattern> <dir>` suffices.
4. Run the verification command(s) from **Measurable**. Paste the output. If green, you are done.
5. If the goal produces a doc artefact (e.g., `MIGRATION.md`, `BASELINE.md`, `DECISIONS.md`, `EXPERIMENT-*.md`), write it now — that artefact *is* the deliverable.

### Step 4 — Hand-off

When the Measurable criterion is met:

1. Update this file: change the **Active goal** line to the next goal in topological order (consult `mdedit outline PLAN.md` — pick the next `### G…` after the one just completed, respecting `‖` parallelism and `Depends on` notes).
2. Append a one-line entry to `## Completed` below: `- Gx.y — <heading> — <one-line summary> — <YYYY-MM-DD>`.
3. Commit to git. The cursor advance must be durable before the session ends — otherwise a future session will re-run the goal you just finished.

   ```bash
   # Stage the goal's deliverables AND the cursor update together.
   # Adjust the file list to whatever this goal actually produced.
   git add NEXT.md <files-the-goal-touched>
   git status                     # sanity check — no stray edits
   git commit -m "$(cat <<'EOF'
   <Gx.y>: <one-line summary matching the Completed entry>

   Discharges PLAN.md goal <Gx.y>. Advances NEXT.md cursor to <next goal>.
   EOF
   )"
   ```

   Rules for the commit:
   - **One commit per goal.** Do not bundle multiple goals into one commit, and do not split one goal across commits unless the goal itself defines a `Split:`.
   - **Do not push** unless explicitly asked. The user controls remote state.
   - **Do not skip hooks** (`--no-verify`, `--no-gpg-sign`). If a hook fails, fix the underlying issue and create a *new* commit; never amend a hook-failed commit.
   - If the working directory is not yet a git repo, stop and ask the user before running `git init` — that is a project-level decision, not a per-goal one.

4. Stop. Do **not** start the next goal in the same session — a fresh session preserves the token budget.

### Hard rules

- Do not edit `PLAN.md` or `DESIGN.md` unless the goal explicitly says so. They are inputs, not work product.
- Do not invent acceptance criteria. If the goal's Measurable line is vague, surface that as a question rather than guessing.
- Do not skip tests. A goal without its tests is not done, even if `go build` is green.
- If you discover the goal is mis-scoped (over- or under-budget, or its premise is wrong), stop and report. A re-plan beats a forced delivery.

---

## Completed

<!-- Append one line per finished goal, newest at the bottom. -->
- G−1.1 — Pin and audit current Gio usage — produced MIGRATION.md with 53 call sites across 4 deprecated API patterns (window.Events, InputOp, InvalidateOp{}, ops.Internal) — 2026-05-05

- G−1.2 — Migrate `mvu/window.go` event loop — rewrote against `app.Window.Event()` goroutine adapter; fixed `message.go` (`event.Op`); corrected `unsafeOps` struct (`version uint32`); stripped removed `font.Font.Variant` field — 2026-05-05

- G−1.3 — Replace `unsafe.Pointer` MessageOp extraction — replaced unsafe ops-cast with package-level `*op.Ops`-keyed collector map; `Add` writes to collector, frame observer drains it; zero unsafe in `mvu/` — 2026-05-05

- G−1.4 — Migrate `coinviz` — bumped go.mod to Gio v0.9.0; replaced `pointer.InputOp`/`gtx.Events` with `event.Op`/`gtx.Source.Event`; fixed `text.NewShaper`; app runs 13 s on BTC-USD without panic — 2026-05-05

- G−1.5a — Migrate `appviz` — bumped go.mod to Gio v0.9.0; replaced `gesture.Click.Events(gtx.Queue)` and `gesture.Hover.Hovered(gtx.Queue)` with `Update(gtx.Source)`; wrapped `style.FontFaces()` in `text.WithCollection` for new `text.NewShaper` signature; app launches, fetches sales reports, and renders without panic — 2026-05-06

- G−1.5b — Migrate `todos` — bumped go.mod to Gio v0.9.0; replaced `clickable.Clicked()` with `Clicked(gtx)`, `gtx.Queue` with `gtx.Source` pattern, `check.Changed()` with `check.Update(gtx)`, `edit.Events()` with `edit.Update(gtx)` loop before Layout, `key.InputOp` with `event.Op`+`key.Filter`; `text.NewShaper` wrapped with `text.WithCollection`; app builds and launches — 2026-05-06

- G−1.5c — Migrate `mindchat` — bumped go.mod to Gio v0.9.0; replaced `text.NewShaper` with `text.WithCollection`, `edit.Focus()` with focusRequested+`key.FocusCmd`, `edit.Events()` with `edit.Update(gtx)` loop before Layout, `clickable.Clicked()` with `Clicked(gtx)`; app launches and renders first frame — 2026-05-06

## Followups

<!-- Out-of-scope observations spotted while working a goal. Do not act on them here. -->

- `todos` and `mindchat` are broken at v0.9 (widget API: `clickable.Clicked(gtx)`, `gtx.Queue` removed, `edit.Events` removed, font shaper mismatch). Pre-existing before G−1.2; to be addressed in G−1.5.
