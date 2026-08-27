package main

// Headless confirmations that this window is dressed by the surface grammar:
// which region of the window wears which rung of the elevation ladder. The
// transcript is what the window exists to show, so it is the content ground
// and fills at level 0 — the Background pin; the conversation list is chrome
// furniture and stands one rung up; levels 2 and 3 stay with what appears and
// leaves. Walking out from the middle of the window, rung numbers may never
// decrease, which in the light scheme means the window is lightest at its
// centre and in the dark scheme darkest at its centre — one rule, both
// schemes, because the ramps are paired.
//
// These assertions are the app's, not the token set's: they are what the
// window would fail if somebody filled a resting expanse of it at a rung the
// ladder keeps for a menu.

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
	raster "github.com/vibrantgio/ivg/raster/gio"
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
			if p.Sidebar != c.SurfaceAt(tokens.Level1) {
				t.Errorf("sidebar = %v, want the level-1 fill %v — furniture stands exactly one rung up", p.Sidebar, c.SurfaceAt(tokens.Level1))
			}
		})
	}
}

// TestRungsNeverDecreaseWalkingOut is the grammar's own check applied to this
// window's three resting fills: transcript, then the conversation list, then
// the surface a dialog arrives on. Each step out is a step further from the
// ground, which the paired ramps render as darker in the light scheme and
// lighter in the dark one.
func TestRungsNeverDecreaseWalkingOut(t *testing.T) {
	for _, tc := range schemes {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.c
			p := PaletteFrom(c)
			out := []struct {
				name string
				fill color.NRGBA
			}{
				{"transcript", p.Ground},
				{"sidebar", p.Sidebar},
				{"dialog surface", c.SurfaceAt(tokens.Level2)},
			}
			dark := isDarkColor(c.Background)
			for i := 1; i < len(out); i++ {
				in, next := out[i-1], out[i]
				step := luma(next.fill) - luma(in.fill)
				if dark && step <= 0 {
					t.Errorf("%s (%v) is not lighter than %s (%v); on slate every rung out lightens", next.name, next.fill, in.name, in.fill)
				}
				if !dark && step >= 0 {
					t.Errorf("%s (%v) is not darker than %s (%v); on paper every rung out darkens", next.name, next.fill, in.name, in.fill)
				}
			}
		})
	}
}

// TestCodeInsetsStepUpFromTheTranscriptGround holds the app to the rung its
// markdown insets take. A raised inset walks from the surface it is lying on,
// and a message body lies on the transcript's paper — so a fenced block and
// an inline code chip sit exactly one neutral step off that paper, in the
// direction the scheme's ramp calls "up". Inheriting FromTokens' grounds is
// how the app gets there; this is the assertion that makes it a decision.
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
			step := luma(md.CodeBackground) - luma(p.Ground)
			if isDarkColor(c.Background) && step <= 0 {
				t.Errorf("fence ground %v is not lighter than the paper %v; on slate a raised inset lightens", md.CodeBackground, p.Ground)
			}
			if !isDarkColor(c.Background) && step >= 0 {
				t.Errorf("fence ground %v is not darker than the paper %v; on paper a raised inset darkens", md.CodeBackground, p.Ground)
			}
		})
	}
}

// TestChipsWalkFromTheSurfaceTheySitOn checks the two chips this app draws
// against the same rule from two different grounds: the header picker sits on
// the transcript's level-0 paper, which the ladder says to treat as a level-1
// surface because the Background pin is off-ramp; the dialog's chips sit on
// the dialog's level-2 surface. Each one's hover is its own ground's one-step
// walk, so neither is invisible at rest and neither moves the wrong way under
// the pointer.
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
				{"header chip", p.Ground, p.Chip, p.ChipHovered, false, "transcript"},
				{"dialog chip", c.SurfaceAt(tokens.Level2), p.ModalChip, p.ModalChipHovered, true, "dialog surface"},
			} {
				if ch.restIsFlush && ch.rest != ch.ground {
					t.Errorf("%s rests at %v, not flush on the %s %v", ch.name, ch.rest, ch.groundRungName, ch.ground)
				}
				if !ch.restIsFlush && ch.rest == ch.ground {
					t.Errorf("%s rests at %v, the same fill as the %s it sits on; nothing marks it as raised", ch.name, ch.rest, ch.groundRungName)
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
			// to the tint.
			walk := c.StateColor(tokens.RoleNeutral, tokens.Elevation.SurfaceStep(tokens.Level1), tokens.StateHover)
			if p.RowHovered.R != walk.R || p.RowHovered.G != walk.G || p.RowHovered.B != walk.B {
				t.Errorf("RowHovered = %v, want the neutral hover walk %v at reduced alpha", p.RowHovered, walk)
			}
			if p.RowHovered.A >= 0xff {
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

// TestWindowButtonsAreCentredInTheBandTheyStandIn states the platform rule the
// window's control placement is derived from: the three buttons are centred in
// whatever band a window gives them, and their leading inset equals their top
// one. The band here is the sidebar's brand row, so both numbers fall out of
// its height and neither is written down twice.
func TestWindowButtonsAreCentredInTheBandTheyStandIn(t *testing.T) {
	if got, want := WindowButtonInset, (BrandRowHeight-WindowButtonDiameter)/2; got != want {
		t.Errorf("WindowButtonInset = %v, want %v — the leading inset is the top inset", got, want)
	}
	if got, want := WindowButtonCenter, WindowButtonInset+WindowButtonDiameter/2; got != want {
		t.Errorf("WindowButtonCenter = %v, want %v — the centre line is the inset plus a radius", got, want)
	}
	if got, want := WindowButtonCenter, BrandRowHeight/2; got != want {
		t.Errorf("WindowButtonCenter = %v, want the band's own middle %v", got, want)
	}
	// The reference's unified-toolbar window measures 52dp deep with its
	// buttons 19dp in; this band is the same depth and must land on the same
	// number, which is the check that the derivation reproduces the platform
	// rather than merely being self-consistent.
	if BrandRowHeight == 52 && WindowButtonInset != 19 {
		t.Errorf("a 52dp band puts the buttons %v in, want the measured 19", WindowButtonInset)
	}
}

// TestBrandRowClearsTheWindowControls checks that the app's name starts after
// the window's controls where the window has them, and at the sidebar's own
// gutter where it does not — the collision AK2.1's review predicted the moment
// the app took over the title bar.
func TestBrandRowClearsTheWindowControls(t *testing.T) {
	saved := windowButtonsEnd
	defer func() { windowButtonsEnd = saved }()

	windowButtonsEnd = func() unit.Dp { return 0 }
	if got := brandLead(); got != SidebarGutter {
		t.Errorf("with no window controls the brand row leads at %v, want the sidebar gutter %v", got, SidebarGutter)
	}

	// The measured edge on this platform: the third circle's trailing side,
	// which is the leading inset plus a diameter plus two pitches.
	const measuredEnd = unit.Dp(19 + 14 + 2*23)
	windowButtonsEnd = func() unit.Dp { return measuredEnd }
	if got, want := brandLead(), measuredEnd+WindowButtonGap; got != want {
		t.Errorf("brandLead() = %v, want %v — the measurement carries no air of its own", got, want)
	}
	if brandLead() <= measuredEnd {
		t.Errorf("brandLead() = %v, which is on top of the controls ending at %v", brandLead(), measuredEnd)
	}
}
