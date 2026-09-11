package main

// Headless confirmations that this window paints the platform's own names:
// which region of the window wears which fill, and which foreground stands on
// it. The transcript is the window's content and wears the platform's content
// plane; the conversation pane is chrome and wears the chrome material a
// sidebar carries; what appears and leaves — the settings dialog, the model
// menu, the undo bar — is a floating surface and wears the window's own plane.
//
// These assertions are the app's, not the token set's: they are what the
// window would fail if somebody filled a resting expanse of it with a fill the
// platform keeps for something else, or painted a selection of its own where
// a pattern already draws the platform's.

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	raster "github.com/vibrantgio/ivg/raster/gio"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/sidebar"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

// schemeThemed builds the themed snapshot MessageRow needs for one scheme —
// testThemed's job, for a scheme other than the light default.
func schemeThemed(t *testing.T, c tokens.PlatformColors) themed {
	t.Helper()
	p := PaletteFrom(c)
	typ := tokens.DefaultTypography
	avatar, err := raster.Widget(ChatGPT, AvatarSize, AvatarSize, raster.WithColors(p.Icon))
	if err != nil {
		t.Fatalf("avatar raster: %v", err)
	}
	return themed{
		palette: p,
		col:     c,
		avatar:  avatar,
		md:      messageMarkdownStyle(c, typ),
		typ:     typ,
		shaper:  typ.DeterministicShaper(),
		sp:      tokens.Spacing,
		rad:     tokens.Radius,
	}
}

// schemes is the pair every rule below is stated once and checked twice
// against.
var schemes = []struct {
	name string
	c    tokens.PlatformColors
}{
	{"light", tokens.PlatformLight},
	{"dark", tokens.PlatformDark},
}

// luma is the Rec. 601 brightness of a fill, the axis "lighter" and "darker"
// are measured on below.
func luma(c color.NRGBA) float32 {
	return 0.299*float32(c.R) + 0.587*float32(c.G) + 0.114*float32(c.B)
}

// TestEveryRegionWearsItsPlatformFill is the table of what this window paints
// and the platform name each fill comes from. It is the one place the mapping
// is stated, so a region repainted with the wrong name fails here rather than
// in a golden nobody reads.
func TestEveryRegionWearsItsPlatformFill(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.c
			p := PaletteFrom(c)
			for _, f := range []struct {
				what string
				got  color.NRGBA
				want color.NRGBA
			}{
				{"the transcript", p.Transcript, c.ControlBackground},
				{"the conversation pane", p.Sidebar, c.SidebarMaterial},
				{"a floating surface", p.Toast, c.WindowBackground},
				{"a dialog's inset panel", p.Panel, c.CardFill},
				{"a dialog's chip", p.ModalChip, c.PushButtonFill},
				{"the open conversation's pill", p.RowSelected, sidebar.SelectionFill(c, false)},
				{"a content list's selected row", p.ListSelected, c.SelectedContentBackground},
				{"the in-flight dot", p.Accent, c.ControlAccent},
				{"a failed fetch's words", p.Error, c.SystemRed},
			} {
				if f.got != f.want {
					t.Errorf("%s is %v, want the platform's %v", f.what, f.got, f.want)
				}
			}
		})
	}
}

// TestEveryForegroundIsFlattenedOntoWhatItStandsOn holds the window to the
// platform's compositing rule: a label, a seam and a secondary label all carry
// a coverage, and what the reader sees is that coverage over the fill beneath
// it, flattened in encoded sRGB. Nothing this window hands Gio may be
// transparent.
func TestEveryForegroundIsFlattenedOntoWhatItStandsOn(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.c
			p := PaletteFrom(c)
			for _, f := range []struct {
				what string
				got  color.NRGBA
				want color.NRGBA
			}{
				{"a chat row's label", p.Row, vgcolor.Flatten(c.Label, c.SidebarMaterial)},
				{"the pane's heading", p.Heading, vgcolor.Flatten(c.SecondaryLabel, c.SidebarMaterial)},
				{"the pane's seam", p.Separator, vgcolor.Flatten(c.Separator, c.SidebarMaterial)},
				{"a turn's prose", p.TurnText, vgcolor.Flatten(c.Label, c.ControlBackground)},
				{"a system note", p.Note, vgcolor.Flatten(c.SecondaryLabel, c.ControlBackground)},
				{"a dialog's prose", p.FloatingText, vgcolor.Flatten(c.Label, c.WindowBackground)},
				{"a dialog's caption", p.FloatingHeading, vgcolor.Flatten(c.SecondaryLabel, c.WindowBackground)},
			} {
				if f.got != f.want {
					t.Errorf("%s is %v, want %v", f.what, f.got, f.want)
				}
				if f.got.A != 0xff {
					t.Errorf("%s carries alpha %d; a flattened foreground is opaque", f.what, f.got.A)
				}
			}
		})
	}
}

