---
name: mdedit
description: AST-aware Markdown section editing using mdedit. Use when editing specific sections of .md files by heading name.
---

You have access to `mdedit`, an AST-aware Markdown editor CLI. Use it instead of the Edit tool when working with Markdown sections by heading name — it parses the heading tree, so a single command can target a section without you having to count lines or copy surrounding context.

> Requires mdedit ≥ v0.0.2. If a command errors with "no help topic" or "flag provided but not defined", run `mdedit --version` — the binary is older than this skill assumes.

## Workflow

1. `mdedit outline <file>` — discover heading names and line ranges before any edit. Targets are matched against this list.
2. `mdedit read -s "<heading>" <file>` — see what's there before changing it.
3. Pick the right edit command (see "Choosing the right command" below) and run it. All mutating commands accept `--dry-run` to preview without touching the file.

Always start with `outline` on an unfamiliar file. The agent-friendly default is text format; use `--json` if downstream tooling needs to parse the structure.

## Targeting sections

`-s / --section` matches case-insensitive substring against the heading title. To address a nested heading unambiguously, use a slash-separated path:

```
mdedit read -s "Milestone M3/Goal" PLAN.md
```

The first segment matches at top-level; each subsequent segment is scoped to the previous match's descendants. `/` has no escape — for headings whose title contains a literal `/`, fall back to `--exact` or `--nth`.

When more than one heading matches and `--exact` is not set, mdedit refuses the edit and lists every match with its line number. Pick one of:

- `--exact` — strict equality (still case-insensitive)
- `--nth <N>` — pick the Nth match (1-indexed)
- a slash-path that disambiguates the level you mean

When nothing matches, mdedit exits with code 2 and prints up to three "did you mean" suggestions drawn from headings at the failing path level.

## Choosing the right command

Three layers of editing commands. Reach for the specialized commands first; they have one intent and are hard to misuse. Drop to `replace` only when one atomic operation actually beats decomposing.

| Goal | Command | Layer |
| --- | --- | --- |
| Replace a section's body (heading preserved) | `update` | specialized |
| Add content after an anchor section | `append` | additive |
| Add content before an anchor section | `prepend` | additive |
| Change a heading's title only | `rename` | specialized |
| Shift a heading's level (and its subtree) | `indent` / `outdent` | specialized |
| Remove a section, or dissolve and re-parent its children | `delete` | specialized |
| Toggle checkboxes inside a section | `toggle` | specialized |
| Atomic full-section rewrite (heading + body + descendants) | `replace` | jack-of-all-trades |
| Regenerate or renumber the Table of Contents | `toc` | specialized |

`replace` accepts all three `--scope` values and is the right choice when one atomic operation beats decomposing into specialized commands. When in doubt, prefer the specialized commands — they restrict scopes, refuse silent destructive defaults, and produce cleaner diffs.

## The `--scope` flag

Section commands take `--scope=body|flat|tree` to pick what region to operate on:

- `--scope=body` — body prose only (no heading line, no descendants)
- `--scope=flat` — heading + body, no descendants
- `--scope=tree` — heading + body + descendants (the full subtree)

Per-command defaults and accepted values:

| Command | Default | Accepted values |
| --- | --- | --- |
| `read` | `tree` | all three |
| `replace` | `tree` | all three |
| `delete` | `tree` | all three (each scope has a distinct intent — see below) |
| `toggle` | `tree` | all three |
| `update` | `tree` | `body`, `tree` only — `flat` is rejected |
| `append` | `tree` | `body`, `tree` only — `flat` is rejected |
| `prepend` | `tree` | `body`, `tree` only — `flat` is rejected |

`update`, `append`, and `prepend` reject `--scope=flat` because they place content at one point — `flat` would silently collapse onto another scope. The error points at the two valid choices.

`delete` keeps all three scopes because each one is a distinct operation:

- `delete --scope=tree` (default) — remove the heading, body, and every descendant.
- `delete --scope=flat` — dissolve the section: drop the heading + body, but re-parent the children under the previous sibling. Use this to collapse intermediate hierarchy while preserving substance.
- `delete --scope=body` — clear just the prose, keep the heading and descendants.

`--shallow` is gone (M6 breaking change). It errors with a hint pointing at `--scope=flat`.

## Available commands

### `outline`

```
mdedit outline PLAN.md          # text, indented, with line ranges
mdedit outline --json PLAN.md   # JSON tree for tooling
```

Text rows look like `Section Name  L<start>–L<end> (<n> lines)`.

### `read`

```
mdedit read -s "Section One" doc.md                # full subtree (default)
mdedit read -s "Section One" --scope=flat doc.md   # heading + body, no descendants
mdedit read -s "Section One" --scope=body doc.md   # body prose only
mdedit read -s "Milestone M3/Goal" PLAN.md         # path addressing
mdedit read -s "Goal" --nth 2 PLAN.md              # pick the 2nd of N matches
mdedit read -s "Section One" --exact doc.md        # strict equality
```

### `update` — replace a section's body, heading preserved

