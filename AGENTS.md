# AGENTS.md — workbench

The eight reference applications of the Vibrant Gio design system, each a
complete product built the way a real one is meant to be built — MVU state,
live theming from theme, components widgets, patterns: `todos` (the
smallest complete bootstrap, and the place to start), `iconbrowser`,
`sitedocs`, `feeds`, `mindchat`, `launcher`, `vaultview` (the document
reader — a folder of markdown notes browsed through a file tree, wikilinks
followed, backlinks in the shell's aside slot) and `themer` (a brand colour
picked out of a dropped picture, the window re-theming to the candidate
chosen, and the one worth keeping written where every application here
adopts it). Applications only: the system's architecture rationale
(`DESIGN.md`) moved to the `design` repository, whose git history for it
lives here, and development planning lives in `.github`.

**Layer.** Outside ADR-001's tier table: applications at the top of the
stack, which the tier rule exempts and which may import any layer of the
design system. Its eight applications import, between them, `backdrop`,
`components`, `components/gallery`, `effects`, `font`, `ivg`,
`ivg/raster/gio`, `markdown`, `mvu`, `mvu/desktop`, `noise`, `patterns`,
`seen`, `seen/context/gio`, `svg`, `svg/driver/gio`, `textdraw` and
`theme`. That direction is measured rather than typed —
`scripts/check-layers.sh --edges` reports the graph and
`scripts/sync-agents.sh` renders these sentences from it — so correcting
them here changes nothing. The other direction is measured too and
deliberately not written down: the gate checks the graph both ways, but a
public API's consumers are unknowable, so this file says what its module
needs and never who needs it.

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
(`github.com/vibrantgio/workbench/sitedocs`), `themer/`
(`github.com/vibrantgio/workbench/themer`), `todos/`
(`github.com/vibrantgio/workbench/todos`), `vaultview/`
(`github.com/vibrantgio/workbench/vaultview`). Each is built, tested and
tagged on its own, with tags that carry the directory as a prefix.

**Build and test.** Inside each of those module directories; there is no
root module to run it from:

    go build ./... && go test ./...

**Golden images.** Tests in three module directories — `feeds/`,
`sitedocs/` and `vaultview/` — compare rendered output against PNGs
committed under `testdata/golden/`. They render through
`github.com/vibrantgio/components/golden`, which declares `-golden.update`
and is the organization's only golden harness. Do not inline a copy of it,
and do not declare a second `-golden.update`: two registrations of one flag
name in a single test binary panic in `flag.Bool` at init, before any test
runs. When a change legitimately moves pixels, regenerate them within the
same change, look at what came out, and say so in the commit. From inside
the directory concerned:

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

**`mindchat/` is the exception to the sentence above.** It has pixel tests
too, but they diff two renders in memory rather than storing an image, so it
links the harness without appearing in the list. No application inlines a
copy of that harness any more — `sitedocs/` alone once carried two, in
adjacent files.

**The `.gitignore` denies everything by default.** Its first line is `*`, and
what follows re-admits exactly: Markdown at any level, `LICENSE`,
`.claude/skills/**`, and the eight application trees minus their compiled
binaries. A file you add anywhere else — a script, a new top-level directory,
a `.json` fixture outside an app — does not show up in `git status` and is
silently not committed. Check with `git check-ignore -v <path>` before
concluding a write failed.

**There is no `llms.txt` here.** The canonical agent guide lives once, in
`vibrantgio/.github`, at the raw URL above; this repository links it and
keeps neither a copy nor a pointer file. Do not add one back.

**Development planning does not happen here.** The organization's plan lives
once, in `vibrantgio/.github`. The finished plan this repository was built
against, the performance baselines it measured and the feedback notes it
filed were removed once that became true; their content is in this
repository's git log and nowhere else. The architecture rationale
`DESIGN.md` left earlier, for `vibrantgio/design`, together with its
archived first edition `DESIGN-v1.md`; that pre-move history stays in this
log too.

**`README.md` lists every application in the repository.** It gains a
section when one is added and loses one when an application is removed, so
read it as current and keep it that way. Where a document and the
application source disagree, the source wins and the document is a bug to
file.

**Arbiters are created in each application's layer function, and that is the
composition root that matters.** ADR-008 gave `patterns`'s popover, tooltip and
modal a plain `Arbiter` passed through `Props`, and the value *is* the scope —
one set per window. `theme/window.Render` calls the build function once per
window, and `feeds` and `mindchat` each compose every arbitrable widget they
own inside exactly one layer built there (`feedsShellLayer`, `ContentLayer`),
so the arbiters are made in those function bodies and threaded down through
every builder call site below them. A second arbitrable layer would have to
take them as parameters, because it would be composed beside this one rather
than within it. The other five applications compose none of these components
and have none.

**Toasts are model state here, not a side channel.** `toast.Queue` lives in
the `feeds` and `vaultview` models, `toast.Requested`/`toast.Expired` are
reduced in `Update`, and every call site raises one with
`toast.Notify(gtx, ...)` on the same `gtx.Ops` its neighbouring application
message already uses. A toast is therefore visible to a test that drives the
app through messages, which it was not before.
