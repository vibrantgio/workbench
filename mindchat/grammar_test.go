package main

// Headless confirmations that this window is dressed by the surface grammar:
// which region of the window wears which storey of the elevation ladder. The
// transcript is what the window exists to show, so it is the content ground
// and fills at level 0 — the Background pin; the conversation list is chrome
// furniture and is therefore the window's FLOOR, a storey UNDER the paper;
// levels 2 and 3 stay with what appears and leaves.
//
// Since ADR-022 the ladder has one direction and no mirror: walking toward
// the viewer never gets darker, in either scheme. Read as a picture of this
// window that is one sentence rather than two — the furniture is the darkest
// region and the nearest surface the lightest, on paper and on slate alike.
//
// These assertions are the app's, not the token set's: they are what the
// window would fail if somebody filled a resting expanse of it at a storey
// the ladder keeps for a menu, or hung the furniture above the page again.

import (
	"image"
	"image/color"
	"math"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/components/picker"
	raster "github.com/vibrantgio/ivg/raster/gio"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/theme/tokens"
)

// schemeThemed builds the themed snapshot MessageRow needs for one scheme —
// testThemed's job, for a scheme other than the light default.
func schemeThemed(t *testing.T, c tokens.ColorTokens) themed {
	t.Helper()
	p := PaletteFrom(c)
	typ := tokens.DefaultTypography
	avatar, err := raster.Widget(ChatGPT, AvatarSize, AvatarSize, raster.WithColors(p.Icon))
	if err != nil {
		t.Fatalf("avatar widget: %v", err)
	}
	return themed{
		palette: p,
		avatar:  avatar,
		md:      messageMarkdownStyle(c, typ),
		typ:     typ,
		shaper:  typ.DeterministicShaper(),
	}
}

// schemes is the pair every rule below is stated once and checked twice
// against.
var schemes = []struct {
	name string
	c    tokens.ColorTokens
}{
	{"light", tokens.DefaultLight},
	{"dark", tokens.DefaultDark},
}

// luma is the Rec. 601 brightness of a fill, the axis "lighter" and "darker"
// are measured on below.
func luma(c color.NRGBA) float32 {
	return 0.299*float32(c.R) + 0.587*float32(c.G) + 0.114*float32(c.B)
}

// TestTranscriptRestsOnTheWindowGround pins the transcript to level 0 in both
// schemes: its resting fill is the Background pin, and it is emphatically not
// the neutral step the ladder reserves for a dialog — the fill this window
// used to spread under every answer, which made it darker in the middle than
// at its edges.
func TestTranscriptRestsOnTheWindowGround(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.c
			p := PaletteFrom(c)
			if p.Ground != c.SurfaceAt(tokens.Level0) {
				t.Errorf("transcript ground = %v, want the level-0 fill %v", p.Ground, c.SurfaceAt(tokens.Level0))
			}
			if p.Ground == c.SurfaceAt(tokens.Level2) {
				t.Errorf("transcript ground = %v, the level-2 fill; no resting expanse may sit that deep", p.Ground)
			}
			if p.Sidebar != c.SurfaceAt(tokens.LevelFloor) {
				t.Errorf("sidebar = %v, want the floor's fill %v — furniture is the window's floor, one storey UNDER the paper", p.Sidebar, c.SurfaceAt(tokens.LevelFloor))
			}
			if p.Sidebar == c.SurfaceAt(tokens.Level1) {
				t.Errorf("sidebar = %v, the level-1 fill; that storey is for what is RAISED on the paper, not for what the paper lies on", p.Sidebar)
			}
		})
	}
}