```
echo "new prose" | mdedit update -s "Section One" doc.md                 # default scope=tree wipes sub-headings
mdedit update -s "Section One" --from-file body.md doc.md                 # default
mdedit update -s "Section One" --scope=body --from-file body.md doc.md    # keep sub-headings
```

`update` is the cheapest way to replace a section's body. The heading line is always preserved by contract.

- Default `--scope=tree` replaces body and wipes sub-headings — natural when you want a clean slate.
- `--scope=body` preserves sub-headings; use when only the prose changes.
- `--scope=flat` is rejected (would silently alias onto another scope).

**Empty-stdin guard.** `update` refuses an empty input (stdin or `--from-file`) at any scope. Pass `--allow-empty` to confirm an intentional wipe. The guard catches the most common wipe-by-typo failure mode now that `tree` is the default.

### `append` — grow a section

```
echo "- new bullet" | mdedit append -s "Up next" PLAN.md                 # default scope=tree → after subtree
mdedit append -s "Section One" --scope=body --from-file note.md doc.md   # before sub-headings
```

- Default `--scope=tree` appends after the entire subtree (the natural "add to this section" reading).
- `--scope=body` appends inside the parent's own prose, before any sub-headings — for tucking content into a parent before its children.
- `--scope=flat` is rejected.

### `prepend` — add content before a section

```
echo "## New section

body
" | mdedit prepend -s "Anchor section" doc.md            # default scope=tree → preceding sibling, before the anchor's heading
mdedit prepend -s "Anchor" --scope=body --from-file note.md doc.md   # right after the anchor's heading, before existing body
```

`prepend` is the symmetric pair of `append`: same anchor, opposite side.

- Default `--scope=tree` lands content before the heading line as a preceding sibling — the natural place to add a new section *before* this anchor.
- `--scope=body` lands content right after the heading, before existing body and sub-headings — for tucking notes into the start of the anchor's own prose.
- `--scope=flat` is rejected (would alias onto body).

Whitespace handling mirrors `append`: leading and trailing blanks of the new content are trimmed, and at least one blank line is inserted as a separator. Empty input is a no-op.

### `rename` — change a heading title in place

```
mdedit rename -s "Up next" "Coming up" PLAN.md
mdedit rename -s "Milestone M3/Goal" --nth 2 "New goal title" PLAN.md
```

The new title is heading text only — no leading `#`. The level is inferred and not changed (use `indent`/`outdent` for level changes). Body and descendants stay byte-for-byte.

### `indent` / `outdent` — shift heading level

```
mdedit indent -s "Section One" doc.md          # ## → ###, descendants shift too
mdedit outdent -s "Section One" doc.md         # ## → #
mdedit indent -s "Section One" --by 2 doc.md   # multi-level shift
```

Shifts the matched heading's level by ±1 (or `--by N`). The whole subtree shifts recursively so the structure is preserved.

**Watch the parent change.** Shifting the level changes the heading's effective parent in the tree: `## A` between `# Top` and `## B` becomes `### A` after `indent` — now a child of `# Top` instead of a sibling of `## B`. This is the operation working as designed, but reach for `indent`/`outdent` deliberately when you want that re-parenting.

If any heading in the subtree would land outside `[1, 6]`, the command refuses without writing. Honors `-s`, `--exact`, `--nth`, slash-paths, `--dry-run`.

### `replace` (and the M1.1 safety net)

`replace` swaps an entire section's content and accepts all three `--scope` values. By default the heading line is *part of what gets replaced*, so stdin (or `--from-file`) must begin with a same-level ATX heading. If it doesn't, mdedit refuses the edit:

```
mdedit replace: stdin must begin with a level-N heading; pass --allow-removal to replace without one
```

This catches the most common silent-corruption failure mode (forgetting to repeat the heading). Either:

- include the heading in your input (the normal path), or
- pass `--allow-removal` to make it act like a delete, or
- use `--scope=body` to replace just the prose — at which point you almost certainly want `update` instead.

```
mdedit replace -s "Up next" --from-file new.md PLAN.md
echo '## Up next

new body
' | mdedit replace -s "Up next" PLAN.md
mdedit replace -s "Up next" --scope=flat --from-file f.md PLAN.md   # preserve descendants
```

Reach for `replace` when the edit spans heading + body + structure all at once and decomposing into `update` + `prepend`/`delete` would be more work than worth.

### `delete`, `toggle`, `toc`, `overwrite`

```
mdedit delete -s "Old idea" doc.md                  # remove subtree (default)
mdedit delete -s "Old idea" --scope=flat doc.md     # dissolve: drop heading + body, re-parent children under previous sibling
mdedit delete -s "Old idea" --scope=body doc.md     # clear body, keep heading + descendants
mdedit toggle -s "Tasks" --item "deploy" doc.md     # toggle checkboxes whose text contains 'deploy'
mdedit toc --renumber doc.md                        # regenerate ToC and renumber numbered headings (Arabic, depth from existing prefix)
mdedit toc --format x.I.1.a.i doc.md                 # renumber with custom scheme: H1 silent, H2=I, H3=1, H4=a, H5=i; --format implies --renumber
mdedit toc -s Inhoud doc.md                          # non-English TOC heading (default 'Table of Contents')
mdedit overwrite doc.md < new-full-file.md          # blow away the file (rare)
```

