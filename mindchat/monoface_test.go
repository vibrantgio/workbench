package main

// Headless confirmations for the F1.5 migration: the chat bodies' markdown
// style resolves the THEME's mono code face (the F1.4 technique), and the
// app palette derives from the ramps and pins rather than the deprecated
// MD3 aliases or stale literals.

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"gioui.org/font"
	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"golang.org/x/image/math/fixed"

	raster "github.com/vibrantgio/ivg/raster/gio"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/spectrum/tokens"
)

// TestChatCodeShapesInMonoFace is the F1.5 headless confirmation that chat
// bodies render code spans and fences in the mono face. It exercises the
// exact pieces ContentLayer composes for the default theme —
// messageMarkdownStyle over the theme-emitted tokens, and the theme's
// cached Typography shaper MessageRow lays the Document out with — in
// three layers (the F1.4 technique):
//  1. the style the app builds names the theme Code role's "Roboto Mono"
//     typeface at the Code role's size;
//  2. that typeface resolves in the app's shaper to a real distinct face —
//     the shaper holds no system fonts, so glyphs coming back proves
//     resolution; the advance differs from proportional Roboto; and the
//     glyph IDs differ (a Gio GlyphID packs the face index, so this is
//     face identity, not just metrics);
//  3. a real assistant chat bubble with a Go code fence, drawn through the
//     app's own MessageRow with the app's style and shaper, renders
//     different pixels than the same bubble with Mono forced back to
//     Roboto — the mono face visibly reaches the composed row.
func TestChatCodeShapesInMonoFace(t *testing.T) {
	typ := tokens.DefaultTypography
	style := messageMarkdownStyle(tokens.DefaultLight, typ)
	shaper := typ.DeterministicShaper()

	// 1. The style resolves the theme's Code role.
	if got, want := string(style.Mono), typ.Code.Typeface; got != want {
		t.Fatalf("Style.Mono = %q, want the theme Code role's %q", got, want)
	}
	if got, want := style.CodeSize, unit.Sp(typ.Code.Size); got != want {
		t.Errorf("Style.CodeSize = %v, want the Code role's %v", got, want)
	}

	// 2. The mono face resolves and is a distinct face from Roboto.
	shapeRun := func(f font.Font) (fixed.Int26_6, []text.GlyphID) {
		shaper.LayoutString(text.Parameters{
			Font:     f,
			PxPerEm:  fixed.I(16),
			MaxWidth: 100000,
		}, "wiiim... {mono[0] != prose}")
		var advance fixed.Int26_6
		var ids []text.GlyphID
		for g, ok := shaper.NextGlyph(); ok; g, ok = shaper.NextGlyph() {
			advance += g.Advance
			ids = append(ids, g.ID)
		}
		return advance, ids
	}
	monoAdvance, monoIDs := shapeRun(font.Font{Typeface: style.Mono})
	if len(monoIDs) == 0 {
		t.Fatalf("Mono typeface %q shaped no glyphs; the face did not resolve in the theme shaper", style.Mono)
	}
	robotoAdvance, robotoIDs := shapeRun(font.Font{Typeface: "Roboto"})
	if monoAdvance == robotoAdvance {
		t.Errorf("mono advance %v equals proportional Roboto's; %q likely fell back to Roboto", monoAdvance, style.Mono)
	}
	if glyphIDsEqual(monoIDs, robotoIDs) {
		t.Errorf("mono and Roboto shaped to identical glyph IDs; the two requests collapsed onto one face")
	}

	// 3. The mono face changes the rendered pixels of a real chat bubble.
	body := "Try `wiiim` inline:\n\n```go\nfunc main() { wiiim := \"....\" }\n```\n"
	propStyle := style
	propStyle.Mono = "Roboto"
	size := image.Pt(640, 360)
	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	a := captureRGBA(t, size, chatScene(testThemed(t, style), body, bg))
	b := captureRGBA(t, size, chatScene(testThemed(t, propStyle), body, bg))
	if a == nil || b == nil {
		return
	}
	if n := pixelDiff(a, b); n <= 0 {
		t.Errorf("chat bubble renders identically with Mono forced to Roboto (%d pixels differ); code is not shaping in the mono face", n)
	}
}