// TestLightnessClimbsTowardTheViewer is ADR-022's own check applied to this
// window's resting fills, in depth order rather than across the window's
// plane: the conversation list is the floor the window stands on, the
// transcript is the paper laid over it, the model chip is raised on that
// paper, and a dialog floats over the lot. Walking that order toward the
// reader, lightness may only increase — in the light scheme AND in the dark
// one, which is the whole of the linchpin and the reason this test needs no
// per-scheme clause where its predecessor needed two.
func TestLightnessClimbsTowardTheViewer(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.c
			p := PaletteFrom(c)
			toward := []struct {
				name string
				fill color.NRGBA
			}{
				{"the sidebar's floor", p.Sidebar},
				{"the transcript's paper", p.Ground},
				// The header picker's fill is the component's, not this
				// palette's: the claim is made against the control's own answer
				// for the storey it stands on, because the window's ladder is
				// what is on trial here and the control is still a rung of it.
				{"the header picker", picker.ToolbarFill(c, tokens.Level0, tokens.StateNormal)},
				{"a dialog's surface", c.SurfaceAt(tokens.Level2)},
			}
			for i := 1; i < len(toward); i++ {
				below, above := toward[i-1], toward[i]
				if step := luma(above.fill) - luma(below.fill); step <= 0 {
					t.Errorf("%s (%v) is not lighter than %s (%v); walking toward the viewer never gets darker",
						above.name, above.fill, below.name, below.fill)
				}
			}
			// The composition corollary, stated as the picture it is: the
			// furniture is this window's darkest region.
			for _, other := range toward[1:] {
				if luma(p.Sidebar) >= luma(other.fill) {
					t.Errorf("the sidebar (%v) is not darker than %s (%v); a window's furniture is its darkest region",
						p.Sidebar, other.name, other.fill)
				}
			}
		})
	}
}

// TestCodeInsetsStepUpFromTheTranscriptGround holds the app to the storey its
// markdown insets take. A raised inset walks from the surface it is lying on,
// and a message body lies on the transcript's paper — so a fenced block and
// an inline code chip sit exactly one storey off that paper, and since
// ADR-022's fence ruling "up" means LIGHTER in both schemes: a fence is a
// raised chip, never a well cut into the page. Inheriting FromTokens' grounds
// is how the app gets there; this is the assertion that makes it a decision.
//
// On paper the step is a whisper the fill alone cannot carry — what says
// where the fence is there is the rim FromTokens derives against it — so this
// test asks only for the direction, and the rim is markdown's own to prove.
func TestCodeInsetsStepUpFromTheTranscriptGround(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.c
			p := PaletteFrom(c)
			md := messageMarkdownStyle(c, tokens.DefaultTypography)

			if md.Paper != p.Ground {
				t.Errorf("Style.Paper = %v, but a message body is read on the transcript ground %v", md.Paper, p.Ground)
			}
			want := c.SurfaceAt(tokens.Level1)
			for _, f := range []struct {
				name string
				got  color.NRGBA
			}{
				{"CodeBackground", md.CodeBackground},
				{"CodeChip", md.CodeChip},
			} {
				if f.got != want {
					t.Errorf("Style.%s = %v, want one rung over the paper, %v", f.name, f.got, want)
				}
			}
			if step := luma(md.CodeBackground) - luma(p.Ground); step <= 0 {
				t.Errorf("fence ground %v is not lighter than the paper %v; a raised inset lightens in BOTH schemes", md.CodeBackground, p.Ground)
			}
		})
	}
}