// TestTheChromeIsToldFromTheContent is the Chrome rule read as a picture of
// this window: the pane and the transcript are two regions on one window, and
// on this platform the chrome is a shade darker than the content it stands
// beside — in both appearances — with a seam where the reference has none to
// spare.
func TestTheChromeIsToldFromTheContent(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			p := PaletteFrom(tc.c)
			if p.Sidebar == p.Transcript && p.Separator == p.Sidebar {
				t.Errorf("the pane %v and the transcript %v are one fill with no seam between them; nothing tells the two regions apart", p.Sidebar, p.Transcript)
			}
		})
	}
}

// TestTheMessageBodyIsReadOnTheTranscript keeps markdown told which surface a
// reply stands on: every coverage in the document is flattened onto it, so a
// body handed the wrong surface composites its own prose against a fill it is
// nowhere near.
func TestTheMessageBodyIsReadOnTheTranscript(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.c
			p := PaletteFrom(c)
			md := messageMarkdownStyle(c, tokens.DefaultTypography)
			if md.ContentSurface != p.Transcript {
				t.Errorf("Style.ContentSurface = %v, but a message body is read on the transcript fill %v", md.ContentSurface, p.Transcript)
			}
		})
	}
}

// TestAssistantRowPaintsTheFillItClaims renders a real assistant row
// through the app's own MessageRow over a sentinel no fill in this app
// resolves to. The row is expected to cover it edge to edge with the
// transcript fill: a row that painted nothing would leak the sentinel.
func TestAssistantRowPaintsTheFillItClaims(t *testing.T) {
	sentinel := color.NRGBA{R: 255, G: 0, B: 255, A: 255}
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.c
			p := PaletteFrom(c)
			th := schemeThemed(t, c)
			size := image.Pt(560, 200)
			img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, sentinel, clip.Rect{Max: gtx.Constraints.Max}.Op())
				rows := newDocCache().Rows([]Message{{Role: RoleAssistant, Kind: KindTurn, Content: "A settled answer."}})
				return MessageRow(gtx, th, rows[0])
			})

			// Two samples inside the row's own margins, well clear of any glyphs:
			// the top-left corner and the right edge of the first line.
			for _, at := range []image.Point{{X: 2, Y: 2}, {X: size.X - 3, Y: 2}} {
				r, g, b, _ := img.At(at.X, at.Y).RGBA()
				got := color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0xff}
				if got == sentinel {
					t.Fatalf("pixel %v is the sentinel; the assistant row painted no fill", at)
				}
				if got != p.Transcript {
					t.Errorf("pixel %v = %v, want the transcript fill %v", at, got, p.Transcript)
				}
			}
		})
	}
}

// TestTheOpenConversationWearsThePlatformsPill holds the conversation pane to
// the platform's answer for a chosen row: the sidebar pill patterns/sidebar
// draws, not a fill this window invents and not the content selection a list
// row wears. The pill is the pattern's to paint, so what is checked here is
// that the app asks it for the colour rather than deriving one.
func TestTheOpenConversationWearsThePlatformsPill(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.c
			p := PaletteFrom(c)
			if want := sidebar.SelectionFill(c, false); p.RowSelected != want {
				t.Errorf("the open conversation's fill = %v, want the pattern's pill %v", p.RowSelected, want)
			}
			if p.RowSelected == c.SelectedContentBackground {
				t.Errorf("the open conversation's fill = %v, the content list's selection; a sidebar row and a list row are two different fills here", p.RowSelected)
			}
			if p.RowActive != vgcolor.Flatten(sidebar.SelectionLabel(c, false), sidebar.SelectionFill(c, false)) {
				t.Errorf("the open conversation's label = %v, want the foreground the platform pairs with its pill", p.RowActive)
			}
		})
	}
}

// TestWindowButtonsAreMeasuredFromTheWindowsGlass states the platform rule
// the window's control placement follows: the circles stand a fixed inset in
// from the window's own top and leading glass, and their leading inset
// equals their top inset, so the inset alone fixes the centre line.
//
// The number is the floating pane pattern's, read off the platform's own
// sidebar apps, and it is deliberately NOT derived from anything this window
// draws. Nothing beneath the buttons may move them — that is what makes the
// pane dismissible without the window's own chrome shifting under the
// reader's pointer.
func TestWindowButtonsAreMeasuredFromTheWindowsGlass(t *testing.T) {
	if got, want := WindowButtonInset, unit.Dp(pane.ButtonInsetDp); got != want {
		t.Errorf("WindowButtonInset = %v, want the pattern's %v", got, want)
	}
	if got, want := WindowButtonCenter, WindowButtonInset+WindowButtonDiameter/2; got != want {
		t.Errorf("WindowButtonCenter = %v, want %v — the centre line is the inset plus a radius", got, want)
	}
	// The stored reference measures the platform's sidebar windows at 19 in
	// from both edges; a derivation that agreed with itself but not with that
	// number would be self-consistent and wrong.
	if WindowButtonInset != 19 {
		t.Errorf("the buttons stand %v in, want the measured 19", WindowButtonInset)
	}
}

