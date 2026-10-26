package main

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/vibrantgio/backdrop"
	"github.com/vibrantgio/components/alert"
	"github.com/vibrantgio/components/golden"
	vgcolor "github.com/vibrantgio/theme/color"
)

// themerApp is the roster entry these tests fail.
func themerApp(t *testing.T) App {
	t.Helper()
	app, ok := findApp("Themer")
	if !ok {
		t.Fatal("no Themer in the roster")
	}
	return app
}

// compileError is a multi-line error text in the shape `go run` writes for a
// compile failure, with a path long enough to wrap.
const compileError = "# github.com/vibrantgio/workbench/themer\n" +
	"./themer.go:12:2: undefined: someIdentifierThatDoesNotExistAnywhereInThisModule\n" +
	"./themer.go:40:9: cannot use x (variable of type int) as string value in return statement\n" +
	"./themer.go:41:1: missing return"

// TestEachFailurePathHasItsOwnTitle checks the two titles character for
// character: the name between U+201C and U+201D, and U+2019 in can’t.
func TestEachFailurePathHasItsOwnTitle(t *testing.T) {
	app := themerApp(t)
	if got, want := failureTitle(app, Status{State: Failed}), "The application “Themer” can’t be opened."; got != want {
		t.Errorf("not opened: title %q, want %q", got, want)
	}
	if got, want := failureTitle(app, Status{State: Failed, Ended: true}), "“Themer” quit unexpectedly."; got != want {
		t.Errorf("ended: title %q, want %q", got, want)
	}
}