// TestChipsWalkFromTheSurfaceTheySitOn checks the two chips this app draws
// against the same rule from two different grounds: the header picker sits on
// the transcript's level-0 paper and is raised a storey off it; the dialog's
// chips sit flush on the dialog's level-2 surface and reveal themselves with
// that surface's own walk. Each one's hover is its own ground's one-step
// walk, so neither is invisible at rest and neither moves the wrong way under
// the pointer.
//
// Two different axes are checked here and they answer differently, which is
// the point. A STOREY is the ladder and answers to the linchpin: the raised
// chip is lighter than its ground in both schemes. A STATE is feedback and
// still walks toward the ramp's 900 end: the hover darkens on paper and
// lightens on slate. That asymmetry is not the mirror ADR-022 abolished —
// the mirror was in the ladder, and the ladder is one direction now.
func TestChipsWalkFromTheSurfaceTheySitOn(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.c
			p := PaletteFrom(c)
			dark := isDarkColor(c.Background)

			for _, ch := range []struct {
				name           string
				ground         color.NRGBA
				rest, hovered  color.NRGBA
				restIsFlush    bool
				groundRungName string
			}{
				{"header picker", p.Ground,
					picker.ToolbarFill(c, tokens.Level0, tokens.StateNormal),
					picker.ToolbarFill(c, tokens.Level0, tokens.StateHover), false, "transcript"},
				{"dialog chip", c.SurfaceAt(tokens.Level2), p.ModalChip, p.ModalChipHovered, true, "dialog surface"},
			} {
				if ch.restIsFlush && ch.rest != ch.ground {
					t.Errorf("%s rests at %v, not flush on the %s %v", ch.name, ch.rest, ch.groundRungName, ch.ground)
				}
				if !ch.restIsFlush && ch.rest == ch.ground {
					t.Errorf("%s rests at %v, the same fill as the %s it sits on; nothing marks it as raised", ch.name, ch.rest, ch.groundRungName)
				}
				if !ch.restIsFlush && luma(ch.rest) <= luma(ch.ground) {
					t.Errorf("%s rests at %v, no lighter than the %s %v it is raised on; a storey nearer the viewer is lighter in both schemes",
						ch.name, ch.rest, ch.groundRungName, ch.ground)
				}
				if ch.hovered == ch.rest {
					t.Errorf("%s hovers to its own resting fill %v; the pointer moves nothing", ch.name, ch.rest)
				}
				step := luma(ch.hovered) - luma(ch.ground)
				if dark && step <= 0 {
					t.Errorf("%s hover %v is not lighter than its ground %v", ch.name, ch.hovered, ch.ground)
				}
				if !dark && step >= 0 {
					t.Errorf("%s hover %v is not darker than its ground %v", ch.name, ch.hovered, ch.ground)
				}
			}
		})
	}
}

// TestAssistantRowPaintsTheGroundItClaims renders a real assistant row
// through the app's own MessageRow over a sentinel no fill in this app
// resolves to. The row is expected to cover it edge to edge with the
// transcript ground: a row that painted nothing would leak the sentinel, and
// a row that painted the old dialog step would come back the wrong grey.
func TestAssistantRowPaintsTheGroundItClaims(t *testing.T) {
	sentinel := color.NRGBA{R: 255, G: 0, B: 255, A: 255}
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.c
			p := PaletteFrom(c)
			th := schemeThemed(t, c)
			size := image.Pt(560, 200)
			img := golden.Capture(t, size, func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, sentinel, clip.Rect{Max: gtx.Constraints.Max}.Op())
				rows := newDocCache().Rows([]Message{{Role: RoleAssistant, Content: "A settled answer."}})
				return MessageRow(gtx, th, rows[0])
			})

			// Two samples inside the row's own margins, well clear of ink:
			// the top-left corner and the right edge of the first line.
			for _, at := range []image.Point{{X: 2, Y: 2}, {X: size.X - 3, Y: 2}} {
				r, g, b, _ := img.At(at.X, at.Y).RGBA()
				got := color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0xff}
				if got == sentinel {
					t.Fatalf("pixel %v is the sentinel; the assistant row painted no ground", at)
				}
				if got != p.Ground {
					t.Errorf("pixel %v = %v, want the transcript ground %v", at, got, p.Ground)
				}
				if got == c.SurfaceAt(tokens.Level2) {
					t.Errorf("pixel %v = %v, the dialog rung under a resting transcript", at, got)
				}
			}
		})
	}
}

