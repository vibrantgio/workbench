// find.go is find in the page: the search field the window opens in its
// toolbar band, the keys that open it and step through what it finds, and
// what the band carries beside it. What is marked — in the prose and on the
// scrollbar — is the document's; this file holds the query and which match
// the reader is on, and hands both to the document every frame.
//
// The field stands in the band and not over the page because the toolbar is
// the strip holding the controls that act on the document, and a window's
// search is one of them: mail-window.png and voicememos-window.png both keep
// a search recess at the trailing end of the band, and finder-window-light.png
// keeps a magnifier capsule there that expands into one. The band keeps the
// expanded width whichever state the find is in, so the recess grows leftward
// from a fixed trailing end and no control in front of it moves.

package main

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/tokens"
)

const (
	// findKey is the letter of the two find shortcuts this window answers.
	// The page's find takes the platform's own — key.ModShortcut is Cmd on
	// macOS and Ctrl elsewhere, so the modifier is Gio's decision and not a
	// second copy of it here — and the rail's find-a-note takes that
	// shortcut with Shift, the platform's place for the wider of two finds.
	findKey key.Name = "F"

	// findFieldDp is how wide the band's find field lays out, and so how wide
	// the band's find SLOT is whether the find is open or shut: the width the
	// rail's own find field takes, so a search field is one size in this
	// window. It is within three of the 223 px
	// finder-window-untinted-dark.png measures its own expanded field at
	// (x 1100-1322). The platform's own resting toolbar recess measures 325
	// in both stored captures that hold one (voicememos-window.png,
	// mail-window.png), which is wider than this window's trailing column.
	findFieldDp = treeWidthDp - 2*treeFieldPadDp
)

// findFieldSurface is the fill the find field stands on: the trailing end of
// the toolbar band, which is the trailing column's fill risen into it.
func findFieldSurface(c tokens.PlatformColors) color.NRGBA { return bandSurface(c) }

// pageFind is find in the page: the query the reader is looking for in the
// note on screen, which of its matches they are on, and whether the field is
// open at all.
//
// It is view state and not model state for the reason the arrival marking is:
// the vault knows nothing about it, it dies with the field, and nothing here
// is remembered from one launch to the next.
type pageFind struct {
	// open is whether the field stands in the band. Closing it takes the
	// query with it, so nothing is marked while it is shut.
	open bool
	// query is what the reader has typed and current indexes the matches of
	// it, counting from zero.
	query   string
	current int
	// count is how many matches the document reported for the query on the
	// last frame: what the field says, and what stepping wraps around.
	count int
	// note is the note the query is being marked in. A different one starts
	// the walk over, because the matches are another note's.
	note string
	// tag is the field's own focus tag and clear empties the field, both
	// handed over when the field instance is created. A page drawn for a
	// stored image has neither: it answers no key and dismisses nothing.
	tag   event.Tag
	clear func()
	// focus asks this frame to put the keyboard in the field, and seek asks
	// the document to bring the current match into view.
	focus bool
	seek  bool
}

// keys drains the frame's find keys: the platform's find shortcut opens the
// field in the band and takes the keyboard, Enter and Shift+Enter step
// through the matches, and Escape closes the field and hands the keyboard
// back to the document.
//
// The stepping keys are filtered on the field's own focus tag, so they mean
// the next match only while the reader is in the field; the editor under them
// never sees them, because a key event is taken by the first target that asks
// for it and this runs before the field lays out.
func (f *pageFind) keys(gtx layout.Context, read *reader) {
	filters := []event.Filter{key.Filter{Name: findKey, Required: key.ModShortcut}}
	if f.open && f.tag != nil {
		filters = append(filters,
			key.Filter{Focus: f.tag, Name: key.NameReturn, Optional: key.ModShift},
			key.Filter{Focus: f.tag, Name: key.NameEnter, Optional: key.ModShift},
			key.Filter{Focus: f.tag, Name: key.NameEscape},
		)
	}
	for {
		e, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		ke, isKey := e.(key.Event)
		if !isKey || ke.State != key.Press {
			continue
		}
		switch ke.Name {
		case findKey:
			f.open = true
			f.focus = true
		case key.NameEscape:
			f.dismiss(gtx, read)
		case key.NameReturn, key.NameEnter:
			f.step(ke.Modifiers.Contain(key.ModShift))
		}
	}
}

// typed takes what the reader has in the field. A changed query is a new
// search: the walk starts at its first match, and the page goes there as the
// matches come rather than waiting to be stepped.
func (f *pageFind) typed(query string) {
	if query == f.query {
		return
	}
	f.query = query
	f.current = 0
	f.seek = query != ""
}

// step moves to the next match, or to the previous one, wrapping at either
// end of the note.
func (f *pageFind) step(back bool) {
	if f.count == 0 {
		return
	}
	if back {
		f.current = (f.current - 1 + f.count) % f.count
	} else {
		f.current = (f.current + 1) % f.count
	}
	f.seek = true
}

