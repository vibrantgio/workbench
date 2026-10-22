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
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/shell"
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
// the toolbar, which is the trailing column's fill risen into it. A toolbar
// carries no fill of its own — it is the fill of whatever region lies under
// it, continued upward (reference/macos/controls.md) — and the trailing end
// of this window's toolbar lies over the inspector.
func findFieldSurface(c tokens.PlatformColors) color.NRGBA { return chromeSurface(c) }

// pageFind is find in the page: the query the reader is looking for in the
// note on screen, and which of its matches they are on.
//
// It is view state and not model state for the reason the arrival marking is:
// the vault knows nothing about it, it dies with the field, and nothing here
// is remembered from one launch to the next. Whether the field stands in the
// band at all is the one piece that is NOT here: that is window state, and
// it is the model's, in Model.FindOpen, so that opening the field goes
// through the loop and the window is asked for the frame that draws it.
// Nothing here is marked while the model says the find is shut.
type pageFind struct {
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
	// returnTo is the focus target that held the keyboard when the field was
	// opened, and where dismissing the field hands it back. A field opened
	// where no holder could be read records nothing and the document takes
	// the keyboard back.
	returnTo event.Tag
	// post is where the find's two answers go — the opening and the
	// shutting, which are the model's: the live window's is the frame's
	// message op, which is what a nil leaves, and a test's is a recorder,
	// so a find driven without a window to post to can still be read back.
	post func(gtx layout.Context, msg mvu.Message)
}

// send hands one of the find's answers on to whatever is collecting this
// frame's messages.
func (f *pageFind) send(gtx layout.Context, msg mvu.Message) {
	if f.post != nil {
		f.post(gtx, msg)
		return
	}
	mvu.MessageOp{Message: msg}.Add(gtx.Ops)
}

// keys drains the frame's find keys: the platform's find shortcut opens the
// field in the band and takes the keyboard, Enter and Shift+Enter step
// through the matches, and Escape closes the field and hands the keyboard
// back to where the field took it from.
//
// The stepping keys are filtered on the field's own focus tag, so they mean
// the next match only while the reader is in the field; the editor under them
// never sees them, because a key event is taken by the first target that asks
// for it and this runs before the field lays out.
func (f *pageFind) keys(gtx layout.Context, m Model, read *reader) {
	filters := []event.Filter{key.Filter{Name: findKey, Required: key.ModShortcut}}
	if m.FindOpen && f.tag != nil {
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
			f.openField(gtx, read)
		case key.NameEscape:
			f.dismiss(gtx, read)
		case key.NameReturn, key.NameEnter:
			f.step(ke.Modifiers.Contain(key.ModShift))
		}
	}
}