// relativeLuminance is the WCAG channel-linearised luminance, the axis a
// contrast ratio is computed on — a different axis from luma, which is the
// perceptual brightness the rung walks are ordered by.
func relativeLuminance(c color.NRGBA) float64 {
	lin := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// contrastRatio is the WCAG ratio between two opaque fills.
func contrastRatio(a, b color.NRGBA) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// TestChosenIsTintedAndTransientIsNeutral holds the conversation list to the
// division the grammar draws between the two: the item the window is showing
// wears the Primary ramp's tinted end, while hover — which is a state, not a
// choice — stays a walk along the neutral ramp. A list has to be able to say
// both things at once, and it cannot if both are neutral steps.
func TestChosenIsTintedAndTransientIsNeutral(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.c
			p := PaletteFrom(c)
			if want := c.Ramps.Primary.Step(300); p.RowSelected != want {
				t.Errorf("RowSelected = %v, want the Primary tint %v — the open conversation is a choice, not a state", p.RowSelected, want)
			}
			for step := 100; step <= 900; step += 100 {
				if n := c.Ramps.Neutral.Step(step); p.RowSelected == n {
					t.Errorf("RowSelected = %v, which is neutral %d; a neutral step cannot say which item is being read", p.RowSelected, step)
				}
			}
			// Hover is a neutral walk off the sidebar's own ground, at half
			// strength, so the pointer's mark is a tick rather than a rival
			// to the tint. The ground is the floor, since that is where the
			// furniture stands.
			walk := c.StateAt(tokens.LevelFloor, tokens.StateHover)
			if p.RowHovered.R != walk.R || p.RowHovered.G != walk.G || p.RowHovered.B != walk.B {
				t.Errorf("RowHovered = %v, want the neutral hover walk %v at reduced alpha", p.RowHovered, walk)
			}
			if p.RowHovered.A == 0xff {
				t.Errorf("RowHovered alpha = %d, want a fraction of the walk", p.RowHovered.A)
			}
		})
	}
}

// TestHoverSitsBetweenRestAndChosen checks the ordering a reader expects down
// the sidebar: the resting surface, then the hovered row, then the row being
// read. It is checked on paper, where the two fills' luminances are close
// enough that a full-strength walk would overshoot the tint and put the two in
// the wrong order — the measurement that set the hover fill's strength.
func TestHoverSitsBetweenRestAndChosen(t *testing.T) {
	c := tokens.DefaultLight
	p := PaletteFrom(c)
	hover := Blend(p.Sidebar, p.RowHovered, p.RowHovered.A)
	rest, chosen := luma(p.Sidebar), luma(p.RowSelected)
	if !(luma(hover) < rest && luma(hover) > chosen) {
		t.Errorf("hover %v (%.0f) does not sit between the resting surface %v (%.0f) and the chosen row %v (%.0f)",
			hover, luma(hover), p.Sidebar, rest, p.RowSelected, chosen)
	}
}

// TestAccentBarReadsOnTheChosenFill keeps the selected row's leading bar
// legible against the tint it is drawn on. The bar is what carries the
// selection at a glance where the tint and the surface are close in
// luminance — most of all in the dark scheme, where they very nearly match —
// so it is the one mark that may not wash out.
func TestAccentBarReadsOnTheChosenFill(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			p := PaletteFrom(tc.c)
			if got := contrastRatio(p.Accent, p.RowSelected); got < 3 {
				t.Errorf("accent bar %v on the chosen fill %v = %.2f:1, want at least 3:1", p.Accent, p.RowSelected, got)
			}
			if got := contrastRatio(p.RowActive, p.RowSelected); got < 4.5 {
				t.Errorf("row text %v on the chosen fill %v = %.2f:1, want at least 4.5:1", p.RowActive, p.RowSelected, got)
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
// pane dismissible without the window's own furniture shifting under the
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
		if got, ink := chatTitleText(name); ink != titleMuted || got != "Untitled chat" {
			t.Errorf("chatTitleText(%q) = %q/%v, want the muted placeholder", name, got, ink)
		}
	}
	got, ink := chatTitleText("reactive layouts.jsonl")
	if ink != titleNamed || got != "Reactive layouts" {
		t.Errorf("chatTitleText of a named chat = %q/%v, want %q named", got, ink, "Reactive layouts")
	}
}