// dismiss closes the field, takes the query and every mark with it, and hands
// the keyboard back to the document the reader was reading.
func (f *pageFind) dismiss(gtx layout.Context, read *reader) {
	*f = pageFind{tag: f.tag, clear: f.clear, note: f.note}
	if f.clear != nil {
		f.clear()
	}
	if read != nil {
		gtx.Execute(key.FocusCmd{Tag: &read.tag})
	}
}

// apply puts the query on the document that is about to lay out and takes
// back what it found: how many matches there are, and where each lies as a
// fraction of the note's height, which is what the scrollbar marks. A
// document with no query on it is left unmarked.
//
// The note the query is marked in is watched here: opening another note
// starts the walk at its first match, because the matches are that note's.
// Arriving at a note moves nobody — a link lands where the link said, and the
// reader steps from there — so the new note's matches are marked where they
// lie and the page stays where the arrival put it.
func (f *pageFind) apply(doc *markdown.Document, note string) []float32 {
	if f.note != note {
		f.note = note
		f.current = 0
		f.seek = false
	}
	if !f.open || f.query == "" {
		doc.ClearFind()
		f.count = 0
		return nil
	}
	doc.Find(f.query, f.current)
	places := doc.MatchPlaces()
	f.count = len(places)
	if f.current >= f.count {
		f.current = 0
		doc.Find(f.query, f.current)
	}
	if f.seek {
		f.seek = false
		doc.ScrollToMatch(f.current)
	}
	return places
}

// label is what the field says about the query it holds: which match the
// reader is on out of how many there are, and that there are none when the
// note holds none. A field with nothing typed in it says nothing.
//
// It stands INSIDE the field at its trailing end, leading of the clear mark,
// which is where a search field on this platform reports what it has found:
// voicememos-multi-folder-search-2026-09-18.png draws the clear mark's disc
// at x 996-1009 in a field whose fill runs x 700-1023, fourteen clear of the
// field's trailing edge against the magnifier's thirteen at its leading one,
// and what the field reports stands in that same trailing end.
func (f *pageFind) label() string {
	switch {
	case f.query == "":
		return ""
	case f.count == 0:
		return "No matches"
	default:
		return fmt.Sprintf("%d of %d", f.current+1, f.count)
	}
}

// findChildren is what the band carries at its trailing end: the find, in a
// slot ONE width whether it is open or shut.
//
// The slot's trailing end is fixed at the measured eight from the window's
// edge, and the recess grows leftward inside it, which is what Finder's does
// when it expands — finder-window-light.png keeps a magnifier capsule at
// x 955-991 in a window 1000 wide and finder-window-untinted-dark.png an
// expanded field at x 1100-1322 in one 1331 wide, both ending eight clear of
// the window. Reserving the open width is what makes that true of this band:
// a slot that grew when the find opened would walk every control before it
// leftward, and a control that moves under the pointer is the defect this
// composition exists to close.
//
// Shut, the slot holds the bordered toolbar control at its trailing end —
// the capsule that opens the field and takes the keyboard, so the shortcut
// has an affordance a hand can reach — and band the window is dragged by in
// front of it. Open, it holds the platform's toolbar search recess, with
// what the query has found standing INSIDE the field at its own trailing
// end.
func (f *frameState) findChildren(m Model, tok themeTokens) []layout.FlexChild {
	find := f.band.find
	// Nothing to search until a note is open: the column drains the find's
	// keys only while it is showing one, so a field standing over an empty
	// page would answer neither Escape nor Enter.
	if m.CurrentNote() == nil {
		return nil
	}
	open := find != nil && find.open
	return []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		w := min(gtx.Dp(unit.Dp(findFieldDp)), gtx.Constraints.Max.X)
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
		if open {
			return f.layoutFindField(gtx, find)
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, dragFill),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return f.layoutFindOpener(gtx, tok)
			}),
		)
	})}
}

// layoutFindOpener draws the magnifier capsule the band keeps while the find
// is shut. It opens the field and asks for the keyboard, which is what the
// shortcut does.
func (f *frameState) layoutFindOpener(gtx layout.Context, tok themeTokens) layout.Dimensions {
	if f.findClick.Clicked(gtx) && f.band.find != nil {
		f.band.find.open = true
		f.band.find.focus = true
	}
	return chromeControl(gtx, tok, &f.findClick, icons.Search, "Find in this note")
}

// layoutFindField lays the search field out at the window's one field width
// and carries out a pending request for the keyboard, which is how the
// shortcut that opened the field puts the reader in it.
func (f *frameState) layoutFindField(gtx layout.Context, find *pageFind) layout.Dimensions {
	if find.focus {
		find.focus = false
		if find.tag != nil {
			gtx.Execute(key.FocusCmd{Tag: find.tag})
		}
	}
	if f.band.field == nil {
		return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
	}
	return f.band.field(gtx)
}