// openField opens the field in the band, asks for the keyboard, and records
// which target held it, so dismissing the field hands it back there. Both
// ways of opening the find run through here — the shortcut and the band's
// magnifier capsule — so the record is made in one place and a field opened
// either way gives the keyboard back to the same target.
//
// The opening is a message because the field standing in the band is the
// model's: the loop reduces it, re-emits the window and invalidates it, so
// the frame that draws the field is a frame the reader's one click asked
// for. The keyboard request is not a message — a pending focus command has
// no business in the model — and it is carried out by the frame that draws
// the field, so the field comes up with the keys already in it.
func (f *pageFind) openField(gtx layout.Context, read *reader) {
	f.send(gtx, OpenFind{})
	f.focus = true
	f.returnTo = keyboardHolder(gtx, read)
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

// keyboardHolder answers which target holds the keyboard as the find is
// opened: the rail's rows while they have it, the document otherwise. A
// composition with no note column reads nothing and records nothing.
//
// A focus command reaches the router at the end of the frame that executes
// it, so what this reads during the frame the press or the click is delivered
// on is where the keyboard stood when the reader acted, whichever column or
// band laid out first.
func keyboardHolder(gtx layout.Context, read *reader) event.Tag {
	if read == nil {
		return nil
	}
	if read.rail.holds(gtx) {
		return read.rail.focus()
	}
	return &read.tag
}

// dismiss closes the field, takes the query and every mark with it, and hands
// the keyboard back to the target the field took it from.
func (f *pageFind) dismiss(gtx layout.Context, read *reader) {
	back := f.returnTo
	f.send(gtx, CloseFind{})
	*f = pageFind{tag: f.tag, clear: f.clear, note: f.note, post: f.post}
	if f.clear != nil {
		f.clear()
	}
	if read == nil {
		return
	}
	// Only the rail's rows are handed the keyboard back, and only while the
	// tag recorded is still the one they hold: a rail rebuilt since the field
	// opened publishes another tag, and a target no longer in the window
	// would take the keyboard out of every focusable there is.
	if back == nil || back != read.rail.focus() {
		back = &read.tag
	}
	gtx.Execute(key.FocusCmd{Tag: back})
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
func (f *pageFind) apply(m Model, doc *markdown.Document) []float32 {
	if f.note != m.Current {
		f.note = m.Current
		f.current = 0
		f.seek = false
	}
	if !m.FindOpen || f.query == "" {
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

// findTrailing is what the toolbar carries at its trailing end: the find, in
// a slot ONE width whether it is open or shut.
//
// The slot's trailing end is fixed at the measured eight from the window's
// edge, and the recess grows leftward inside it, which is what Finder's does
// when it expands — finder-window-light.png keeps a magnifier capsule at
// x 955-991 in a window 1000 wide and finder-window-untinted-dark.png an
// expanded field at x 1100-1322 in one 1331 wide, both ending eight clear of
// the window. Reserving the open width is what makes that true of this
// toolbar: a slot that grew when the find opened would walk every control
// before it leftward, and a control that moves under the pointer is the
// defect this composition exists to close.
//
// Shut, the slot holds the bordered toolbar control at its trailing end —
// the capsule that opens the field and takes the keyboard, so the shortcut
// has an affordance a hand can reach — and room the window is dragged by in
// front of it. Open, it holds the platform's toolbar search recess, with
// what the query has found standing INSIDE the field at its own trailing
// end.
func (f *frameState) findTrailing(m Model, tok themeTokens) layout.Widget {
	find := f.band.find
	// Nothing to search until a note is open: the column drains the find's
	// keys only while it is showing one, so a field standing over an empty
	// page would answer neither Escape nor Enter. A fetch out of the cloud
	// is one of those states — the window stands on the note it asked for
	// and has none of it — so the control is away for the length of the
	// wait and stands again the moment the note lands.
	if m.CurrentNote() == nil {
		return nil
	}
	return func(gtx layout.Context) layout.Dimensions {
		w := min(gtx.Dp(unit.Dp(findFieldDp)), gtx.Constraints.Max.X)
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
		// Read at layout time, off the model this frame is drawn from: the
		// state the child draws by is the state the frame was given, not a
		// value this list carried in from before its children ran.
		if find != nil && m.FindOpen {
			return f.layoutFindField(gtx, find)
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, pane.DragFill),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return f.layoutFindOpener(gtx, tok)
			}),
		)
	}
}

// layoutFindOpener draws the magnifier capsule the band keeps while the find
// is shut. It opens the field, asks for the keyboard and records which target
// held it, which is what the shortcut does: the capsule and the shortcut are
// two ways to the same control, so dismissing the field gives the keyboard
// back to the same target whichever opened it.
func (f *frameState) layoutFindOpener(gtx layout.Context, tok themeTokens) layout.Dimensions {
	if f.findClick.Clicked(gtx) && f.band.find != nil {
		f.band.find.openField(gtx, f.band.read)
	}
	return shell.ToolbarControl(gtx, tok.col, tokens.Radius, tok.den, &f.findClick,
		icons.Mark(icons.Search), "Find in this note", false)
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
