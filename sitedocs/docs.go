// docs.go composes the docs pages. Each page stacks a cadence/breadcrumb
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
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/breadcrumb"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/highlight"
	pllayout "github.com/vibrantgio/prism/layout"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"

	"github.com/vibrantgio/mvu"
)

// Page layout constants. The outer inset matches the marketing-pattern
// inset (S6 = 24 dp) so the docs page reads at the same canvas inset as
// the landing page.
const (
	docsOuterInsetDp = 24
	docsProseGapDp   = 12
	docsCardGapDp    = 16
)

// Chroma styles for the two appearance modes; built once, shared by every
// page. FromTokens leaves Highlight nil, so assigning these is the app's
// opt-in to syntax highlighting.
var (
	docsHighlightLight = highlight.New("github")
	docsHighlightDark  = highlight.New("github-dark")
)

// docsMarkdownStyle derives the markdown document style for the current
// colour and type tokens: the token-themed defaults plus the app's two
// opt-ins — chroma highlighting matched to the appearance, and links
// opening in the system browser.
func docsMarkdownStyle(c tokens.ColorTokens, ts tokens.TypeScale) markdown.Style {
	st := markdown.FromTokens(c, ts)
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
// page re-emits on any theme change.
func docsPage(
	th rx.Observable[theme.Theme],
	shaper *text.Shaper,
	def docsPageDef,
) rx.Observable[layout.Widget] {
	bcObs := breadcrumb.Breadcrumb(th, breadcrumb.Props{
		Items:  docsBreadcrumb(def),
		Shaper: shaper,
	})

	doc := markdown.NewDocument(markdown.Parse(def.Source))

	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.ColorTokens] { return t.Color })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.TypeScale] { return t.Type })
	tokensObs := rx.CombineLatest2(colObs, typObs)

	full := rx.CombineLatest2(bcObs, tokensObs)
	return rx.Map(full, func(t rx.Tuple2[layout.Widget, rx.Tuple2[tokens.ColorTokens, tokens.TypeScale]]) layout.Widget {
		bcW := t.First
		style := docsMarkdownStyle(t.Second.First, t.Second.Second)
		return func(gtx layout.Context) layout.Dimensions {
			return drawDocsPage(gtx, bcW, doc, shaper, style)
		}
	})
}

// renderDocsPage is the static counterpart of docsPage used by goldens: a
// fresh top-scrolled Document laid out once with the given token sets.
func renderDocsPage(
	shaper *text.Shaper,
	def docsPageDef,
	colors tokens.ColorTokens,
	sp tokens.SpacingScale,
	ts tokens.TypeScale,
) layout.Widget {
	bcW := breadcrumb.Render(shaper, breadcrumb.Props{Items: docsBreadcrumb(def), Shaper: shaper}, colors, sp, ts)
	doc := markdown.NewDocument(markdown.Parse(def.Source))
	style := docsMarkdownStyle(colors, ts)
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
	inset := pllayout.Inset(docsOuterInsetDp)
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if bcW != nil {
					return bcW(gtx)
				}
				return layout.Dimensions{}
			}),
			layout.Rigid(pllayout.VSpacer(docsCardGapDp)),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return doc.Layout(gtx, shaper, style)
			}),
		)
	})
}

// paragraphWidget renders one body-text paragraph at BodyMedium font size.
// (Used by the About page, whose short prose stays hand-composed.)
func paragraphWidget(
	shaper *text.Shaper,
	textBody string,
	fg color.NRGBA,
	ts tokens.TypeScale,
) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		mColor := op.Record(gtx.Ops)
		paint.ColorOp{Color: fg}.Add(gtx.Ops)
		material := mColor.Stop()
		wl := widget.Label{Alignment: text.Start}
		return wl.Layout(gtx, shaper, font.Font{}, unit.Sp(ts.BodyMedium), textBody, material)
	}
}
