// model.go defines the canonical MVU model for the sitedocs app, plus
// the message types and the Update function that reduces them.
//
// Messages:
//   - ToggleOutline{Idx int}     — flip the disclosure of the Idx-th ## section
//     of the docs outline; sections disclose independently
//   - SelectHeading{Block int}   — mark the outline row for the heading at
//     that document block index as selected
//   - OpenFind{}                 — open the find bar and give the keyboard to
//     its field
//   - CloseFind{}                — close the find bar and give the keyboard
//     back to the guide
//
// Update is pure: it takes the current Model and a message and returns the
// next Model. The Command is always DoNothing() — sitedocs has no async
// side-effects.

package main

import "github.com/vibrantgio/mvu"

// Model is the complete runtime state of the sitedocs app.
type Model struct {
	// outlineOpen is the docs outline's disclosure state: ## entry index →
	// disclosed. Sections fold independently — an outline is a map of the
	// document, not an accordion with a single-open policy.
	outlineOpen map[int]bool
	// selectedHeading is the document block index of the selected outline
	// row, -1 when nothing is selected yet.
	selectedHeading int
	// findOpen is whether the find bar stands over the guide column.
	findOpen bool
	// findMoves counts the requests that move the keyboard between the find
	// bar's field and the guide. The page compares it on each frame and
	// moves the keyboard once per change.
	findMoves int
}

// initialModel returns the state the app opens on: the first ## section of
// the docs outline disclosed, nothing selected.
func initialModel() Model {
	return Model{
		outlineOpen:     map[int]bool{0: true},
		selectedHeading: -1,
	}
}

// ToggleOutline flips the disclosure of the Idx-th ## section in the docs
// outline tree. Disclosure is per-section and independent.
type ToggleOutline struct{ Idx int }

// SelectHeading marks the outline row whose heading sits at document
// block index Block as the selected row. The scroll itself is the
// markdown Document's (ScrollToBlock), fired by the same click.
type SelectHeading struct{ Block int }

// OpenFind opens the find bar and gives the keyboard to its field. With the
// bar open already, it gives the keyboard to the field again.
type OpenFind struct{}

// CloseFind closes the find bar and gives the keyboard back to the guide.
// With the bar closed, it does nothing.
type CloseFind struct{}

// copyOpenMap returns a shallow copy of m, keeping Update pure.
func copyOpenMap(m map[int]bool) map[int]bool {
	cp := make(map[int]bool, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

// Update reduces a message into the next Model. It always returns
// mvu.DoNothing() — sitedocs has no async side-effects.
func Update(model Model, msg mvu.Message) (Model, mvu.Command) {
	switch m := msg.(type) {
	case ToggleOutline:
		next := copyOpenMap(model.outlineOpen)
		if next[m.Idx] {
			delete(next, m.Idx)
		} else {
			next[m.Idx] = true
		}
		model.outlineOpen = next
	case SelectHeading:
		model.selectedHeading = m.Block
	case OpenFind:
		model.findOpen = true
		model.findMoves++
	case CloseFind:
		if model.findOpen {
			model.findOpen = false
			model.findMoves++
		}
	}
	return model, mvu.DoNothing()
}
