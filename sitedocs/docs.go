// docs.go styles and lays out the Docs tab's one document: the
// application guide rendered by vibrantgio/markdown — type-scale
// headings, paragraph prose with links, chroma-highlighted code blocks,
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
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/mvu/desktop"
	"github.com/vibrantgio/patterns/shell"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// Page layout constants.
const (
	// docsOuterInsetDp is the blank the document keeps above its first
	// block and below its last, the padding it keeps on both sides of its
	// text, and the blank the empty state keeps from the pane edges
	// (S6 = 24 dp).
	docsOuterInsetDp = 24
	// docsMeasureDp caps the guide document's line length to a readable
	// column; the window is wider than a comfortable measure, and the
	// capped blocks are centred in the column.
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
// colour set and typography: the platform-themed defaults plus the app's
// two opt-ins — chroma highlighting matched to the appearance, and links
// opening in the system browser.
//
// The document is read on the content plane, which is what the tab shell
// fills its pane with and what this column therefore stands on. Every
// coverage the style carries is composited onto it there.
//
// Mono and CodeSize are re-resolved from the theme's Code role: FromTokens
// defaults them from the Typography it is handed, and setting them here keeps
// that explicit at the call site.
func docsMarkdownStyle(p tokens.PlatformColors, typ tokens.Typography) markdown.Style {
	st := markdown.FromTokens(p, typ, p.ControlBackground)
	st.Mono = font.Typeface(typ.Code.Typeface)
	st.CodeSize = unit.Sp(typ.Code.Size)
	if isDarkColor(p.ControlBackground) {
		st.Highlight = docsHighlightDark
	} else {
		st.Highlight = docsHighlightLight
	}
	st.Text.OnLinkClick = func(_ layout.Context, url string) { openURL(url) }
	return st
}

// isDarkColor reports whether c reads as a dark fill (Rec. 601 luma
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
// column: the column's whole frame is the document's scrolling viewport,
// and the document's blocks are capped at the reading measure and centred
// in it by markdown's own Style.Measure. The region that scrolls is the
// whole column, so the bar is at the column's trailing edge, which is the
// pane's edge, as every other tab's bar is. The document is long-lived —
// scroll position and interaction state belong to it, so ScrollToBlock
// from an outline row moves the same reader rather than rebuilding the
// page.
//
// The blank above the first block and below the last is the document's
// own StartSpace and EndSpace, so it scrolls with the content instead of
// cutting a still strip out of the viewport. Gutter is the padding: the
// same blank is kept on both sides of the text when the column is narrower
// than the measure and the padding on both sides, and the measure is capped
// at the width that remains.
//
// The bar is passed in rather than built here because it is the window's
// bar, spent against the setting read once where the window was composed;
// see columnBar. Its anchor is what [list.AnchorFor] returns, as every other tab's
// bar's is: a standing bar takes a lane of its own and the text stops short
// of it, and a fading bar lies over the column's trailing edge and takes no
// lane. It is the document's own scrollbar entry point that draws it,
// because the viewport being scrolled is the document's own list.
func drawGuideDoc(
	gtx layout.Context,
	doc *markdown.Document,
	shaper *text.Shaper,
	style markdown.Style,
	bar scrollbar.Style,
) layout.Dimensions {
	style.Measure = docsMeasureDp
	style.Gutter = docsOuterInsetDp
	style.StartSpace = docsOuterInsetDp
	style.EndSpace = docsOuterInsetDp
	return doc.LayoutScrollbar(gtx, shaper, style, bar, list.AnchorFor(bar))
}

// drawGuideUnavailable draws the Docs column with no guide in it: this
// app's own sentence saying the guide cannot be loaded, and under it the
// location it was looked for at. The system's own text is not here — it
// went to the log where the read failed.
//
// It is handed the column's whole frame, which is what the group centres
// in, and it is held off the pane's edges by docsOuterInsetDp so a long
// location does not run into them; that inset is even on
// every side, so the centre it centres on is the column's own. The
// document's reading measure is not applied: an empty state is not
// content and stands on no measure.
//
// The column's frame is the one the document's rows would occupy, so where
// the reader's setting makes the column's bar stand, its lane is taken off
// the trailing side before the inset is: bar is the [columnBar] the
// document is drawn with, and its lane is read off it.
func drawGuideUnavailable(
	gtx layout.Context,
	shaper *text.Shaper,
	c tokens.PlatformColors,
	typ tokens.Typography,
	bar scrollbar.Style,
	fail *guideFailure,
) layout.Dimensions {
	frame := gtx.Constraints.Max
	gtx.Constraints = layout.Exact(shell.EmptyStateFrame(gtx, frame, bar))
	complayout.Inset(docsOuterInsetDp).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return shell.EmptyState(gtx, shaper, c, typ, shellFill(c), shell.EmptyStateProps{
			Line:        guideUnavailableLine,
			Description: fail.Path,
		})
	})
	return layout.Dimensions{Size: frame}
}

// drawDocsUnavailable draws the Docs page when there is no guide: the
// outline column is kept at its width with its own line, and the content
// column beside it draws the guide's line and location in the rest.
func drawDocsUnavailable(gtx layout.Context, tok themeTokens, bar scrollbar.Style, fail *guideFailure) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return drawOutlineUnavailable(gtx, tok)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return drawGuideUnavailable(gtx, tok.shaper, tok.col, tok.typ, bar, fail)
		}),
	)
}

// guideUnavailableObservable is the Docs page when there is no guide to
// render: both columns' empty states, restyled per theme change. The
// failure is read once at startup and does not change while the window
// is open, so nothing but the theme drives this stream.
func guideUnavailableObservable(th rx.Observable[theme.Theme], fail *guideFailure, bars desktop.ScrollBarSetting) rx.Observable[layout.Widget] {
	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] { return t.Platform })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Typography] { return t.Typography })
	return rx.Map(rx.CombineLatest2(colObs, typObs), func(t rx.Tuple2[tokens.PlatformColors, tokens.Typography]) layout.Widget {
		c, typ := t.First, t.Second
		tok := themeTokens{col: c, typ: typ, shaper: typ.Shaper()}
		bar := columnBar(c, bars)
		return func(gtx layout.Context) layout.Dimensions {
			return drawDocsUnavailable(gtx, tok, bar, fail)
		}
	})
}

// guideDocObservable is the live main-column stream for the one guide
// document: the same Document on every emission, restyled per theme
// change. The shaper is the theme's cached Typography shaper.
func guideDocObservable(
	th rx.Observable[theme.Theme],
	doc *markdown.Document,
	bars desktop.ScrollBarSetting,
) rx.Observable[layout.Widget] {
	colObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] { return t.Platform })
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Typography] { return t.Typography })
	tokensObs := rx.CombineLatest2(colObs, typObs)
	return rx.Map(tokensObs, func(t rx.Tuple2[tokens.PlatformColors, tokens.Typography]) layout.Widget {
		typ := t.Second
		style := docsMarkdownStyle(t.First, typ)
		// The bar inside an overflowing fence follows the reader's "Show
		// scroll bars" setting, as every column in this window does.
		style.CodeScrollbar = scrollBarStyle(style.CodeScrollbar, bars)
		bar := columnBar(t.First, bars)
		shaper := typ.Shaper()
		return func(gtx layout.Context) layout.Dimensions {
			return drawGuideDoc(gtx, doc, shaper, style, bar)
		}
	})
}
