// g0a3_prefs_test.go covers the two halves of the settings pattern feeds is
// the reference implementation of: the ACCELERATOR the app chrome binds
// (⌘,/Ctrl-,, via key.ModShortcut) and the PANEL it opens — a cadence/modal
// with a nil Props.Decision, so its close X, Escape and backdrop dismissal all
// come from the intent rather than from flags.
//
// The reducer half is asserted directly; the accelerator is driven through a
// real gioui input.Router so the modifier requirement is proven rather than
// read; the panel is pinned as a golden and driven live over the articles
// table it edits, and driven again through the real composed shell in
// TestPreferencesPanelInShellLive — the test that was impossible until G0B.1
// lifted the eight-subscriber ceiling on cadence/toast's Subject.
package main

import (
	"image"
	"image/color"
	"testing"

	gioinput "gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/modal"
	"github.com/vibrantgio/cadence/table"
	"github.com/vibrantgio/prism/button"
	"github.com/vibrantgio/prism/golden"
	"github.com/vibrantgio/spectrum/theme"
	"github.com/vibrantgio/spectrum/tokens"
)

// --- the accelerator (arrival) -------------------------------------------

// driveShortcut lays shortcutArea out through a real input.Router, queues the
// given key events between frames, and reports how many times the callback
// fired.
func driveShortcut(t *testing.T, events ...key.Event) int {
	t.Helper()
	fired := 0
	area := shortcutArea(prefsAccelerator, func(layout.Context) { fired++ })

	r := new(gioinput.Router)
	frame := func() {
		ops := new(op.Ops)
		gtx := layout.Context{
			Constraints: layout.Exact(image.Pt(400, 300)),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Ops:         ops,
			Source:      r.Source(),
		}
		area(gtx)
		r.Frame(ops)
	}
	// Frame 1 registers the key area; the events are then queued and drained
	// by frame 2.
	frame()
	for _, e := range events {
		r.Queue(e)
	}
	frame()
	return fired
}

// TestPrefsAcceleratorFiresOnShortcutModifier proves the binding is the
// platform accelerator and not merely the comma key: key.ModShortcut resolves
// to Cmd on darwin and Ctrl elsewhere, so the SAME event literal is the right
// chord on every platform feeds runs on.
func TestPrefsAcceleratorFiresOnShortcutModifier(t *testing.T) {
	if n := driveShortcut(t, key.Event{
		Name:      prefsAccelerator,
		Modifiers: key.ModShortcut,
		State:     key.Press,
	}); n != 1 {
		t.Errorf("shortcut+, fired %d times, want 1", n)
	}
}

// TestPrefsAcceleratorIgnoresBareComma is the other half of the same claim: a
// comma typed with no modifier is text, not an accelerator. Without
// Filter.Required the panel would open every time someone typed a comma into
// the article filter.
func TestPrefsAcceleratorIgnoresBareComma(t *testing.T) {
	if n := driveShortcut(t, key.Event{
		Name:  prefsAccelerator,
		State: key.Press,
	}); n != 0 {
		t.Errorf("bare , fired %d times, want 0", n)
	}
}

// TestPrefsAcceleratorIgnoresRelease guards against the panel toggling twice
// per keypress: only the Press edge counts.
func TestPrefsAcceleratorIgnoresRelease(t *testing.T) {
	if n := driveShortcut(t, key.Event{
		Name:      prefsAccelerator,
		Modifiers: key.ModShortcut,
		State:     key.Release,
	}); n != 0 {
		t.Errorf("release fired %d times, want 0", n)
	}
}

// --- the reducer (what the panel edits) ----------------------------------

func TestUpdateOpenClosePreferences(t *testing.T) {
	m := initialModel()
	if m.prefsOpen {
		t.Fatal("seed model has the Preferences panel open")
	}
	m, _ = Update(m, OpenPreferences{})
	if !m.prefsOpen {
		t.Fatal("OpenPreferences did not open the panel")
	}
	// Every one of the panel's three exits — the ghost X, Escape and a
	// backdrop press — routes through modal.Props.OnClose to this one message.
	m, _ = Update(m, ClosePreferences{})
	if m.prefsOpen {
		t.Fatal("ClosePreferences did not close the panel")
	}
}

