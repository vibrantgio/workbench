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
// keeps a magnifier capsule there that expands into one.

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
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/tokens"
)

const (
	// findKey is the letter of the two find shortcuts this window answers.
	// The page's find takes the platform's own — key.ModShortcut is Cmd on
	// macOS and Ctrl elsewhere, so the modifier is Gio's decision and not a
	// second copy of it here — and the rail's find-a-note takes that
	// shortcut with Shift, the platform's place for the wider of two finds.
	findKey key.Name = "F"

	// findFieldDp is how wide the band's find field lays out: the width the
	// rail's own find field takes, so a search field is one size in this
	// window. The platform's own toolbar recess measures 325 px in both
	// stored captures that hold one (voicememos-window.png,
	// mail-window.png), which is wider than this window's trailing column;
	// what is kept here is the window's one size for a search field.
	findFieldDp = treeWidthDp - 2*treeFieldPadDp

	// findCountGapDp is the air between the field and what it says it has
	// found: the pad the rail keeps around its own find field, which is the
	// tightest stop of the scale this window spends beside a control.
	findCountGapDp = treeFieldPadDp
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

// label is what the field says about the query beside it: which match the
// reader is on out of how many there are, and that there are none when the
// note holds none. A field with nothing typed in it says nothing.
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

// findChildren is what the band carries at its trailing end: the find. Shut,
// it is the bordered toolbar control finder-window-light.png keeps there — a
// magnifier capsule 37 px wide at x 955-991, eight clear of the window's own
// trailing edge — which opens the field and takes the keyboard, so the
// shortcut has an affordance a hand can reach. Open, it is the platform's
// toolbar search recess, with what the query has found standing leading of
// it: the recess keeps the trailing end the captures give it, so the count
// stands before it rather than after.
func (f *frameState) findChildren(m Model, tok themeTokens) []layout.FlexChild {
	find := f.band.find
	// Nothing to search until a note is open: the column drains the find's
	// keys only while it is showing one, so a field standing over an empty
	// page would answer neither Escape nor Enter.
	if m.CurrentNote() == nil {
		return nil
	}
	if find == nil || !find.open {
		return []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return f.layoutFindOpener(gtx, tok)
		})}
	}
	return []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// The count says how much of the query is left to walk rather
			// than anything about the note, so it reads under the note's own
			// text at the platform's secondary strength.
			return drawLabel(gtx, tok.shaper, find.label(), tok.typ.BodyMedium,
				vgcolor.Flatten(tok.col.SecondaryLabel, bandSurface(tok.col)))
		}),
		// The count belongs to the field beside it and to nothing else, so it
		// stands one stop of the scale from it rather than a band gap away.
		layout.Rigid(dragSpacer(unit.Dp(findCountGapDp))),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return f.layoutFindField(gtx, find)
		}),
	}
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
	w := min(gtx.Dp(unit.Dp(findFieldDp)), gtx.Constraints.Max.X)
	gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
	if f.band.field == nil {
		return layout.Dimensions{Size: image.Pt(w, 0)}
	}
	return f.band.field(gtx)
}
