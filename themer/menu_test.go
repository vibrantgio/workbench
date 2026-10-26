package main

import (
	"testing"

	"github.com/vibrantgio/mvu"
)

// The bar holds Keep in File and Light and Dark in View, with no key
// equivalent, because the window binds no chord. Each item posts the
// message of its control: KeepColor is the keep control's, and the two
// SetScheme values are the scheme control's two segments'.
func TestTheBarHoldsKeepLightAndDark(t *testing.T) {
	want := []struct {
		menu, title string
		msg         mvu.Message
	}{
		{"File", "Keep", KeepColor{}},
		{"View", "Light", SetScheme{Dark: false}},
		{"View", "Dark", SetScheme{Dark: true}},
	}
	items := MenuItems()
	if len(items) != len(want) {
		t.Fatalf("the menu declares %d items, want %d", len(items), len(want))
	}
	for i, w := range want {
		it := items[i]
		if it.Menu != w.menu || it.Title != w.title || it.Msg != w.msg {
			t.Errorf("item %d is %q in %q posting %#v, want %q in %q posting %#v",
				i, it.Title, it.Menu, it.Msg, w.title, w.menu, w.msg)
		}
		if it.Key != "" {
			t.Errorf("%q has the key equivalent %q, want none", it.Title, it.Key)
		}
	}
}

// Light and Dark move the window to the scheme they name, from either side.
func TestLightAndDarkSetTheScheme(t *testing.T) {
	for _, it := range MenuItems() {
		if it.Menu != "View" {
			continue
		}
		dark := it.Msg.(SetScheme).Dark
		for _, from := range []Scheme{FollowOS, ShowLight, ShowDark} {
			m, _ := Update(Model{Scheme: from}, it.Msg)
			want := ShowLight
			if dark {
				want = ShowDark
			}
			if m.Scheme != want {
				t.Errorf("%q from %v shows %v, want %v", it.Title, from, m.Scheme, want)
			}
		}
	}
}
