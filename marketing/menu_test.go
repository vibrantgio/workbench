package main

import "testing"

// The bar holds no item of the window's own: the page posts no message
// through a control.
func TestTheBarHoldsNoItemOfTheWindow(t *testing.T) {
	if items := MenuItems(); len(items) != 0 {
		t.Errorf("the menu declares %d items, want none: %+v", len(items), items)
	}
}
