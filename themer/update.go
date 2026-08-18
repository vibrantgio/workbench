package main

import (
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
)

// Update is the MVU update function. The only side effect the application
// has is reading a dropped file, and it runs as a command so the render
// goroutine never waits on a disk or on a twenty-megapixel decode.
func Update(model Model, message mvu.Message) (Model, mvu.Command) {
	switch msg := message.(type) {
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
	case SetScheme:
		m.Scheme = ShowLight
		if msg.Dark {
			m.Scheme = ShowDark
		}
	}
	return m
}