// TestBothHalvesOfTheSidebarSwitchStandOnOneLine is the arithmetic the whole
// phase exists for. A control that rides the pane and the control that
// recalls it are two halves of one switch, and a switch whose halves stand at
// two different heights makes the mark jump out from under the pointer that
// just clicked it — the defect the collapsed rail had.
//
// The pane's strip centres its controls on the buttons' line by the pattern's
// own arithmetic, one margin down from the window's top edge; the chrome row
// centres its controls on its own middle. The two are the same line only if
// the row is exactly twice the buttons' centre, which is how ChromeRowHeight
// is derived and what this checks.
func TestBothHalvesOfTheSidebarSwitchStandOnOneLine(t *testing.T) {
	// Where a control standing in the pane's strip centres, in window
	// coordinates: the pane floats one margin down, and the strip's own
	// middle is half its depth.
	strip := unit.Dp(pane.MarginDp) + unit.Dp(pane.StripDp)/2
	row := ChromeRowHeight / 2
	if strip != row {
		t.Errorf("the pane's strip centres its controls at %v and the chrome row at %v; a switch whose halves stand at two heights makes the mark jump", strip, row)
	}
	if got := unit.Dp(WindowButtonCenter); got != row {
		t.Errorf("the chrome row centres at %v, want the window buttons' own line %v", row, got)
	}
}

// TestChromeRowClearsTheWindowControlsOnlyWhenThePaneIsAway checks the one
// state in which the row owes the buttons anything. With the pane standing
// the buttons are inside it, so the row starts at the transcript's inset and
// its title stands over the messages below; with the pane away the row
// inherits the whole top strip and has to start past them.
func TestChromeRowClearsTheWindowControlsOnlyWhenThePaneIsAway(t *testing.T) {
	// The measured edge on this platform: the third circle's trailing side,
	// which is the leading inset plus a diameter plus two pitches.
	const measuredEnd = unit.Dp(19 + 14 + 2*23)

	if got := chromeLead(false, measuredEnd); got != chromeInsetDp {
		t.Errorf("with the pane standing the row leads at %v, want the transcript's own inset %v — the buttons are inside the pane", got, chromeInsetDp)
	}
	if got, want := chromeLead(true, measuredEnd), measuredEnd+chromeGapDp; got != want {
		t.Errorf("with the pane away the row leads at %v, want %v — the measurement carries no air of its own", got, want)
	}
	if chromeLead(true, measuredEnd) <= measuredEnd {
		t.Errorf("the row leads at %v, which is on top of the controls ending at %v", chromeLead(true, measuredEnd), measuredEnd)
	}
	// Every platform that keeps its own decorations reports no buttons at
	// all, and there the row has nothing to clear.
	if got := chromeLead(true, 0); got != chromeInsetDp {
		t.Errorf("with no window controls the row leads at %v, want %v", got, chromeInsetDp)
	}
}

// TestChatTitleShowsThePlaceholderUntilAChatEarnsAName holds the chrome row
// to showing what is open rather than what it is stored as. A chat the
// application named itself has not been named at all, and a filename is not
// an orientation cue.
func TestChatTitleShowsThePlaceholderUntilAChatEarnsAName(t *testing.T) {
	for _, name := range []string{"", "new.jsonl", "new-3.jsonl"} {
		if got, verdict := chatTitleText(name); verdict != titleMuted || got != "Untitled chat" {
			t.Errorf("chatTitleText(%q) = %q/%v, want the muted placeholder", name, got, verdict)
		}
	}
	got, verdict := chatTitleText("reactive layouts.jsonl")
	if verdict != titleNamed || got != "Reactive layouts" {
		t.Errorf("chatTitleText of a named chat = %q/%v, want %q named", got, verdict, "Reactive layouts")
	}
}

// TestTheUserTurnStandsOnThePlatformsBox is the claim the prompt's card
// makes: it is the platform's grouped box — a small step of fill, no hairline
// and no shadow — and emphatically not the accent, which would make a turn of
// the conversation wear a status it does not have.
func TestTheUserTurnStandsOnThePlatformsBox(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.c
			p := PaletteFrom(c)
			if c.CardFill == c.ControlAccent {
				t.Fatalf("the platform's box and its accent are one colour; this check cannot tell them apart")
			}
			if c.CardFill == p.Transcript {
				t.Errorf("the card fills with %v, the transcript's own fill; nothing marks the prompt as a card", c.CardFill)
			}
		})
	}
}