// TestUpdateSetRowsPerPageAppliesLiveAndResetsPage pins the behaviour that
// makes this surface a panel rather than a decision: the preference lands on
// the model immediately, with no Save and with the panel still open.
func TestUpdateSetRowsPerPageAppliesLiveAndResetsPage(t *testing.T) {
	m := initialModel()
	if m.rowsPerPage != defaultRowsPerPage {
		t.Fatalf("seed rowsPerPage = %d, want %d", m.rowsPerPage, defaultRowsPerPage)
	}
	m, _ = Update(m, OpenPreferences{})
	m, _ = Update(m, SetPage{Page: 3})
	m, _ = Update(m, SetRowsPerPage{Rows: 25})
	if m.rowsPerPage != 25 {
		t.Errorf("rowsPerPage = %d, want 25", m.rowsPerPage)
	}
	if m.currentPage != 1 {
		t.Errorf("currentPage = %d, want 1 (a bigger page size shrinks the page count)", m.currentPage)
	}
	if !m.prefsOpen {
		t.Error("changing a preference closed the panel; a panel's preferences apply under it")
	}
}

func TestUpdateSetRowsPerPageRejectsNonPositive(t *testing.T) {
	m := initialModel()
	for _, n := range []int{0, -5} {
		got, _ := Update(m, SetRowsPerPage{Rows: n})
		if got.rowsPerPage != defaultRowsPerPage {
			t.Errorf("SetRowsPerPage{%d} set rowsPerPage = %d; a non-positive page size would divide by zero downstream", n, got.rowsPerPage)
		}
	}
}

func TestUpdateToggleUnreadOnlyResetsPage(t *testing.T) {
	m := initialModel()
	m, _ = Update(m, SetPage{Page: 2})
	m, _ = Update(m, ToggleUnreadOnly{})
	if !m.unreadOnly {
		t.Error("ToggleUnreadOnly did not turn the preference on")
	}
	if m.currentPage != 1 {
		t.Errorf("currentPage = %d, want 1", m.currentPage)
	}
	m, _ = Update(m, ToggleUnreadOnly{})
	if m.unreadOnly {
		t.Error("ToggleUnreadOnly did not turn the preference back off")
	}
}

func TestUnreadOnlyArticlesFilters(t *testing.T) {
	all := hardCodedArticles()
	if got := unreadOnlyArticles(all, false); len(got) != len(all) {
		t.Fatalf("unreadOnly off dropped articles: %d of %d", len(got), len(all))
	}
	got := unreadOnlyArticles(all, true)
	if len(got) == 0 || len(got) == len(all) {
		t.Fatalf("unreadOnly on kept %d of %d articles; the fixture needs a mix to make this test meaningful", len(got), len(all))
	}
	for _, a := range got {
		if !a.Unread {
			t.Fatalf("read article %q survived the unread-only filter", a.Title)
		}
	}
}

// --- the panel (dismissal, and the emphasis axis at work) -----------------

// staticPreferencesBody assembles the panel body from the STATIC Render paths
// of the same prism/button calls preferencesPanel composes, at the default
// preferences: 10 rows per page (tonal) with 5 and 25 quiet beside it, and
// unread-only off (ghost). Sharp radii keep the golden deterministic.
func staticPreferencesBody(shaper *text.Shaper, colors tokens.ColorTokens) layout.Widget {
	render := func(label string, emph button.Emphasis) layout.Widget {
		return button.Render(shaper, label, colors, tokens.Spacing, modalSharpRadius,
			tokens.DefaultTypography.LabelLarge, tokens.Comfortable,
			button.RenderState{Emphasis: emph})
	}
	tok := themeTokens{col: colors, typ: tokens.DefaultTypography, shaper: shaper}
	return func(gtx layout.Context) layout.Dimensions {
		w := gtx.Constraints.Max.X
		rowH := gtx.Dp(unit.Dp(prefsRowHDp))
		sizes := []layout.Widget{
			render("5", button.Ghost),
			render("10", button.Tonal),
			render("25", button.Ghost),
		}
		y := prefsRow(gtx, tok, 0, w, rowH, "Rows per page", sizes, gtx.Dp(unit.Dp(prefsSizeBtnW)))
		y += gtx.Dp(unit.Dp(prefsRowGapDp))
		y = prefsRow(gtx, tok, y, w, rowH, "Unread only",
			[]layout.Widget{render("Off", button.Ghost)}, gtx.Dp(unit.Dp(prefsToggleBtnW)))
		return layout.Dimensions{Size: image.Pt(w, y)}
	}
}

