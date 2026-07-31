# AGENTS.md — workbench

The seven reference applications of the VibrantGio design system, each a
complete product built the way a real one is meant to be built — MVU state,
spectrum theming, prism components, cadence patterns: `todos` (the smallest
complete bootstrap, and the place to start), `iconbrowser`, `sitedocs`,
`feeds`, `watchlist`, `mindchat` and `launcher`. The repository also
carries the architecture documents — `DESIGN.md` and `BASELINE.md` — that
no other repository in the organization has.

**Layer.** Outside ADR-001's tier table. `workbench` has no root module,
and nothing in the design system imports it; its applications sit on top of
the whole stack and are its only end-to-end consumers. Between them they
import mvu, spectrum, prism, pulse, cadence, markdown, style, textdraw and
backdrop, plus the ivg, svg, seen and noise support libraries.

**Read the canonical guide before you write code against this module.** It is
the organization's one agent guide — the module inventory with current tags,
the application skeleton, the MVU loop and rx semantics, typography, and the
pitfalls that are not guessable. It lives exactly once, in `vibrantgio/.github`,
and this file links it rather than copying it:

    https://raw.githubusercontent.com/vibrantgio/.github/master/llms.txt

**Modules.** No module at the repository root: this repository is seven
modules in subdirectories — `feeds/`
(`github.com/vibrantgio/workbench/feeds`), `iconbrowser/`
(`github.com/vibrantgio/workbench/iconbrowser`), `launcher/`
(`github.com/vibrantgio/workbench/launcher`), `mindchat/`
(`github.com/vibrantgio/workbench/mindchat`), `sitedocs/`
(`github.com/vibrantgio/workbench/sitedocs`), `todos/`
(`github.com/vibrantgio/workbench/todos`), `watchlist/`
(`github.com/vibrantgio/workbench/watchlist`). Each is built, tested and
tagged on its own, with tags that carry the directory as a prefix.

**Build and test.** Inside each of those module directories; there is no
root module to run it from:

    go build ./... && go test ./...

**Golden images.** Tests in two module directories — `feeds/` and
`sitedocs/` — compare rendered output against PNGs committed under
`testdata/golden/`. When a change legitimately moves pixels, regenerate
them within the same change, look at what came out, and say so in the
commit. From inside the directory concerned:

    go test . -golden.update

The flag comes last on purpose: `go test` cannot tell that an unfamiliar
flag is boolean, so anything after it stops being a package argument.

**The `.gitignore` denies everything by default.** Its first line is `*`, and
what follows re-admits exactly: Markdown at any level, `LICENSE`, `llms.txt`,
`.claude/skills/**`, and the seven application trees minus their compiled
binaries. A file you add anywhere else — a script, a new top-level directory,
a `.json` fixture outside an app — does not show up in `git status` and is
silently not committed. Check with `git check-ignore -v <path>` before
concluding a write failed.

**`llms.txt` here is a signpost, not the guide.** The canonical agent guide
moved to `vibrantgio/.github` in task A1.2 (ADR-004); the three lines left
behind point at its raw URL, which is the URL above. Do not restore content
into it.

**`README.md` and `DESIGN.md` are behind the code.** The README still
describes three example applications where there are seven, and DESIGN.md
predates the layering ADR-001 sets out. F2.2 and F2.3 rewrite them; until
then, trust the application source over either document.
