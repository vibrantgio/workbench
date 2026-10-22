package main

import (
	"image"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/shell"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
	"github.com/vibrantgio/theme/typeset"
)

// scanningModel is the window on a vault it is still reading, with nothing
// loaded to show yet.
func scanningModel() Model {
	return Model{Screen: screenVault, Vault: "/vaults/Second Brain", CurAnchor: -1, Scanning: true}
}

// emptyVaultModel is the window on a vault that holds no notes at all: the
// reading is over and there is nothing to open.
func emptyVaultModel() Model {
	return Model{Screen: screenVault, Vault: "/vaults/Second Brain", CurAnchor: -1, Index: treeIndex()}
}

// TestEmptyStateStandsInTheColumnsFullFrame holds the two states a vault
// column stands empty in against the platform's placement: the line is
// drawn by the shell pattern in the column's FULL frame — the main slot
// grown upward by the toolbar's share — so its centre stands on that
// frame's centre and not on the centre of the row below the toolbar.
//
// The glyph box is the painted glyphs' own and the line box is what the
// pattern centres, so the two centres differ by what the descenders reach
// below the box's middle: a pixel across and two down at this size. A line placed any other
// way in this column is out by tens of pixels at least.
func TestEmptyStateStandsInTheColumnsFullFrame(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	states := []struct {
		name  string
		model Model
	}{
		{"scanning", scanningModel()},
		{"empty", emptyVaultModel()},
	}
	for _, st := range states {
		for _, tc := range themeCases {
			t.Run(st.name+"-"+tc.name, func(t *testing.T) {
				fill := tc.colors.TextBackground
				img := golden.Capture(t, noteFrameSize,
					renderNotePage(shaper, st.model, tc.colors, tokens.Spacing, tokens.DefaultTypography, tokens.Comfortable))
				box := noteGlyphBox(img, fill)
				if box.Empty() {
					t.Fatal("the column is bare, want the line the empty state draws")
				}
				// The frame reaches a toolbar's share above the slot, so its
				// centre stands that much above the slot's own.
				bar := int(shell.ToolbarDepth)
				wantX, wantY := noteFrameW, noteFrameH-bar
				if got := box.Min.X + box.Max.X; abs(got-wantX) > 2 {
					t.Errorf("the line's glyphs centre at %.1f across, want the column's %.1f",
						float64(got)/2, float64(wantX)/2)
				}
				if got := box.Min.Y + box.Max.Y; abs(got-wantY) > 4 {
					t.Errorf("the line's glyphs centre at %.1f down, want the full frame's %.1f",
						float64(got)/2, float64(wantY)/2)
				}
			})
		}
	}
}

// TestEmptyStateStandsOnNoMeasure holds the other half of the ruling: the
// line is not content, so it does not begin where the column's text begins.
// Content stands on the measure's lead with everything else in the column;
// this stands in the middle of the column, which is a long way past it.
func TestEmptyStateStandsOnNoMeasure(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	tok := goldenTokens()
	tok.shaper = shaper
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(noteFrameSize),
		Ops:         new(op.Ops),
	}
	lead, _ := noteTextBox(gtx, tok, noteFrameW)

	img := golden.Capture(t, noteFrameSize,
		renderNotePage(shaper, emptyVaultModel(), tokens.PlatformLight, tokens.Spacing,
			tokens.DefaultTypography, tokens.Comfortable))
	box := noteGlyphBox(img, tokens.PlatformLight.TextBackground)
	if box.Empty() {
		t.Fatal("the column is bare, want the line the empty state draws")
	}
	if box.Min.X <= lead {
		t.Errorf("the line begins at x=%d, at or before the measure's lead %d; the empty state stands on no measure",
			box.Min.X, lead)
	}
}

// noteGlyphBox is the box the painted glyphs occupy in a rendering of the note
// column: every pixel the column's own fill does not account for.
func noteGlyphBox(img *image.RGBA, fill color.NRGBA) image.Rectangle {
	b := img.Bounds()
	box := image.Rectangle{}
	first := true
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if c := img.RGBAAt(x, y); c.R == fill.R && c.G == fill.G && c.B == fill.B {
				continue
			}
			if first {
				box, first = image.Rect(x, y, x+1, y+1), false
				continue
			}
			box = box.Union(image.Rect(x, y, x+1, y+1))
		}
	}
	return box
}

