package main

import (
	"testing"

	"github.com/vibrantgio/mvu"
)

// The bar holds one View item for each tab, in the strip's order, with no
// key equivalent. Each item posts the message its tab posts, and the update
// selects that tab.
func TestTheBarHoldsOneViewItemPerTab(t *testing.T) {
	want := []struct {
		title string
		msg   mvu.Message
	}{
		{"Docs", SetRoute{Page: pageDocs}},
	}
	items := MenuItems()
	if len(items) != len(want) {
		t.Fatalf("the menu declares %d items, want %d", len(items), len(want))
	}
	for i, w := range want {
		item := items[i]
		if item.Menu != "View" || item.Title != w.title || item.Key != "" || item.Msg != w.msg {
			t.Errorf("item %d is %+v, want View %q with no key posting %#v", i, item, w.title, w.msg)
		}
		m, _ := Update(initialModel(), item.Msg)
		if got := tabIndex(m.currentPage); got != i {
			t.Errorf("%q selects tab %d, want %d", w.title, got, i)
		}
	}
}
