// find.go wires the toolbar's search field to the guide: the query marks
// every match in the guide and in the guide column's scroll bar, the field
// counts the matches, and Enter and Shift+Enter step the current match.

package main

import (
	"strconv"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/input"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/theme/theme"
)

// toolbarSearchPrompt is the search field's placeholder.
const toolbarSearchPrompt = "Search"

// finder is the find that the toolbar's search field drives in the guide.
//
// The field stands on chrome. The marks and the count stay while the query
// is in the field, wherever the keyboard is. They leave when the query is
// removed: by the clear mark, by Escape or by deleting the text. The field
// itself empties on the clear mark and on Escape, and reports the empty
// query through OnChange.
//
// The query and the current match are frame state: the field and the page
// write them, all on the frame.
type finder struct {
	// field is the field's focus tag. Edit ▸ Find… gives it the keyboard.
	field event.Tag
	// doc is the guide that the find marks. It is nil when the guide did
	// not load, and the find then marks nothing.
	doc *markdown.Document
	// query is the field's text.
	query string
	// current indexes the document's matches: the match that Enter and
	// Shift+Enter step from.
	current int
}

// searchField builds the toolbar's search field over the finder: the chrome
// variant in a toolbar, on the page's fill.
//
// The chrome variant does not read Enter. readSteps reads Return and Enter
// in the field before the field lays out, because Gio gives a key event to
// the first request in the frame that matches it.
func (f *finder) searchField(th rx.Observable[theme.Theme]) rx.Observable[layout.Widget] {
	field := input.SearchField(th, input.SearchFieldProps{
		Placeholder: toolbarSearchPrompt,
		Description: "search the guide",
		Variant:     input.Chrome,
		Region:      input.Toolbar,
		Surface:     pageFill,
		FocusTag:    func(tag event.Tag) { f.field = tag },
		OnChange:    func(_ layout.Context, text string) { f.setQuery(text) },
		Count:       f.count,
	})
	return rx.Map(field, func(w layout.Widget) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			f.readSteps(gtx)
			return w(gtx)
		}
	})
}

// readSteps steps to the next match on Enter and to the previous one on
// Shift+Enter, while the field holds the keyboard.
func (f *finder) readSteps(gtx layout.Context) {
	if f.field == nil {
		return
	}
	for _, name := range []key.Name{key.NameReturn, key.NameEnter} {
		for {
			ev, ok := gtx.Event(key.Filter{Focus: f.field, Name: name, Optional: key.ModShift})
			if !ok {
				break
			}
			ke, isKey := ev.(key.Event)
			if !isKey || ke.State != key.Press {
				continue
			}
			if ke.Modifiers.Contain(key.ModShift) {
				f.step(-1)
			} else {
				f.step(1)
			}
		}
	}
}

// setQuery takes the field's new text. The first match becomes the current
// match and scrolls into view. An empty query takes every mark off.
func (f *finder) setQuery(text string) {
	f.query = text
	f.current = 0
	if f.doc == nil {
		return
	}
	if text == "" {
		f.doc.ClearFind()
		return
	}
	f.doc.Find(text, 0)
	f.doc.ScrollToMatch(0)
}

// step moves the current match by delta, from the last match to the first
// and from the first to the last, and scrolls it into view.
func (f *finder) step(delta int) {
	if f.doc == nil || f.query == "" {
		return
	}
	n := len(f.doc.Matches())
	if n == 0 {
		return
	}
	f.current = ((f.current+delta)%n + n) % n
	f.doc.Find(f.query, f.current)
	f.doc.ScrollToMatch(f.current)
}

// currentMatch is the index of the current match, for the scroll bar's track.
func (f *finder) currentMatch() int { return f.current }

// count is the field's count: the number of matches, and nothing while the
// field holds no text. The reference measures the number alone, with no
// "1 of 8".
func (f *finder) count() string {
	if f.query == "" {
		return ""
	}
	if f.doc == nil {
		return "0"
	}
	return strconv.Itoa(len(f.doc.Matches()))
}

// track puts the matches on the guide column's scroll bar: where each match
// lies, and which one is current. With no marks on the guide it puts none.
func track(bar scrollbar.Style, doc *markdown.Document, current int) scrollbar.Style {
	if places := doc.MatchPlaces(); len(places) > 0 {
		bar.Matches = places
		bar.Current = current
	}
	return bar
}
