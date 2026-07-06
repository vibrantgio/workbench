// model.go defines the canonical MVU model for the sitedocs app, plus
// the message types and the Update function that reduces them.
//
// Messages:
//   - SetRoute{Page string}     — navigate to a named page (pageHome, pageDocsGettingStarted, …)
//   - ToggleAccordion{Idx int}  — single-open toggle: open section Idx (closing any
//     other open section) or, if Idx is already open, close it
//   - OpenAccordion{Sections map[int]bool} — replace the open-section map wholesale
//
// Update is pure: it takes the current Model and a message and returns the
// next Model. The Command is always DoNothing() — sitedocs has no async
// side-effects.

package main

import "github.com/vibrantgio/mvu"

// Model is the complete runtime state of the sitedocs app.
type Model struct {
	currentPage  string
	openSections map[int]bool
}

// initialModel returns the seed state: the home page with the first
// accordion section open.
func initialModel() Model {
	return Model{
		currentPage:  pageHome,
		openSections: map[int]bool{0: true},
	}
}

// SetRoute navigates to the named page.
type SetRoute struct{ Page string }

// ToggleAccordion applies the single-open policy for accordion section Idx:
// opening Idx closes every other section, and clicking an already-open Idx
// collapses it. The cadence accordion runs with SingleOpen=false, so exactly
// one ToggleAccordion is emitted per click and this reducer — not N+1 OnToggle
// calls — owns the single-open invariant.
type ToggleAccordion struct{ Idx int }

// OpenAccordion replaces the open-section map wholesale. Useful for
// external resets (e.g. restoring a saved UI state).
type OpenAccordion struct{ Sections map[int]bool }

// Update reduces a message into the next Model. It always returns
// mvu.DoNothing() — sitedocs has no async side-effects.
func Update(model Model, msg mvu.Message) (Model, mvu.Command) {
	switch m := msg.(type) {
	case SetRoute:
		model.currentPage = m.Page
	case ToggleAccordion:
		// Single-open policy: opening a section replaces the open set with
		// just that index; clicking the already-open section collapses it.
		if model.openSections[m.Idx] {
			model.openSections = map[int]bool{}
		} else {
			model.openSections = map[int]bool{m.Idx: true}
		}
	case OpenAccordion:
		model.openSections = copyOpenMap(m.Sections)
	}
	return model, mvu.DoNothing()
}
