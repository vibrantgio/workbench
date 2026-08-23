// docs.go styles and lays out the Docs tab's one document: the
// application guide rendered by vibrantgio/markdown — type-scale
// headings, richtext prose with links, chroma-highlighted code blocks,
// lists, blockquotes, and tables. The live entry point is
// guideDocObservable; drawGuideDoc is the shared frame path the static
// review/golden renders reuse.

package main

import (
	"image/color"
	"os/exec"
	"runtime"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// Page layout constants.
const (
	// docsOuterInsetDp is the blank the document keeps from the panel
	// edges (S6 = 24 dp).
	docsOuterInsetDp = 24
	// docsMeasureDp caps the guide document's line length to a readable
	// column; the window is wider than a comfortable measure.
	docsMeasureDp = 720
)

// Chroma styles for the two appearance modes; built once, shared by every
// emission. FromTokens leaves Highlight nil, so assigning these is the
// app's opt-in to syntax highlighting.
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
// destinations are ignored — the guide only carries web links.
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

// drawGuideDoc lays out the one guide document as the Docs tab's main
// column: the shared outer inset, then the document as its own scrolling
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

// guideDocObservable is the live main-column stream for the one guide
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
