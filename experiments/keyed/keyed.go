// Package keyed provides KeyedDefer, a companion to rx.Defer for
// preserving per-item state across list reorders.
//
// Problem: rx.Map over an Observable[[]Item] produces new widget closures
// on every emission. When the list is reordered, per-row state (editors,
// checkboxes, scroll offsets) re-binds to the new position rather than
// following its item. KeyedDefer solves this by maintaining a key→value
// registry inside an rx.Defer closure so the same value pointer is
// returned for the same key on every emission.
//
// Typical use inside an rx.Defer closure:
//
//	rx.Defer(func() rx.Observable[layout.Widget] {
//	    editors := keyed.New(func(_ ItemID) *widget.Editor { return &widget.Editor{} })
//	    return rx.Map(items, func(list []Item) layout.Widget {
//	        rows := make([]layout.Widget, len(list))
//	        for i, item := range list {
//	            ed := editors.For(item.ID)  // same *widget.Editor for same ID
//	            rows[i] = editorRow(ed, item)
//	        }
//	        return renderList(rows)
//	    })
//	})
package keyed

import "sync"

// Deferred[K, V] maintains stable per-key values for use inside an rx.Defer
// closure. V is typically a pointer type so mutations made during a Gio frame
// are visible on the next emission without additional indirection.
//
// Cleanup policy is "sticky": values for keys removed from the list are
// retained in the registry and reused if the key is re-added. This preserves
// editor contents and checkbox state across accidental deletions but means
// memory is never freed for the lifetime of the enclosing rx.Defer
// subscription. For long-lived lists with high churn, call Sweep explicitly.
type Deferred[K comparable, V any] struct {
	mu      sync.Mutex
	store   map[K]V
	factory func(K) V
}

// New creates a Deferred state registry. factory is called at most once per
// unique key.
func New[K comparable, V any](factory func(K) V) *Deferred[K, V] {
	return &Deferred[K, V]{
		store:   make(map[K]V),
		factory: factory,
	}
}

// For returns the stable value for key k. If k has not been seen before,
// factory(k) is called and the result stored. Subsequent calls with the same
// key return the stored value regardless of position in the list.
func (d *Deferred[K, V]) For(key K) V {
	d.mu.Lock()
	defer d.mu.Unlock()
	if v, ok := d.store[key]; ok {
		return v
	}
	v := d.factory(key)
	d.store[key] = v
	return v
}

// Sweep releases state for keys not present in activeKeys. Call this after
// a deletion to reclaim memory. Surviving keys are unaffected.
func (d *Deferred[K, V]) Sweep(activeKeys []K) {
	d.mu.Lock()
	defer d.mu.Unlock()
	live := make(map[K]struct{}, len(activeKeys))
	for _, k := range activeKeys {
		live[k] = struct{}{}
	}
	for k := range d.store {
		if _, ok := live[k]; !ok {
			delete(d.store, k)
		}
	}
}

// Len returns the number of live entries in the registry.
func (d *Deferred[K, V]) Len() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.store)
}