// TestAVaultThatCannotBeReadSaysSoInItsOwnWords drives a real scan at a
// path no vault stands at and holds both halves of the state: the
// column's line is the application's own sentence, read off
// emptyStateLine rather than off the picture, since no picture can be
// asked what its glyphs spell; and the system's own text, which the
// line does not carry, reaches the log with the vault's path.
func TestAVaultThatCannotBeReadSaysSoInItsOwnWords(t *testing.T) {
	vault := filepath.Join(t.TempDir(), "no-vault-here")

	logged := captureStderr(t)
	model := Model{Screen: screenVault, Vault: vault, Scanning: true, CurAnchor: -1}
	gen, stop := model.nextScan()
	model = foldScan(t, model, scanCmd(vault, "", gen, stop, func(err string) mvu.Message {
		return vaultScanned{vault: vault, gen: gen, err: err}
	}))
	written := logged()

	if model.ScanErr == "" {
		t.Fatal("scanning a path with no vault at it reported no error")
	}
	const want = "This vault can't be read."
	got := emptyStateLine(model)
	if got != want {
		t.Errorf("the column says %q, want %q", got, want)
	}
	if strings.Contains(got, model.ScanErr) {
		t.Errorf("the column says %q, which stands the system's text %q in the empty state", got, model.ScanErr)
	}
	if !strings.Contains(written, model.ScanErr) {
		t.Errorf("the log says %q, want the system's text %q in it", written, model.ScanErr)
	}
	if !strings.Contains(written, vault) {
		t.Errorf("the log says %q, want the vault's path %q in it", written, vault)
	}
}

// captureStderr diverts the stream the application writes its diagnostics
// to and answers the reader that restores it and returns what was written.
func captureStderr(t *testing.T) func() string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stderr
	os.Stderr = w
	read := make(chan string, 1)
	go func() {
		var b strings.Builder
		_, _ = io.Copy(&b, r)
		read <- b.String()
	}()
	return func() string {
		os.Stderr = saved
		w.Close()
		s := <-read
		r.Close()
		return s
	}
}

// paneEmptyModel is the window with all three of its pane sections empty
// at once: a note with no headings that nothing cites, and a query in the
// rail's find field that no note's name matches.
func paneEmptyModel() Model {
	m := goldenModel()
	m = cacheNote(m, noteFromSource("Sources.md", plainNoteSource))
	m.Current = "Sources.md"
	m.History = []HistEntry{{Path: "guide/Reading list.md", Anchor: -1}, {Path: "Sources.md", Anchor: -1}}
	m.Cursor = 1
	m.Filter = "zzzz"
	return m
}

// TestPaneSectionsCentreTheirEmptyState holds the three lines outside the
// content column against the platform's placement for a section: each
// centres in the frame of the section that has nothing — the rail's row
// region, the outline pane, the backlinks pane — at the size the rows it
// stands for are set at, which is not the column's own reading.
//
// Each line is read off the whole-window render inside its section's own
// rectangle, as the arrangement reported it, and held against what
// patterns/shell draws alone in a frame of that size: the two boxes are
// equal to the pixel, so the window hands the pattern the section's whole
// frame and nothing less.
func TestPaneSectionsCentreTheirEmptyState(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	m := paneEmptyModel()
	for _, tc := range themeCases {
		t.Run(tc.name, func(t *testing.T) {
			tok := themeTokens{col: tc.colors, typ: tokens.DefaultTypography,
				sp: tokens.Spacing, den: tokens.Comfortable, shaper: shaper}
			f := newFrameState(defaultArrangement())
			f.leading = func() unit.Dp { return goldenLeading }
			cur := &docCursor{}
			av := newAsideView(cur)
			sb := renderTree(shaper, m, tc.colors, tokens.Spacing, goldenRadius,
				tokens.DefaultTypography, tokens.Comfortable)
			main := renderNotePageInto(cur, shaper, m, tc.colors, tokens.Spacing,
				tokens.DefaultTypography, tokens.Comfortable, &pageFind{})
			as := func(gtx layout.Context) layout.Dimensions { return av.layout(gtx, m, tok) }
			w := func(gtx layout.Context) layout.Dimensions { return f.layout(gtx, m, tok, sb, as, main) }
			// Two frames, the second kept: the first is what the
			// arrangement and both panes measure themselves on.
			golden.Capture(t, windowFrameSize, windowScene(w, tc.colors))
			img := golden.Capture(t, windowFrameSize, windowScene(w, tc.colors))

			fill := chromeSurface(tc.colors)
			origin := f.geom.Inspector.Min
			sections := []struct {
				name   string
				line   string
				region image.Rectangle
				// edge is how far the frame's own drawing reaches into
				// the section's rectangle, which both readings skip: the
				// rail stands in a pane whose rim and rounded corners run
				// inside its row region, and the aside stands behind a
				// hairline seam one pixel wide.
				edge int
			}{
				{"the rail's row region", "No note by that name.", f.geom.Column.Rows, pane.RadiusDp},
				{"the outline pane", "This note has no headings.", av.geom.outline.Add(origin), 1},
				{"the backlinks pane", "No notes link here.", av.geom.backlinks.Add(origin), 1},
			}
			for _, s := range sections {
				t.Run(s.name, func(t *testing.T) {
					if s.region.Empty() {
						t.Fatalf("the arrangement laid out no %s", s.name)
					}
					got := paneGlyphBox(img, s.region.Inset(s.edge), fill)
					if got.Empty() {
						t.Fatalf("%s is bare, want the line the empty state draws", s.name)
					}

					// What the pattern draws by itself in a frame of this
					// size, at the size the section's rows are set at.
					ref := paneEmptyStateProbe(t, shaper, tc.colors, s.region.Size(), fill, s.line)
					refBox := paneGlyphBox(ref, ref.Bounds().Inset(s.edge), fill)
					if want := refBox.Add(s.region.Min); got != want {
						t.Errorf("%s draws its line at %v, want %v — the frame the line centres in is the section's own",
							s.name, got, want)
					}
					// And that frame is centred in, both ways.
					if c, want := refBox.Min.X+refBox.Max.X, s.region.Dx(); abs(c-want) > 2 {
						t.Errorf("%s centres its line at %.1f across, want the section's %.1f",
							s.name, float64(c)/2, float64(want)/2)
					}
					if c, want := refBox.Min.Y+refBox.Max.Y, s.region.Dy(); abs(c-want) > 4 {
						t.Errorf("%s centres its line at %.1f down, want the section's %.1f",
							s.name, float64(c)/2, float64(want)/2)
					}

					// The size is the rows' own and not the content
					// column's reading, which is twice it.
					rows := paneLineHeight(t, shaper, tc.colors, fill, tokens.DefaultTypography.BodyMedium, s.line)
					column := tokens.DefaultTypography.BodyLarge
					column.Size, column.LineHeight = shell.EmptyStateSizeSp, 0
					if h := got.Dy(); h != rows {
						t.Errorf("%s sets its line %d pixels tall, want the rows' own %d", s.name, h, rows)
					} else if h == paneLineHeight(t, shaper, tc.colors, fill, column, s.line) {
						t.Errorf("%s sets its line at the content column's reading, %d sp", s.name, shell.EmptyStateSizeSp)
					}

					// And it spends the platform's tertiary label over the
					// surface the section stands on.
					want := vgcolor.Flatten(tc.colors.TertiaryLabel, fill)
					if p := emptyStatePlateau(img, fill, s.region.Inset(s.edge)); p.R != want.R || p.G != want.G || p.B != want.B {
						t.Errorf("%s plateaus at #%02x%02x%02x, want TertiaryLabel over the section, #%02x%02x%02x",
							s.name, p.R, p.G, p.B, want.R, want.G, want.B)
					}
				})
			}
		})
	}
}

