package main

import (
	"fmt"
	"image"
	"testing"

	"gioui.org/unit"

	"github.com/vibrantgio/components/golden"
	"github.com/vibrantgio/theme/tokens"
)

// settingsTrigger is the settings dialog's default-model trigger in this
// window: x 514-771, y 457-480 in window-settings-light.png, the box
// defaultPickerCentre is the centre of.
var settingsTrigger = image.Rect(514, 457, 772, 481)

// TestTheSettingsPickersMenuIsBoundedByTheWindow holds the settings dialog's
// default-model picker to the attachment, with two catalogues.
//
// The first catalogue is taller than the dialog's body and shorter than the
// window. The menu over the trigger runs past the window's bottom edge, so
// the attachment pushes it up inside the window. The menu then reaches far
// above the body's top edge, which a menu held to the body's own rectangle
// cannot do, and its top edge stays inside the window.
//
// The second catalogue is taller than the window. The menu prefers the side
// above the trigger, so the window caps it at the window's top edge, and the
// menu scrolls inside itself. Its plane starts on the window's first row
// only where the field was given the window's available room at the
// trigger's own offset.
func TestTheSettingsPickersMenuIsBoundedByTheWindow(t *testing.T) {
	saved := windowButtonsEnd
	defer func() { windowButtonsEnd = saved }()
	windowButtonsEnd = func() unit.Dp { return buttonsEndDp }

	// The body's top edge: the trigger's row is the body's last, and the
	// body is SettingsBodyHeight tall at one pixel per dp. The menu's
	// shadow reaches past its plane, so the menu must clear the edge by
	// more than the shadow does.
	bodyTop := settingsTrigger.Max.Y - int(SettingsBodyHeight)
	const clear = 40
	top, bottom, left := openMenuRows(t, 25, bodyTop-clear)
	if left < settingsTrigger.Min.X-clear {
		t.Errorf("25 models: the open menu changes columns from %d; want it over the trigger, which starts at %d: the field was not given the available room at its own offset", left, settingsTrigger.Min.X)
	}
	if top >= bodyTop-clear {
		t.Errorf("25 models: the open menu changes rows %d..%d; want it to reach more than %d px above the body's top edge at %d: the menu is held to the body, not to the window", top, bottom, clear, bodyTop)
	}
	if top <= 0 {
		t.Errorf("25 models: the open menu changes rows %d..%d; want its top edge inside the window", top, bottom)
	}

	if top, bottom, _ := openMenuRows(t, 60, 0); top != 0 {
		t.Errorf("60 models: the open menu changes rows %d..%d; want its plane to start on the window's top edge, where the window caps it", top, bottom)
	}
}

// openMenuRows opens the settings dialog's default-model picker on a
// catalogue of n models and reports the first and last pixel rows the open
// menu changes in the window, and the first pixel column it changes above
// row above. Only the menu reaches above the dialog, so the column is the
// menu's left edge there; inside the dialog, moving the keyboard to the
// trigger changes the form too.
func openMenuRows(t *testing.T, n, above int) (top, bottom, left int) {
	t.Helper()
	settings, _ := Update(demoModel(), OpenSettings{})
	if len(settings.Settings.Draft) == 0 {
		t.Fatal("the demo model has no provider to give a catalogue to")
	}
	models := make([]string, n)
	for i := range models {
		models[i] = fmt.Sprintf("model-%02d", i)
	}
	settings.Settings.Draft[0].Models = models

	c := tokens.PlatformLight
	closed := golden.Capture(t, windowSize, withWindowControls(frame(t, c, settings)))
	open := golden.Capture(t, windowSize, clicked(withWindowControls(frame(t, c, settings)), windowSize, defaultPickerCentre))

	top, bottom, left = -1, -1, -1
	b := open.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if open.RGBAAt(x, y) != closed.RGBAAt(x, y) {
				if top < 0 {
					top = y
				}
				bottom = y
				if y < above && (left < 0 || x < left) {
					left = x
				}
				break
			}
		}
	}
	if top < 0 {
		t.Fatalf("%d models: clicking the default-model trigger at %v changed nothing: the menu did not open", n, defaultPickerCentre)
	}
	return top, bottom, left
}
