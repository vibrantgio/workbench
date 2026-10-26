package main

import "testing"

// The bar holds no item of the window's own: the window has no selected
// application for a Launch item to act on.
func TestTheBarHoldsNoItemOfTheWindow(t *testing.T) {
	if items := MenuItems(); len(items) != 0 {
		t.Errorf("the menu declares %d items, want none: %+v", len(items), items)
	}
}