// paneEmptyStateProbe draws the pattern alone in a frame of the given size
// on the section's own fill, at the size a pane's rows are set at.
func paneEmptyStateProbe(t *testing.T, shaper *text.Shaper, c tokens.PlatformColors,
	size image.Point, fill color.NRGBA, line string) *image.RGBA {
	t.Helper()
	typ := tokens.DefaultTypography
	return golden.Capture(t, size, scene(func(gtx layout.Context) layout.Dimensions {
		return shell.EmptyState(gtx, shaper, c, typ, fill,
			shell.EmptyStateProps{Line: line, LineStyle: typ.BodyMedium})
	}, fill))
}

// paneLineHeight is how tall the painted glyphs of one line stand when it
// is set at a stated style in the colour the empty state spends, read off
// a drawing of nothing else and with the same reading.
func paneLineHeight(t *testing.T, shaper *text.Shaper, c tokens.PlatformColors,
	fill color.NRGBA, style tokens.TextStyle, line string) int {
	t.Helper()
	size := image.Pt(1200, 200)
	img := golden.Capture(t, size, scene(func(gtx layout.Context) layout.Dimensions {
		return typeset.Text(gtx, shaper, line, style,
			vgcolor.Flatten(c.TertiaryLabel, fill), 1)
	}, fill))
	return paneGlyphBox(img, img.Bounds(), fill).Dy()
}

// paneGlyphBox is the box the line's glyphs occupy inside one region of a
// render: every pixel standing at least a fifth of the way from the
// region's own fill to the colour the glyphs plateau at.
//
// It is a share of that plateau and not simply "not the fill" because the
// pane's rounded corners leave a ramp of anti-aliased pixels down at the
// region's foot, which a reading that counted every pixel the fill does
// not account for would take for a glyph and put the box out by the whole
// height of the section. A fifth stands clear of it.
func paneGlyphBox(img *image.RGBA, region image.Rectangle, fill color.NRGBA) image.Rectangle {
	least := paneFromFill(emptyStatePlateau(img, fill, region), fill) / 5
	box := image.Rectangle{}
	first := true
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			if c := img.RGBAAt(x, y); paneFromFill(c, fill) < least {
				continue
			}
			if first {
				box, first = image.Rect(x, y, x+1, y+1), false
				continue
			}
			box = box.Union(image.Rect(x, y, x+1, y+1))
		}
	}
	return box
}

// paneFromFill is how far a painted pixel stands from the fill it was
// painted over, summed across the three channels.
func paneFromFill(c color.RGBA, fill color.NRGBA) int {
	return abs(int(c.R)-int(fill.R)) + abs(int(c.G)-int(fill.G)) + abs(int(c.B)-int(fill.B))
}

// emptyStatePlateau is the painted colour furthest from the fill inside a
// region, which is the colour of the pixels the glyphs cover wholly.
func emptyStatePlateau(img *image.RGBA, fill color.NRGBA, region image.Rectangle) color.RGBA {
	var found color.RGBA
	far := -1
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			c := img.RGBAAt(x, y)
			if d := paneFromFill(c, fill); d > far {
				far, found = d, c
			}
		}
	}
	return found
}