// TestPreferencesPanelGolden records the panel intent: a ghost close X in the
// header (no footer, because there is nothing to confirm) over preference
// controls in the tonal and ghost registers. Its counterpart is
// TestAddFeedModalGolden next door.
func TestPreferencesPanelGolden(t *testing.T) {
	shaper := tokens.DefaultTypography.DeterministicShaper()
	cases := []struct {
		name   string
		colors tokens.ColorTokens
		bg     color.NRGBA
	}{
		{"preferences-panel-light", tokens.DefaultLight, color.NRGBA{R: 240, G: 240, B: 240, A: 255}},
		{"preferences-panel-dark", tokens.DefaultDark, color.NRGBA{R: 20, G: 20, B: 20, A: 255}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			props := modal.Props{
				Title:  "Preferences",
				Body:   staticPreferencesBody(shaper, tc.colors),
				Shaper: shaper,
			}
			if props.Intent() != modal.IntentPanel {
				t.Fatalf("Preferences modal intent = %v, want %v", props.Intent(), modal.IntentPanel)
			}
			m := modal.Render(shaper, props, true, tc.colors, tokens.Spacing, modalSharpRadius,
				tokens.DefaultTypography.TitleMedium, tokens.Comfortable)
			golden.Render(t, tc.name, image.Pt(modalCanvasW, modalCanvasH), scene(m, tc.bg))
		})
	}
}

// Canvas for the live panel-over-articles composition below. At 1000 dp the
// modal surface is its 560 dp maximum, centred, so it spans x∈[220,780) and
// leaves a clean strip of table either side.
const (
	prefsCanvasW = 1000
	prefsCanvasH = 700
)

// prefsArticlesRegion is the strip of articles table LEFT of the open panel's
// surface. Excluding the surface is the whole point of the region: the panel's
// own buttons change emphasis when a preference changes, so a sample that
// overlapped it would pass without the table having moved. Here the only thing
// that can differ is the table — go-blog holds 14 articles, so ten to a page
// fills these rows and five to a page empties them.
var prefsArticlesRegion = image.Rect(20, 60, 216, prefsCanvasH-20)

// prefsScrimRegion samples the middle of the canvas, where an open panel
// paints its scrim and surface over the table.
var prefsScrimRegion = image.Rect(prefsCanvasW/2-260, prefsCanvasH/2-120, prefsCanvasW/2+260, prefsCanvasH/2+120)

// TestPreferencesPanelOverArticlesLive drives the REAL panel — the live
// modal.Modal path, the live prism/button emphasis, the live articles
// pipeline — over the surface it edits, and asserts the three claims that
// make it a panel: OpenPreferences paints the scrim, a preference changed
// while it is open repaints the TABLE underneath with no Save, and closing it
// puts the canvas back.
//
// It composes preferencesPanel over articlesMain rather than subscribing
// feedsShellLayer, which keeps the canvas tight enough for the two regions
// above to mean what they say. When it was written that was not a choice:
// cadence/toast's Notify Subject was process-global, prism/coordination.Subject
// then capped it at eight concurrent subscribers, every feedsShellLayer
// subscription took one via toast.Stack — and rx never returned a slot on
// Unsubscribe, so the eight were spent for the life of the binary. This
// package stood at exactly eight, and a ninth shell subscription anywhere in
// it made the LAST such test in the binary fail with "out of subject
// subscriptions", which looks for all the world like a wrong AutoConnect
// count. G0B.1 made Unsubscribe release the slot, so the shell-level half of
// this pattern got its own test below, TestPreferencesPanelInShellLive — the
// ninth. G0C.3 removed the ceiling's cause entirely: the toast queue is model
// state now, toast.Stack subscribes no Subject at all, and a tenth shell
// subscription costs nothing but time.
func TestPreferencesPanelOverArticlesLive(t *testing.T) {
	send, modelObs := rx.Subject[Model](0, 1, 256)
	th := rx.Of(theme.Default())

	articlesObs := articlesMain(th,
		rx.Map(modelObs, func(m Model) FeedID { return m.selectedFeed }),
		rx.Map(modelObs, func(m Model) int { return m.currentPage }),
		rx.Map(modelObs, func(m Model) table.Sort { return m.sort }),
		rx.Map(modelObs, func(m Model) int { return m.rowsPerPage }),
		rx.Map(modelObs, func(m Model) bool { return m.unreadOnly }),
	)
	prefsObs := preferencesPanel(th,
		rx.Map(modelObs, func(m Model) bool { return m.prefsOpen }),
		rx.Map(modelObs, func(m Model) int { return m.rowsPerPage }),
		rx.Map(modelObs, func(m Model) bool { return m.unreadOnly }),
	)
	composed := rx.Map(rx.CombineLatest2(articlesObs, prefsObs),
		func(n rx.Tuple2[layout.Widget, layout.Widget]) layout.Widget {
			articlesW, panelW := n.First, n.Second
			return func(gtx layout.Context) layout.Dimensions {
				dims := articlesW(gtx)
				if panelW != nil {
					panelW(gtx)
				}
				return dims
			}
		},
	)

	emissions := make(chan layout.Widget, 64)
	sub := composed.Subscribe(rx.GoroutineContext(), func(w layout.Widget, _ error, done bool) {
		if !done && w != nil {
			select {
			case emissions <- w:
			default:
			}
		}
	})
	defer sub.Unsubscribe()

	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	size := image.Pt(prefsCanvasW, prefsCanvasH)
	snap := func(what string) *image.RGBA {
		return golden.Capture(t, size, scene(awaitStableWidget(t, emissions, what), bg))
	}

	m := initialModel()
	send.Next(m)
	closed := snap("initial model")

	m, _ = Update(m, OpenPreferences{})
	send.Next(m)
	open := snap("OpenPreferences")
	if n := regionDiff(closed, open, prefsScrimRegion); n <= 0 {
		t.Errorf("canvas unchanged after OpenPreferences (diff=%d in the scrim region); the panel did not open", n)
	}

	// The preference applies live: the table behind the panel repaginates
	// with no Save and with the panel still on screen.
	m, _ = Update(m, SetRowsPerPage{Rows: 5})
	send.Next(m)
	resized := snap("SetRowsPerPage{5}")
	if !m.prefsOpen {
		t.Fatal("SetRowsPerPage closed the panel")
	}
	if n := regionDiff(open, resized, prefsArticlesRegion); n <= 0 {
		t.Errorf("table unchanged beside the open panel after SetRowsPerPage{5} (diff=%d); the preference did not apply live", n)
	}

	// And the panel leaves: OnClose is what the ghost X, Escape and a
	// backdrop press all reach.
	m, _ = Update(m, ClosePreferences{})
	send.Next(m)
	dismissed := snap("ClosePreferences")
	if n := regionDiff(open, dismissed, prefsScrimRegion); n <= 0 {
		t.Errorf("scrim still painted after ClosePreferences (diff=%d); the panel did not dismiss", n)
	}
}

