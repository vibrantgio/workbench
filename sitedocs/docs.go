// docs.go composes the docs pages. Each page stacks a patterns/breadcrumb
// row over a vibrantgio/markdown Document rendered from the page's
// embedded .md source (docs_content.go): type-scale headings, richtext
// prose with links, chroma-highlighted code blocks, lists, blockquotes,
// and tables. The runtime entry point is docsPage; the static counterpart
// renderDocsPage is used by goldens.

package main

import (
	"image/color"
	"os/exec"
	"runtime"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/patterns/breadcrumb"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"

	"github.com/vibrantgio/mvu"
)

// Page layout constants. The outer inset matches the marketing-pattern
// inset (S6 = 24 dp) so the docs page reads at the same canvas inset as
// the landing page.
//
// docsBreadcrumbGapDp is the fallback for the seam between the page chrome
// and the reading column; docsBreadcrumbGap prefers the document's own
// number and is what the page actually lays out with.
const (
	docsOuterInsetDp    = 24
	docsProseGapDp      = 12
	docsCardGapDp       = 16
	docsBreadcrumbGapDp = 32
	// docsMeasureDp caps the guide document's line length to a readable
	// column; the window is wider than a comfortable measure.
	docsMeasureDp = 720
)

// docsBreadcrumbGap is the blank between the breadcrumb row and the top of
// the document: the space the renderer would itself put above a level-1
// heading that had a block above it. The page title is the document's first
// block, so it reaches nothing above and the app owns that space instead —
// and owning it as a hardcoded number is what goes stale, because the
// document's rhythm is derived from the theme and moves when the theme
// does. Sized from the chrome's side it would be wrong in the other
// direction: a seam narrower than the blank between two paragraphs binds
// the title upward to the chrome rather than downward to the prose it
// opens. A style built by hand carries no heading spaces, and then the
// constant stands in.
func docsBreadcrumbGap(style markdown.Style) float32 {
	if above := style.HeadingSpaceAbove[0]; above > 0 {
		return float32(above)
	}
	return docsBreadcrumbGapDp
}

// Chroma styles for the two appearance modes; built once, shared by every
// page. FromTokens leaves Highlight nil, so assigning these is the app's
// opt-in to syntax highlighting.
var (
	docsHighlightLight = highlight.New("github")
	docsHighlightDark  = highlight.New("github-dark")
)

// docsMarkdownStyle derives the markdown document style for the current
// colour and typography tokens: the token-themed defaults plus the app's
// two opt-ins — chroma highlighting matched to the appearance, and links
// opening in the system browser.
//
// FromTokens now takes the whole Typography (it spends several roles), so
// the type argument the deleted TypeScale used to carry is gone. Mono and
// CodeSize are still re-resolved from the theme's Code role (F1.4):
// FromTokens defaults them from the Typography it is handed, and setting
// them here keeps that explicit at the call site.
func docsMarkdownStyle(c tokens.ColorTokens, typ tokens.Typography) markdown.Style {
	st := markdown.FromTokens(c, typ)
	st.Mono = font.Typeface(typ.Code.Typeface)
	st.CodeSize = unit.Sp(typ.Code.Size)
	if isDarkColor(c.Background) {
		st.Highlight = docsHighlightDark
	} else {
		st.Highlight = docsHighlightLight
	}
	st.Text.OnLinkClick = func(_ layout.Context, url string) { openURL(url) }
	return st
}

// isDarkColor reports whether c reads as a dark ground (Rec. 601 luma
// below mid-grey), selecting the dark chroma style.
func isDarkColor(c color.NRGBA) bool {
	luma := 0.299*float32(c.R) + 0.587*float32(c.G) + 0.114*float32(c.B)
	return luma < 128
}

// openURL opens an absolute web URL in the system browser. Non-http(s)
// destinations are ignored — the docs sources only carry web links.
func openURL(url string) {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// docsPage returns the runtime observable for the named page. The markdown
// Document is allocated once per page and closed over by every emission,
// so scroll position and link interaction state survive theme changes and
// navigation. The breadcrumb and token observables are combined so the
// page re-emits on any theme change. The shaper is the theme's cached
// Typography shaper, so prose shapes in Roboto and the code blocks'
// Style.Mono face resolves to the theme-carried Roboto Mono (F0.2).
func docsPage(
	th rx.Observable[theme.Theme],
	def docsPageDef,
) rx.Observable[layout.Widget] {
	bcObs := breadcrumb.Breadcrumb(th, breadcrumb.Props{
		Items: docsBreadcrumb(def),
	})

	doc := markdown.NewDocument(markdown.Parse(def.Source))

	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Typography] { return t.Typography })
	tokensObs := rx.CombineLatest2(colObs, typObs)

	full := rx.CombineLatest2(bcObs, tokensObs)
	return rx.Map(full, func(t rx.Tuple2[layout.Widget, rx.Tuple2[tokens.ColorTokens, tokens.Typography]]) layout.Widget {
		bcW := t.First
		typ := t.Second.Second
		style := docsMarkdownStyle(t.Second.First, typ)
		shaper := typ.Shaper()
		return func(gtx layout.Context) layout.Dimensions {
			return drawDocsPage(gtx, bcW, doc, shaper, style)
		}
	})
}

