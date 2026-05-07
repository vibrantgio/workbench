# NEXT — Cursor for the active milestone

**Active goal:** `G00.C1` in [PLAN.md](./PLAN.md) (Phase 00 — Validation Experiments; Experiment C1: Drag-drop with shared Subject).

---

## Prompt for Claude Sonnet 4.6

You are working a single SMART milestone from `PLAN.md` in the current repository. Your job: discharge the **Active goal** named at the top of this file (e.g., `G−1.6`) to its acceptance criterion, and nothing more. The five steps below are the contract — work them in order.

In the commands below, substitute `<G>` with the active goal ID from the cursor above (e.g., `G−1.6`).

### Step 1 — Gather milestone information from `PLAN.md`

`PLAN.md` and `DESIGN.md` are large. Do **not** `Read` them whole. Use `mdedit` to fetch only the slices you need:

```bash
# The active milestone — read this first.
mdedit read --section "<G>" --scope tree PLAN.md

# The SMART contract and anti-goals (one-time refresher).
mdedit read --section "SMART contract for every goal" --scope tree PLAN.md

# DESIGN.md sections cited by the milestone's "Relevant:" line.
# Read each only if the milestone text is ambiguous on intent.
mdedit outline DESIGN.md
mdedit read --section "<heading>" --scope tree DESIGN.md
```

If the milestone references neighbouring milestones (e.g., "Depends on Gx.y"), read those headings the same way — never the whole plan.

The milestone's own `Specific / Measurable / Achievable / Relevant / Budget` lines are the contract. In particular:

- **Measurable** is the only definition of done. Do not declare success until that exact criterion is met.
- **Budget** caps your session at ≤100 K Sonnet 4.6 tokens total. If you are approaching the cap with the milestone incomplete, stop and split per the `Split:` note (or propose one) — do not blow the budget.
- **Achievable** rules out scope expansion. Do not refactor adjacent code, rename modules, or "while I'm here" cleanups. If you spot unrelated issues, mention them in your final reply and move on — do not act on them.

### Step 2 — Reach the objectives for the milestone

1. Reproduce, in one or two lines at the top of your reply, the **Measurable** criterion verbatim — this anchors you.
2. Do the work. Prefer focused tool calls; avoid full-tree greps when a targeted `grep -rn <pattern> <dir>` suffices.
3. If the milestone produces a doc artefact (e.g., `MIGRATION.md`, `BASELINE.md`, `DECISIONS.md`, `EXPERIMENT-*.md`), write it as part of the work — that artefact *is* the deliverable.

### Step 3 — Verify the objectives were reached

1. Run the verification command(s) from **Measurable**. Paste the output. If the criterion is met verbatim, the milestone is done.
2. If verification fails, fix and re-run. Do not declare success until the **Measurable** criterion is met. A passing `go build` is not a substitute for the test, benchmark, or doc artefact named in **Measurable**.

### Step 4 — Change `NEXT.md` to point to the next milestone

Once **Measurable** is green:

1. **Tick the milestone's checkbox in `PLAN.md`** by toggling the `- [ ] **Done**` bullet under its heading:

   ```bash
   mdedit toggle -s "<G>" PLAN.md
   ```

2. **Rewrite this file (`NEXT.md`)** so it points to the next milestone:
   - Pick the next `### G…` heading whose `- [ ] **Done**` is still unchecked, in topological order. Consult `mdedit outline PLAN.md` and respect `‖` parallelism markers and `Depends on` notes in the milestone bodies.
   - Update the **Active goal** line at the top of `NEXT.md` to that milestone — heading title, phase, and one-line gist.
   - Keep `NEXT.md` minimal: cursor + this prompt + the "Working with markdown" blurb. No completion log, no followups, no history. NEXT.md is rewritten cleanly each turn — progress lives in `PLAN.md`'s checkboxes, history lives in `git log`.

### Step 5 — Commit all changes

The cursor advance must be durable before the session ends — otherwise a future session will re-run the milestone you just finished.

```bash
# Stage the milestone's deliverables AND the cursor update together.
# Adjust the file list to whatever this milestone actually produced.
git add NEXT.md PLAN.md <files-the-milestone-touched>
git status                     # sanity check — no stray edits
git commit -m "$(cat <<'EOF'
<G>: <one-line summary of what the milestone delivered>

Discharges PLAN.md milestone <G>. Advances NEXT.md cursor to <next milestone>.
EOF
)"
```

Rules for the commit:
- **One commit per milestone.** Do not bundle multiple milestones into one commit, and do not split one milestone across commits unless the milestone itself defines a `Split:`.
- **Do not push** unless explicitly asked. The user controls remote state.
- **Do not skip hooks** (`--no-verify`, `--no-gpg-sign`). If a hook fails, fix the underlying issue and create a *new* commit; never amend a hook-failed commit.

Stop. Do **not** start the next milestone in the same session — a fresh session preserves the token budget.

### Hard rules

- Do not edit `PLAN.md` body or `DESIGN.md` content other than toggling the milestone's `- [ ] **Done**` checkbox per Step 4. They are inputs, not work product.
- Do not invent acceptance criteria. If the milestone's **Measurable** line is vague, surface that as a question rather than guessing.
- Do not skip tests. A milestone without its tests is not done, even if `go build` is green.
- If you discover the milestone is mis-scoped (over- or under-budget, or its premise is wrong), stop and report. A re-plan beats a forced delivery.

---

## Working with markdown

Use the **mdedit** tool/skill to explore and edit markdown files (PLAN.md, NEXT.md, vault notes, fixtures). It is faster and more accurate for navigation, section-scoped edits, and structural changes than generic Read/Edit roundtrips.

If you hit friction with mdedit during the milestone — a missing capability, a rough edge, an awkward flow — run `mdedit log suggest` to capture a suggestion for improving the mdedit skill itself. That records the feedback in the right place without derailing the milestone work.