// shellPrefsScrimRegion samples the middle of the FULL shell canvas, where an
// open preferences panel paints its scrim and surface over the split pane.
// The shell's sidebar is 192 dp and its navbar 64 px, so a centred sample of
// the 1200×800 canvas lies wholly inside the region the panel covers.
var shellPrefsScrimRegion = image.Rect(shellCanvasW/2-260, shellCanvasH/2-120, shellCanvasW/2+260, shellCanvasH/2+120)

// TestPreferencesPanelInShellLive is the ninth feedsShellLayer subscription in
// this binary, and until G0B.1 it could not exist: the eight-subscriber
// ceiling on cadence/toast's process-global Subject was already spent, so
// adding this test broke a different, later test with an error that named
// neither this test nor the Subject. It is kept as much for that as for what
// it asserts — if the ceiling ever comes back, this is what says so, in the
// package that paid for it the first time.
//
// What it asserts is the half TestPreferencesPanelOverArticlesLive cannot:
// that the panel reaches the canvas through the REAL composed shell — navbar,
// sidebar, split pane, toast stack and all — rather than through a two-widget
// composition built for the test. Opening it must change the middle of the
// shell; closing it must put the shell back exactly, because every widget
// here is a pure function of model and theme.
func TestPreferencesPanelInShellLive(t *testing.T) {
	send, modelObs := rx.Subject[Model](0, 1, 256)
	layer := feedsShellLayer(rx.Of(theme.Default()), modelObs)

	emissions := make(chan layout.Widget, 64)
	sub := layer.Subscribe(rx.GoroutineContext(), func(w layout.Widget, err error, done bool) {
		if err != nil {
			t.Errorf("shell layer errored: %v", err)
			return
		}
		if !done && w != nil {
			select {
			case emissions <- w:
			default:
			}
		}
	})
	defer sub.Unsubscribe()

	bg := color.NRGBA{R: 240, G: 240, B: 240, A: 255}
	size := image.Pt(shellCanvasW, shellCanvasH)
	snap := func(what string) *image.RGBA {
		return golden.Capture(t, size, scene(awaitStableWidget(t, emissions, what), bg))
	}

	m := initialModel()
	send.Next(m)
	closed := snap("initial model")

	m, _ = Update(m, OpenPreferences{})
	send.Next(m)
	if !m.prefsOpen {
		t.Fatal("OpenPreferences did not open the panel in the model")
	}
	open := snap("OpenPreferences")
	if n := regionDiff(closed, open, shellPrefsScrimRegion); n <= 0 {
		t.Errorf("shell canvas unchanged after OpenPreferences (diff=%d in the scrim region); the panel never reached the composed shell", n)
	}

	m, _ = Update(m, ClosePreferences{})
	send.Next(m)
	dismissed := snap("ClosePreferences")
	if n := golden.PixelDiff(closed, dismissed); n != 0 {
		t.Errorf("shell after ClosePreferences differs from the pre-open shell by %d pixel(s); the panel did not dismiss cleanly", n)
	}
}