// renderDocsPage is the static counterpart of docsPage used by goldens: a
// fresh top-scrolled Document laid out once with the given token sets. The
// breadcrumb takes the TitleSmall role its live path uses; the code role
// comes from the same Typography, matching what the runtime path renders.
func renderDocsPage(
	shaper *text.Shaper,
	def docsPageDef,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	typo tokens.Typography,
) layout.Widget {
	bcW := breadcrumb.Render(shaper, breadcrumb.Props{Items: docsBreadcrumb(def), Shaper: shaper}, colors, sp, typo.TitleSmall)
	doc := markdown.NewDocument(markdown.Parse(def.Source))
	style := docsMarkdownStyle(colors, typo)
	return func(gtx layout.Context) layout.Dimensions {
		return drawDocsPage(gtx, bcW, doc, shaper, style)
	}
}

// docsBreadcrumb returns the breadcrumb trail for a docs page: Home
// (clickable) / layer / title. Callbacks emit mvu.MessageOp so
// navigation fires on the same frame as the click.
func docsBreadcrumb(def docsPageDef) []breadcrumb.Item {
	layer := def.Layer
	if layer == "" {
		layer = "Docs"
	}
	return []breadcrumb.Item{
		{Label: "Home", OnClick: func(gtx layout.Context) {
			mvu.MessageOp{Message: SetRoute{Page: pageHome}}.Add(gtx.Ops)
		}},
		{Label: layer},
		{Label: def.Title},
	}
}

// drawDocsPage lays out one docs page frame: the breadcrumb row pinned at
// the top, then the markdown document filling the rest as its own
// scrolling viewport.
func drawDocsPage(
	gtx layout.Context,
	bcW layout.Widget,
	doc *markdown.Document,
	shaper *text.Shaper,
	style markdown.Style,
) layout.Dimensions {
	inset := complayout.Inset(docsOuterInsetDp)
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if bcW != nil {
					return bcW(gtx)
				}
				return layout.Dimensions{}
			}),
			layout.Rigid(complayout.VSpacer(docsBreadcrumbGap(style))),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return doc.Layout(gtx, shaper, style)
			}),
		)
	})
}

// drawGuideDoc lays out the one guide document as the Docs shell's main
// slot: the shared outer inset, then the document as its own scrolling
// viewport. The document is long-lived — scroll position and interaction
// state belong to it, so ScrollToBlock from an outline row moves the same
// reader rather than rebuilding the page.
func drawGuideDoc(
	gtx layout.Context,
	doc *markdown.Document,
	shaper *text.Shaper,
	style markdown.Style,
) layout.Dimensions {
	inset := complayout.Inset(docsOuterInsetDp)
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		// Cap the reading measure: unbounded, the guide's prose runs the
		// window's whole width — far past a comfortable line length
		// (fresh-eyes, AF2.1). The document keeps its left edge; spare
		// width stays blank.
		if cap := gtx.Dp(unit.Dp(docsMeasureDp)); gtx.Constraints.Max.X > cap {
			gtx.Constraints.Max.X = cap
			if gtx.Constraints.Min.X > cap {
				gtx.Constraints.Min.X = cap
			}
		}
		return doc.Layout(gtx, shaper, style)
	})
}

// guideDocObservable is the live main-slot stream for the one guide
// document: the same Document on every emission, restyled per theme
// change. The shaper is the theme's cached Typography shaper.
func guideDocObservable(
	th rx.Observable[theme.Theme],
	doc *markdown.Document,
) rx.Observable[layout.Widget] {
	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Typography] { return t.Typography })
	tokensObs := rx.CombineLatest2(colObs, typObs)
	return rx.Map(tokensObs, func(t rx.Tuple2[tokens.ColorTokens, tokens.Typography]) layout.Widget {
		typ := t.Second
		style := docsMarkdownStyle(t.First, typ)
		shaper := typ.Shaper()
		return func(gtx layout.Context) layout.Dimensions {
			return drawGuideDoc(gtx, doc, shaper, style)
		}
	})
}

// paragraphWidget renders one body-text paragraph in the given Typography
// role (BodyMedium at both call sites). (Used by the About page and the
// footer, whose short prose stays hand-composed.)
func paragraphWidget(
	shaper *text.Shaper,
	textBody string,
	fg color.NRGBA,
	style tokens.TextStyle,
) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		mColor := op.Record(gtx.Ops)
		paint.ColorOp{Color: fg}.Add(gtx.Ops)
		material := mColor.Stop()
		wl := typeset.Label(style, 0)
		wl.Alignment = text.Start
		return typeset.Layout(gtx, shaper, wl, typeset.Font(style, font.Normal),
			unit.Sp(style.Size), textBody, material)
	}
}
