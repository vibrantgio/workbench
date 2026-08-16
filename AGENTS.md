# AGENTS.md — workbench

The eight reference applications of the Vibrant Gio design system, each a
complete product built the way a real one is meant to be built — MVU state,
live theming from theme, components widgets, patterns: `todos` (the
smallest complete bootstrap, and the place to start), `iconbrowser`,
`sitedocs`, `feeds`, `watchlist`, `mindchat`, `launcher` and `vaultview`
(the document reader — a folder of markdown notes browsed through a file
tree, wikilinks followed, backlinks in the shell's aside slot). The
repository also carries `BASELINE.md`, the measured performance baselines
the component benchmarks compare against — an application-history document
that stayed behind when the system's architecture rationale (`DESIGN.md`)
moved to the `design` repository, whose git history for it lives here.

**Layer.** Outside ADR-001's tier table: applications at the top of the
stack, which the tier rule exempts and which may import any layer of the
design system. Its eight applications import, between them, `backdrop`,
`components`, `effects`, `font`, `ivg`, `ivg/raster/gio`, `markdown`,
`mvu`, `mvu/desktop`, `noise`, `patterns`, `seen`, `seen/context/gio`,
`svg`, `svg/driver/gio`, `textdraw` and `theme`. Nothing in the
organization imports it. Both directions are measured rather than typed —
`scripts/check-layers.sh --edges` reports the graph and
`scripts/sync-agents.sh` renders these sentences from it — so correcting
them here changes nothing.

**Read the canonical guide before you write code against this module.** It is
the organization's one agent guide — the module inventory with current tags,
the application skeleton, the MVU loop and rx semantics, typography, and the
pitfalls that are not guessable. It lives exactly once, in `vibrantgio/.github`,
and this file links it rather than copying it:

    https://raw.githubusercontent.com/vibrantgio/.github/master/llms.txt

**Modules.** No module at the repository root: this repository is eight
modules in subdirectories — `feeds/`
(`github.com/vibrantgio/workbench/feeds`), `iconbrowser/`
(`github.com/vibrantgio/workbench/iconbrowser`), `launcher/`
(`github.com/vibrantgio/workbench/launcher`), `mindchat/`
(`github.com/vibrantgio/workbench/mindchat`), `sitedocs/`
(`github.com/vibrantgio/workbench/sitedocs`), `todos/`
(`github.com/vibrantgio/workbench/todos`), `vaultview/`
(`github.com/vibrantgio/workbench/vaultview`), `watchlist/`
(`github.com/vibrantgio/workbench/watchlist`). Each is built, tested and
tagged on its own, with tags that carry the directory as a prefix.

**Build and test.** Inside each of those module directories; there is no
root module to run it from:

    go build ./... && go test ./...

**Golden images.** Tests in four module directories — `feeds/`,
`sitedocs/`, `vaultview/` and `watchlist/` — compare rendered output
against PNGs committed under `testdata/golden/`. They render through
`github.com/vibrantgio/components/golden`, which declares `-golden.update`
and is shared with `design`, `effects`, `markdown` and `patterns`. Do not
inline a copy of it, and do not declare a second `-golden.update`: two
registrations of one flag name in a single test binary panic in `flag.Bool`
at init, before any test runs. When a change legitimately moves pixels,
regenerate them within the same change, look at what came out, and say so
in the commit. From inside the directory concerned:

    go test . -golden.update

The flag comes last on purpose: `go test` cannot tell that an unfamiliar
flag is boolean, so anything after it stops being a package argument.

**Nothing but a developer's machine has ever compared these images.** This
repository has no CI workflow, so the stored PNGs are checked only where
they are regenerated. That is not the weaker half of an arrangement — it is
the same guarantee the four repositories with CI have, for the reason F5.7
measured: a golden test whose `headless.NewWindow` fails answers with
`t.Skipf` and passes, and installing the drivers that would make it render
instead turns nine of pulse's twenty-one images red, because the
organization's goldens were recorded on macOS and Linux mesa does not
reproduce them exactly. CI there gates the build and the tests and not the
pixels, by decision. Here there is simply no run to ask.

**A golden test pins its faces; application code does not.** Every golden
and pixel test here builds its shaper with
`tokens.DefaultTypography.DeterministicShaper()` — the default typography's
faces and nothing else, system fonts off, so the stored PNGs are the same
on every machine. Applications call `Shaper()` instead, which falls back to
the platform's own fonts so that text outside Roboto and Roboto Mono still
resolves. The two are not interchangeable: a golden written against
`Shaper()` passes on the machine that wrote it and fails on one with a
different font set, which is the failure the split constructor exists to
make impossible.

When a test genuinely needs a glyph the default faces lack, widen the
collection rather than reach for the system:

    tokens.DefaultTypography.WithFaces(notosansmono.FontFace()).DeterministicShaper()

Then assert that the shaper resolved the rune, rather than storing the
result as pixels. A stored image proves the glyph came out somewhere; only
the assertion says which face drew it.

**`watchlist/` and `mindchat/` are the two exceptions to the sentence above.**
`watchlist/` kept its two images in `testdata/` directly until F5.5, where a
sweep keyed on the `golden/` path silently missed them; moving onto the shared
harness moved them into line, since the harness resolves that path itself and
no longer takes the caller's word for it. `mindchat/` has pixel tests too, but
they diff two renders in memory rather than storing an image, so it links the
harness without appearing in the list. F5.5 deleted the five inlined copies
these apps carried between them — `sitedocs/` alone had two, in adjacent
files.

**The `.gitignore` denies everything by default.** Its first line is `*`, and
what follows re-admits exactly: Markdown at any level, `LICENSE`, `llms.txt`,
`.claude/skills/**`, and the eight application trees minus their compiled
binaries. A file you add anywhere else — a script, a new top-level directory,
a `.json` fixture outside an app — does not show up in `git status` and is
silently not committed. Check with `git check-ignore -v <path>` before
concluding a write failed.

**`llms.txt` here is a signpost, not the guide.** The canonical agent guide
moved to `vibrantgio/.github` in task A1.2 (ADR-004); the three lines left
behind point at its raw URL, which is the URL above. Do not restore content
into it.

**`PLAN.md` here is a finished plan against a design that has since been
rewritten.** Phases −1 through 6, every task checked off — `mdplan next
PLAN.md` prints DONE, which is how to re-check it — and its header names
`DESIGN.md` as its source of truth. That document no longer lives in this
repository: G0E.3 moved the rationale — the second edition F2.2 wrote and
the archived `DESIGN-v1.md` this plan was actually written against — into
`vibrantgio/design`, and their pre-move history stays in this repository's
log. `BASELINE.md` stayed here: it is coinviz's performance capture, an
application's history rather than the system's rationale. `FEEDBACK-G6.4.md`
is one of this plan's outputs, filed against `vibrantgio/markdown`. The
organization's plan lives in `vibrantgio/.github`; do not execute this one.

**`README.md` lists every application in the repository, and gains a
section whenever one is added.** It used to describe three where there were
seven — and this note used to say so — until F2.3 rewrote it around the
whole set; read it as current and keep it that way. Where a document and
the application source disagree, the source wins and the document is a bug
to file.

**Arbiters are created in each application's layer function, and that is the
composition root that matters.** ADR-008 gave `patterns`'s popover, tooltip and
modal a plain `Arbiter` passed through `Props`, and the value *is* the scope —
one set per window. `theme/window.Render` calls the build function once per
window, and `feeds`, `watchlist` and `mindchat` each compose every arbitrable
widget they own inside exactly one layer built there (`feedsShellLayer`,
`watchlistShellLayer`, `ContentLayer`), so the arbiters are made in those
function bodies and threaded down through fifteen builder call sites. A second
arbitrable layer would have to take them as parameters, because it would be
composed beside this one rather than within it. The other four applications
compose none of these components and have none.

**Toasts are model state here, not a side channel.** `toast.Queue` lives in
`feeds`' and `watchlist`' models, `toast.Requested`/`toast.Expired` are
reduced in `Update`, and every call site raises one with
`toast.Notify(gtx, ...)` on the same `gtx.Ops` its neighbouring application
message already uses. A toast is therefore visible to a test that drives the
app through messages, which it was not before.