// TestPaletteDerivesFromRampsAndPins pins the palette's derivation to the
// ramps, pins and semantic fields of ADR-007 — not the deprecated MD3
// aliases and not stale literals — in both schemes, and confirms the chroma
// style selection follows the scheme's background.
func TestPaletteDerivesFromRampsAndPins(t *testing.T) {
	for _, tc := range []struct {
		name string
		c    tokens.ColorTokens
		dark bool
	}{
		{"light", tokens.DefaultLight, false},
		{"dark", tokens.DefaultDark, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.c
			p := PaletteFrom(c)
			hover := c.Ramps.Neutral.Step(300)
			hover.A = 128
			for _, f := range []struct {
				name      string
				got, want color.NRGBA
			}{
				{"Sidebar", p.Sidebar, c.Surface},
				{"Separator", p.Separator, c.Divider},
				{"Heading", p.Heading, c.Ramps.Neutral.Step(700)},
				{"Row", p.Row, c.Ramps.Neutral.Step(700)},
				{"RowActive", p.RowActive, c.Ramps.Neutral.Step(900)},
				{"RowSelected", p.RowSelected, c.Ramps.Neutral.Step(300)},
				{"RowHovered", p.RowHovered, hover},
				{"Accent", p.Accent, c.Primary},
				{"UserBubble", p.UserBubble, c.Primary},
				{"UserText", p.UserText, c.OnPrimary},
				{"BotBubble", p.BotBubble, c.Ramps.Neutral.Step(300)},
				{"BotText", p.BotText, c.Ramps.Neutral.Step(900)},
				{"Icon", p.Icon, c.Primary},
				{"Error", p.Error, c.Error},
			} {
				if f.got != f.want {
					t.Errorf("Palette.%s = %v, want %v (ramp/pin resolution)", f.name, f.got, f.want)
				}
			}
			if got := isDarkColor(c.Background); got != tc.dark {
				t.Errorf("isDarkColor(Background) = %v, want %v — the chroma style would follow the wrong appearance", got, tc.dark)
			}
			md := messageMarkdownStyle(c, tokens.DefaultTypography)
			if md.Highlight == nil {
				t.Error("messageMarkdownStyle left Highlight nil; chroma highlighting is the app's opt-in")
			}
		})
	}
}

// testThemed builds the themed snapshot MessageRow needs, the way
// ContentLayer builds it for one theme emission.
func testThemed(t *testing.T, md markdown.Style) themed {
	t.Helper()
	c := tokens.DefaultLight
	p := PaletteFrom(c)
	typ := tokens.DefaultTypography
	avatar, err := raster.Widget(ChatGPT, AvatarSize, AvatarSize, raster.WithColors(p.Icon))
	if err != nil {
		t.Fatalf("avatar widget: %v", err)
	}
	return themed{palette: p, avatar: avatar, md: md, typ: typ, shaper: typ.DeterministicShaper()}
}

// chatScene renders one assistant message row — parsed through the app's
// own docCache pipeline — over a solid background.
func chatScene(th themed, body string, bg color.NRGBA) layout.Widget {
	rows := newDocCache().Rows([]Message{{Role: RoleAssistant, Content: body}})
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return MessageRow(gtx, th, rows[0])
	}
}

// ---- headless capture harness (the sitedocs/F1.4 inlined helpers) ----

func captureRGBA(t *testing.T, size image.Point, draw layout.Widget) *image.RGBA {
	t.Helper()
	hw, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		t.Skipf("headless rendering not supported: %v", err)
		return nil
	}
	defer hw.Release()

	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}
	draw(gtx)
	if err := hw.Frame(&ops); err != nil {
		t.Fatalf("Frame: %v", err)
	}
	img := image.NewRGBA(image.Rectangle{Max: size})
	if err := hw.Screenshot(img); err != nil {
		t.Fatalf("Screenshot: %v", err)
	}
	return img
}

// pixelDiff counts the pixels that differ between a and b, which must have equal
// bounds. It panics if they do not.
//
// The panic replaces a returned -1. There is no pixel count to report for two
// images of different shapes, and -1 read as "no difference" to every `n > 0`
// test — which is how a golden whose size had moved compared as a pass, here
// and across the whole organization. A caller for which a size change is a
// real outcome rather than a defect — the stored-golden comparison, and only
// it — must compare Bounds itself before calling.
func pixelDiff(a, b *image.RGBA) int {
	if a.Bounds() != b.Bounds() {
		panic(fmt.Sprintf("pixelDiff: images must have equal bounds, got %v and %v",
			a.Bounds(), b.Bounds()))
	}
	bounds := a.Bounds()
	n := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			off := (y-bounds.Min.Y)*a.Stride + (x-bounds.Min.X)*4
			if a.Pix[off] != b.Pix[off] ||
				a.Pix[off+1] != b.Pix[off+1] ||
				a.Pix[off+2] != b.Pix[off+2] ||
				a.Pix[off+3] != b.Pix[off+3] {
				n++
			}
		}
	}
	return n
}

func glyphIDsEqual(a, b []text.GlyphID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
