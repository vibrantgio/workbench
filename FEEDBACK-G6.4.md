# FEEDBACK-G6.4 — Sitedocs adoption notes for vibrantgio/markdown

Running-notes style: severity-tagged `####` entries logged while moving the
sitedocs docs pages from hand-coded Go (`docs_content.go` paragraphs + code
cards) to embedded `.md` sources rendered with `vibrantgio/markdown`. The
first real-document consumer of the module: 11 pages, using headings,
paragraphs, emphasis, inline code, links, a bulleted list, a blockquote, a
GFM table, and chroma-highlighted `go`/`sh` fences.

#### [Major] An unpublished module cannot be pinned in a consumer go.mod — go.work does not shield the require line

The plan assumed sitedocs `go.mod` could carry
`require github.com/vibrantgio/markdown v0.1.0` with go.work overriding the
resolution. In practice every workspace build then fails with
`reading github.com/vibrantgio/markdown/go.mod at revision v0.1.0: … Repository
not found`: module-graph pruning fetches the *required version's* go.mod even
when the module is `use`'d in go.work, and the vibrantgio/markdown repo is not
on GitHub yet, so there is nothing to fetch. Removing the require line builds
fine — workspace mode resolves the import directly from `use ./markdown`. So
until the module is pushed and tagged, the consumer go.mod cannot mention it
at all; only the chroma/goldmark/regexp2 indirect lines could land now (they
become accurate the moment the markdown require is added). The usual
push+tag+consumer-bump flow has a bootstrap gap for brand-new modules: the
consumer adoption can ship, but its go.mod completion has to trail the first
tag.

#### [Minor] No inline-code styling hook — `` `code` `` spans only switch typeface

`Style.spanStyles` maps `Span.Code` to the mono typeface and nothing else;
`CodeColor`/`CodeBackground` apply to code *blocks* only. Inline code in the
docs prose (`prism/tokens`, `ColorTokens`, …) is readable but visually
indistinguishable from body text apart from the face — no colour shift, no
background chip like GitHub renders. An `InlineCodeColor` (and ideally an
optional background) on `Style` would let inline code read as code. Worked
around by nothing — accepted the default look.

#### [Minor] Uniform `BlockGap` gives section headings no extra air

Every sibling block is separated by the same `BlockGap` (S2 = 8 dp). In a real
document a `##` heading that follows a code block sits as close to the block
above as two paragraphs sit to each other, so sections don't visually separate
(visible on every sitedocs page). Typographic rhythm wants roughly 2–3× the
gap above a heading. A per-class spacing hook — e.g. `HeadingTopGap`, or a
`Style.GapAbove(Block) unit.Dp` — would fix it; overriding `BlockGap` globally
just inflates everything.

#### [Minor] Appearance-matched highlighting is every consumer's boilerplate

`FromTokens` deliberately leaves `Highlight` nil, and `highlight.New` takes a
chroma style name — so each app re-derives "am I on a dark ground?" to choose
`"github"` vs `"github-dark"`. Sitedocs now carries a Rec. 601 luma check on
`ColorTokens.Background` for exactly this. Either a documented pairing helper
(`highlight.ForTokens(c tokens.ColorTokens)`) or a README recipe would stop
the next consumer from re-inventing the luminance threshold.

#### [Preserve] Adoption collapsed ~250 lines of bespoke rendering into Parse + NewDocument + Layout

The whole card/caption/code-line pipeline in docs.go (per-card observables,
atomic token mirrors, fixed-height cards, manual scroll list) reduced to one
`markdown.NewDocument(markdown.Parse(src))` per page plus a single
`doc.Layout(gtx, shaper, style)` call site. Per-page Document allocation slots
perfectly into the existing "build every page once, select on route" MVU
wiring: scroll position and link state survive navigation and theme changes
with zero extra glue, exactly as the pointer-identity state maps promise.

#### [Preserve] FromTokens defaults were production-ready in both themes

The token-derived style rendered every construct correctly on first run in
light and dark — heading scale, quote bar on Primary, SurfaceVariant code
grounds, table header emphasis, link colouring — and the goldens confirmed
light/dark parity without any per-block styling code in the app.
