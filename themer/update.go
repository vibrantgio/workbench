package main

import (
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
)

// Update is the MVU update function. The application's side effects are
// both files — reading a dropped picture and keeping a chosen colour — and
// both run as commands so the render goroutine never waits on a disk or on
// a twenty-megapixel decode.
func Update(model Model, message mvu.Message) (Model, mvu.Command) {
	switch msg := message.(type) {
	case KeepSeed:
		seed, ok := model.Seed()
		if !ok {
			// Nothing is chosen, so there is nothing to keep. The
			// affordance is not on screen in that state; a message that
			// arrives anyway is not an error.
			return model, mvu.DoNothing()
		}
		return model, KeepTheme(model.KeepPath, seed, model.AppliedBases(), model.Name)
	case desktop.FilesDropped:
		model.DragOver = false
		if len(msg.Paths) == 0 {
			return model, mvu.DoNothing()
		}
		// A multi-file drop takes the first path. Nothing here has a use
		// for a second picture, and refusing the drop outright would be a
		// worse answer than acting on the one the user aimed with.
		return model, LoadImage(msg.Paths[0])
	}
	return ReduceModel(model, message), mvu.DoNothing()
}

// ReduceModel is the pure reducer — every state transition the application
// has except the one that needs a file read. See update_test.go.
func ReduceModel(m Model, message any) Model {
	switch msg := message.(type) {
	case desktop.FilesEntered:
		m.DragOver = true
	case desktop.FilesExited:
		m.DragOver = false
	case ImageLoaded:
		// A picture that decodes but yields nothing — every pixel
		// transparent — is a rejection, not an empty success: replacing
		// the candidate row with nothing would look like a bug.
		if len(msg.Candidates) == 0 {
			m.Problem = shortName(msg.Path) + " has no visible colour in it"
			return m
		}
		m.Preview = msg.Preview
		m.Name = shortName(msg.Path)
		m.Style = "" // a picture replaces a style as the colours' provenance
		m.Candidates = msg.Candidates
		m.Selected = 0 // the leading candidate is the one worth seeing first
		m.Problem = ""
	case ImageRejected:
		// The previous picture and its candidates stay: a failed drop
		// takes nothing away from what is already on screen.
		m.Problem = msg.Reason
	case SelectCandidate:
		if msg.Index >= 0 && msg.Index < len(m.Candidates) {
			m.Selected = msg.Index
		}
	case AdoptStyle:
		// One click, both halves of a theme. The candidates are the style's
		// own, handed to the row exactly as a picture's are, so the leading
		// one is applied and the rest stay on offer beside it; the pair is
		// the style on its own side and the measured counterpart on the
		// other. Everything downstream — the ring, the keep affordance, the
		// base list's override — is looking at the same fields it was.
		if msg.Index >= 0 && msg.Index < len(m.Styles) {
			s := m.Styles[msg.Index]
			m.Preview, m.Style, m.Name = nil, s.Name, s.Name
			m.Candidates, m.Selected = s.Candidates, 0
			m.LightAt = baseIndex(m.Bases, s.Pair.Light, false)
			m.DarkAt = baseIndex(m.Bases, s.Pair.Dark, true)
			m.Problem = ""
		}
	case ShowStyles:
		// Everything the seed came with goes; the pair, which was a separate
		// choice, stays.
		m.Preview, m.Style, m.Name = nil, "", ""
		m.Candidates, m.Selected = nil, 0
		m.Problem = ""
	case SelectBase:
		// The appearance the row was clicked under is the one it changes: a
		// base is fitted to a ground, and the list a name was picked off is
		// the list of names fitted to the ground on screen.
		if msg.Index >= 0 && msg.Index < len(m.Bases) {
			if msg.Dark {
				m.DarkAt = msg.Index
			} else {
				m.LightAt = msg.Index
			}
		}
	case SeedKept:
		m.Kept, m.KeptBases = msg.Seed, msg.Bases
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