#### `toc --format <spec>` — custom renumber schemes

When you want headings numbered with mixed schemes (Roman parts, Arabic chapters, lettered sub-sections), pass a dot-separated format:

| Token | Scheme | Example |
| --- | --- | --- |
| `1` | Arabic | 1, 2, 3, ... |
| `I` | upper Roman | I, II, III, IV, ... |
| `i` | lower Roman | i, ii, iii, iv, ... |
| `A` | upper letter | A, B, ..., Z, AA, ... |
| `a` | lower letter | a, b, ..., z, aa, ... |
| `x` | silent (leading only) | counter participates in the hierarchy but isn't rendered |

Format positions map directly to heading levels: position 0 → H1, position 1 → H2, etc. So `I.1.a.i` numbers H1-H4. To skip H1 from the rendered numbering (typical for docs with a single title-style H1), use a leading `x`: `x.I.1.a.i` makes H1 silent and starts the visible numbering at H2.

`x` is allowed only as a leading token; rejecting `x` mid-format keeps rendered prefixes unambiguous. The format length caps the depth that gets prefixed — headings deeper than the format are left untouched.

Behavior is **aggressive**: any heading at a level the format covers gets a prefix, even if it didn't have one before. Existing prefixes (Arabic, Roman, letter) are detected and stripped before the new one is applied. If you want a heading to stay unnumbered, place it at a depth beyond the format length.

`--format` implies `--renumber`. Bare `--renumber` (no format) keeps the legacy Arabic-only behavior where depth is taken from the existing prefix string.

### `log` — feedback-loop namespace

`log` groups every command that operates on the mdedit data dir (`MDEDIT_DIR` if set, else `~/Library/Application Support/mdedit/` on macOS or `$XDG_CONFIG_HOME/mdedit/` on Linux). Five subcommands:

```
mdedit log on / off / status   # persistent toggle for usage logging
mdedit log suggest <text>      # append a free-form suggestion to suggestions.md
mdedit log stats [flags]       # summarise usage.log
```

#### `log suggest`

```
mdedit log suggest "rename --nth was hard to discover from --help"
```

If something feels clunky, fire `log suggest` once. It appends a timestamped entry to `suggestions.md` for the maintainer to review later. Cheap and doesn't break flow — use it instead of guessing whether a friction is "worth reporting".

#### `log on` / `log off` / `log status`

```
mdedit log on
mdedit log status
```

Pairs with `log suggest`. When on, every `mdedit` invocation appends one line to `usage.log` in the same data dir — `<RFC3339-ts> <args...> exit=<N> in=<bytes> out=<bytes> file=<bytes>` — capturing the silent friction that doesn't make it into a suggestion (commands that fail, args retried, sequences that should have been one call) plus the byte-traffic each call moved (M10.1). `file=` is omitted for commands without a positional `<file>` arg. State is persistent across shells, so flip it on at the start of an implementation session and leave it. `MDEDIT_LOG=1` / `MDEDIT_LOG=0` are still honored as one-off overrides; otherwise the sentinel file decides.

If the user asks why a command was retried or which mdedit recipes are expensive, suggest enabling `mdedit log on` so the data accumulates, then `mdedit log stats` summarises it.

#### `log stats`

```
mdedit log stats --since 1h          # text summary of recent activity
mdedit log stats --json              # same numbers, machine-readable
mdedit log stats --command update    # focus on one subcommand
mdedit log stats --tokens            # convert bytes → estimated tokens
```

Reads `usage.log` and reports per-command call count, exit-code distribution, and median/p90/p99 of in/out/file bytes. Old log entries (pre-M10.1) without byte fields are counted as calls but excluded from byte percentiles, so the distributions stay clean even after the format change. The `--tokens` flag divides byte counts by `--bytes-per-token` (default 3.5; try 2.5 for code-heavy markdown) for rough token estimates. Counterfactual savings ("X tokens vs reading the whole file") are deliberately not modelled — the no-mdedit alternatives are too contextual for any single-baseline number to mean much.

## Input plumbing: `--from-file` vs stdin

Every command that reads new content (`replace`, `prepend`, `update`, `append`, `overwrite`) accepts `--from-file <path>` as an alternative to stdin. Prefer it whenever the content has anything that wants escaping (backticks, `$`, embedded code fences, multi-line lists). The two are mutually exclusive — pass one, not both.

## Previewing changes: `--dry-run`

```
mdedit replace -s "Up next" --dry-run --from-file new.md PLAN.md
```

`--dry-run` prints a *bounded* preview: a header line naming the file/heading/line range plus the changed region with ±3 lines of context. The full file is NOT printed. For section edits on a 5000-line doc, the preview is a few hundred bytes instead of hundreds of KB.

If you genuinely need the entire post-edit file (e.g. to diff it externally), pass `--dry-run-full`. Default to bounded; reach for full only when you actually want it.

## Exit codes

- `0` — success
- `1` — generic failure (I/O error, invalid input, empty-stdin guard fired)
- `2` — no heading matches the query, or invalid flag value (e.g. `--scope=bogus`, `update --scope=flat`)