// TestTheReducerRecordsTheFailurePath folds each failure message into the
// model: Status records which path failed and keeps the whole error text.
func TestTheReducerRecordsTheFailurePath(t *testing.T) {
	notOpened := `exec: "go": executable file not found in $PATH`
	cases := []struct {
		name string
		msg  Exited
		want Status
	}{
		{"not opened", Exited{Name: "Themer", Err: notOpened}, Status{State: Failed, Detail: notOpened}},
		{"ended", Exited{Name: "Themer", Ended: true, Err: compileError}, Status{State: Failed, Ended: true, Detail: compileError}},
		{"clean exit", Exited{Name: "Themer", Ended: true}, Status{}},
	}
	for _, tc := range cases {
		model, _ := Update(Model{}, tc.msg)
		if got := model.StatusOf("Themer"); got != tc.want {
			t.Errorf("%s: status %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// TestExitDetailKeepsTheWholeErrorText checks that the error text is all of
// stderr, every line of it, and that the exit error is used only when stderr
// is empty.
func TestExitDetailKeepsTheWholeErrorText(t *testing.T) {
	exit := errors.New("exit status 1")
	if got := exitDetail(exit, bytes.NewBufferString(compileError+"\n\n")); got != compileError {
		t.Errorf("detail %q, want the whole stderr %q", got, compileError)
	}
	if got := exitDetail(exit, new(bytes.Buffer)); got != "exit status 1" {
		t.Errorf("empty stderr: detail %q, want the exit error", got)
	}
}

// TestAFailedCardDrawsTheErrorAlertOutsideTheButton renders a failed card on
// each path in both schemes. The alert's box is the span of its hairline. The
// Error mark is drawn inside that box, the button's fill is drawn below it and
// never inside it, and the card is the alert's depth taller than a card with
// no failure.
func TestAFailedCardDrawsTheErrorAlertOutsideTheButton(t *testing.T) {
	app := themerApp(t)
	for _, sc := range windowSchemes {
		for _, ended := range []bool{false, true} {
			name := sc.name + "/not opened"
			if ended {
				name = sc.name + "/ended"
			}
			t.Run(name, func(t *testing.T) {
				tok := staticThemed(sc.colors)
				status := Status{State: Failed, Ended: ended, Detail: compileError}
				size := image.Pt(int(CellW), 600)
				var click widget.Clickable
				card := appGroup(tok, app, &click, status)
				var cardH int
				img := golden.Capture(t, size, golden.Stack(backdrop.Widget(windowPlane(sc.colors)), func(gtx layout.Context) layout.Dimensions {
					d := card(gtx)
					cardH = d.Size.Y
					return d
				}))
				box := alertBox(img, vgcolor.Flatten(sc.colors.Separator, windowPlane(sc.colors)), cardH)
				if box.Empty() {
					t.Fatal("no alert hairline drawn")
				}
				mark := spanOf(img, alert.Mark(sc.colors, alert.Error), image.Rectangle{Max: size})
				if mark.Empty() || !mark.In(box) {
					t.Errorf("Error mark span %v, want it inside the alert box %v", mark, box)
				}
				button := spanOf(img, sc.colors.DefaultButtonFill, image.Rectangle{Max: size})
				if button.Empty() {
					t.Fatal("no button fill drawn")
				}
				if button.Overlaps(box) || button.Min.Y < box.Max.Y {
					t.Errorf("button span %v, want it below the alert box %v and outside it", button, box)
				}
				if want := int(CellH) + box.Dy() + int(tok.spacing.S3); cardH != want {
					t.Errorf("card height %d, want %d: the fixed height, the alert %d and the gap", cardH, want, box.Dy())
				}
			})
		}
	}
}

// TestTheAlertShowsTheWholeErrorText measures the alert at the card's slot
// width. Each line added to the error text makes the alert deeper, so no
// line of the text is cut off or folded into an ellipsis.
func TestTheAlertShowsTheWholeErrorText(t *testing.T) {
	tok := staticThemed(windowSchemes[0].colors)
	app := themerApp(t)
	depth := func(lines int) int {
		detail := strings.TrimSuffix(strings.Repeat("panic: line\n", lines), "\n")
		var h int
		golden.Capture(t, image.Pt(int(CellW), 10), func(gtx layout.Context) layout.Dimensions {
			h = failureHeight(gtx, tok, failureAlert(tok, app, Status{State: Failed, Ended: true, Detail: detail}))
			return layout.Dimensions{}
		})
		return h
	}
	prev := depth(1)
	for _, lines := range []int{2, 10, 40} {
		h := depth(lines)
		if h <= prev {
			t.Errorf("%d lines: alert depth %d, want more than %d", lines, h, prev)
		}
		prev = h
	}
}

// TestALaunchThatRunsRemovesTheAlert starts a failed app again. The status
// leaves Failed at once, and the card then has no alert.
func TestALaunchThatRunsRemovesTheAlert(t *testing.T) {
	app := themerApp(t)
	tok := staticThemed(windowSchemes[0].colors)
	model := Model{}.withStatus(app.Name, Status{State: Failed, Ended: true, Detail: compileError})
	if failureAlert(tok, app, model.StatusOf(app.Name)) == nil {
		t.Fatal("failed: no alert")
	}
	model, _ = Update(model, Launch{Name: app.Name})
	if s := model.StatusOf(app.Name); s.State != Starting || failureAlert(tok, app, s) != nil {
		t.Errorf("relaunched: status %+v, want Starting and no alert", s)
	}
	model, _ = Update(model, Started{Name: app.Name})
	if s := model.StatusOf(app.Name); s.State != Running || failureAlert(tok, app, s) != nil {
		t.Errorf("running: status %+v, want Running and no alert", s)
	}
}

// alertBox is the alert's rectangle on a card cardH high: from the first to
// the last row inside the card's own hairline that holds a run of the
// hairline colour across most of the card's width. Text never draws such a
// run, and the card's own hairline is on the rows and columns excluded.
func alertBox(img *image.RGBA, edge color.NRGBA, cardH int) image.Rectangle {
	w := int(CellW)
	var box image.Rectangle
	for y := 2; y < cardH-2; y++ {
		n, minX, maxX := 0, w, 0
		for x := 2; x < w-2; x++ {
			if pixelAt(img, image.Pt(x, y)) == edge {
				n++
				minX, maxX = min(minX, x), max(maxX, x)
			}
		}
		if n >= w*2/3 {
			box = box.Union(image.Rect(minX, y, maxX+1, y+1))
		}
	}
	return box
}

// spanOf is the smallest rectangle within r holding every pixel of img that
// is exactly c.
func spanOf(img *image.RGBA, c color.NRGBA, r image.Rectangle) image.Rectangle {
	var span image.Rectangle
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if pixelAt(img, image.Pt(x, y)) == c {
				span = span.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return span
}
