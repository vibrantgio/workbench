package main

import (
	stdcolor "image/color"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
)

// Update is the MVU update function. The application's side effects are both
// files — reading a dropped picture and keeping a chosen colour — and both run
// as commands so the render goroutine never waits on a disk or on a
// twenty-megapixel decode.
func Update(model Model, message mvu.Message) (Model, mvu.Command) {
	switch msg := message.(type) {
	case KeepSeed:
		col, ok := model.Color()
		if !ok {
			// There is no colour on screen, so there is nothing to keep.
			// The affordance is not offered in that state; a message that
			// arrives anyway is not an error.
			return model, mvu.DoNothing()
		}
		if model.Follows() {
			// A theme that follows the system keeps no colour, and credits
			// no picture for one: the colour is the platform's and changes
			// after this file is written.
			return model, KeepTheme(model.KeepPath, stdcolor.NRGBA{}, true, "")
		}
		return model, KeepTheme(model.KeepPath, col, false, model.KeepSource())
	case desktop.FilesDropped:
		model.DragOver = false
		if len(msg.Paths) == 0 {
			return model, mvu.DoNothing()
		}
		// A multi-file drop takes the first path. Nothing here has a use for
		// a second picture, and refusing the drop outright would be a worse
		// answer than acting on the one the user aimed with.
		return model, LoadImage(msg.Paths[0])
	}
	return ReduceModel(model, message), mvu.DoNothing()
}

// ReduceModel is the pure reducer — every state transition the application has
// except the one that needs a file read.
func ReduceModel(m Model, message any) Model {
	switch msg := message.(type) {
	case desktop.FilesEntered:
		m.DragOver = true
	case desktop.FilesExited:
		m.DragOver = false
	case ImageLoaded:
		// A picture that decodes but yields nothing — every pixel
		// transparent — is a rejection, not an empty success: replacing the
		// row with nothing would look like a bug.
		if len(msg.Candidates) == 0 {
			m.Problem = shortName(msg.Path) + " has no visible colour in it"
			return m
		}
		m.Preview = msg.Preview
		m.Name = shortName(msg.Path)
		m.Candidates = msg.Candidates
		m.Selected = 0 // the leading colour is the one worth seeing first
		m.From = FromImage
		m.Problem = ""
	case ImageRejected:
		// The previous picture and its colours stay: a failed drop takes
		// nothing away from what is already on screen.
		m.Problem = msg.Reason
	case SelectCandidate:
		if msg.Index >= 0 && msg.Index < len(m.Candidates) {
			m.Selected, m.From = msg.Index, FromImage
		}
	case FollowSystem:
		// Only where there is a colour to follow. A platform that reports
		// none draws no cell to click, and a message that arrives anyway
		// must not put the window on the zero colour.
		if m.Platform.A != 0 {
			m.From = FromPlatform
		}
	case PlatformColorChanged:
		// The colour on screen while the window is following moves with
		// this, which is the whole of following: no message chooses again.
		m.Platform = msg.Color
	case HexTyped:
		m.Text = msg.Text
		// A run that is not yet a colour leaves the theme colour where it
		// was: a field is empty, then one character long, then five, on the
		// way to being six, and re-theming the preview at each step would
		// show colours nobody asked for.
		if col, ok := parseHex(msg.Text); ok {
			m.Typed, m.From = col, FromHex
		}
	case SeedKept:
		m.Kept, m.KeptFollows = msg.Seed, msg.Follows
		m.Problem = ""
	case KeepFailed:
		m.Problem = msg.Reason
	case SetScheme:
		m.Scheme = ShowLight
		if msg.Dark {
			m.Scheme = ShowDark
		}
	}
	return m
}
