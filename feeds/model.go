// model.go defines the canonical MVU model for the feeds app, plus the
// message types and the Update function that reduces them. GX.10 migrates
// feeds off the rx.Subject + atomic-mirror controller pattern onto the
// Model/Update/Messages loop so every interactive callback (sidebar
// selection, accordion toggle, table sort, pagination) lands a message that
// re-emits the layer observable on the same frame as the click.
//
// Messages:
//   - SelectFeed{Feed FeedID}        — select a feed (filters the articles table, resets to page 1)
//   - SelectArticle{Article ArticleID} — record the row-clicked article (drives the detail pane)
//   - SetPage{Page int}              — navigate the articles table to a 1-indexed page
//   - SetSort{Sort table.Sort}       — set the table sort key/direction
//   - ToggleSection{Idx int}         — single-open accordion toggle for sidebar section Idx
//   - SelectTab{Idx int}             — switch the detail pane's Reader/Raw/Comments tab
//   - ToggleShare{}                  — toggle the navbar Share popover
//   - CloseShare{}                   — close the Share popover (destination click, outside press)
//   - SetSplitRatio{Ratio float32}   — record the articles/detail split-divider position
//
// Update is pure: it takes the current Model and a message and returns the
// next Model. The Command is always DoNothing() — feeds has no async
// side-effects yet.

package main

import (
	"strings"

	"github.com/vibrantgio/cadence/table"
	"github.com/vibrantgio/mvu"
)

// Model is the complete runtime state of the feeds app.
type Model struct {
	feeds           []feedGroup // mutable feed tree, seeded from hardCodedGroups()
	selectedFeed    FeedID
	selectedArticle ArticleID
	currentPage     int
	sort            table.Sort
	openSections    map[int]bool
	selectedTab     int     // detail pane tab: 0 Reader, 1 Raw, 2 Comments
	shareOpen       bool    // navbar Share popover visibility
	splitRatio      float32 // articles/detail SplitPane divider position
	addFeedOpen     bool    // "Add feed" modal visibility
	addFeedError    bool    // empty-URL submit raised the modal alert
}

// initialSplitRatio gives the articles table ~3/5 of the main area so the
// four-column table keeps usable widths next to the detail pane.
const initialSplitRatio = 0.6

// initialModel returns the seed state: the default feed selected, the first
// accordion section open, Published-descending sort, page 1, the Reader tab,
// the Share popover closed, and the default split position.
func initialModel() Model {
	return Model{
		feeds:        hardCodedGroups(),
		selectedFeed: defaultFeedID(),
		currentPage:  1,
		sort:         table.Sort{Column: colPublished, Asc: false},
		openSections: map[int]bool{0: true},
		selectedTab:  0,
		splitRatio:   initialSplitRatio,
	}
}

// SelectFeed selects a feed. Switching feeds resets the table to page 1 so
// the user never lands on an out-of-range page slice for the new feed.
type SelectFeed struct{ Feed FeedID }

// SelectArticle records the article whose row was clicked. The detail pane
// (feeds/detail.go) renders the selected article's header, body tabs, and
// comments placeholder.
type SelectArticle struct{ Article ArticleID }

// SetPage navigates the articles table to the given 1-indexed page.
type SetPage struct{ Page int }

// SetSort sets the table sort key and direction.
type SetSort struct{ Sort table.Sort }

// ToggleSection applies the single-open policy for accordion section Idx:
// opening Idx closes every other section, and clicking an already-open Idx
// collapses it. The cadence accordion runs with SingleOpen=false, so exactly
// one ToggleSection is emitted per click and this reducer — not N+1 OnToggle
// calls — owns the single-open invariant.
type ToggleSection struct{ Idx int }

// SelectTab switches the detail pane's tab strip (0 Reader, 1 Raw,
// 2 Comments). Out-of-range values are stored as-is; cadence/tabs renders
// them as "no tab selected", which is its documented contract.
type SelectTab struct{ Idx int }

// ToggleShare flips the navbar Share popover open/closed. The reducer owns
// the flip so the anchor click callback needs no read-back mirror of the
// current open state.
type ToggleShare struct{}

// CloseShare closes the Share popover. Emitted by destination clicks and by
// the popover's OnDismiss (outside press / arbitration). Idempotent when the
// popover is already closed.
type CloseShare struct{}

// SetSplitRatio records the articles/detail SplitPane divider position as a
// fraction in [0, 1]. Emitted by shell.Props.OnSplitChange during drags so
// the position survives theme re-emissions (which would otherwise snap the
// divider back to the constant fed into Props.SplitRatio).
type SetSplitRatio struct{ Ratio float32 }

// OpenAddFeed shows the "Add feed" modal. It clears any stale alert flag so
// a fresh open never starts with the empty-URL banner showing.
type OpenAddFeed struct{}

// CloseAddFeed hides the "Add feed" modal (close button, backdrop press,
// Escape, or a successful submit). Idempotent when already closed.
type CloseAddFeed struct{}

// SubmitFeed reduces the modal's submit. An empty URL raises the modal alert
// and appends nothing; a non-empty URL synthesises a feed entry, appends it
// to the first group, clears the alert, and closes the modal. The success
// toast is fired from the click callback (the reducer is pure); the reducer
// owns the append/alert/close policy.
type SubmitFeed struct{ URL string }

// ConfirmDelete removes Feed from its group. If the deleted feed was the
// selected one, selection falls back to the first remaining feed (and the
// table resets to page 1, since the old slice no longer applies).
type ConfirmDelete struct{ Feed FeedID }

// Update reduces a message into the next Model. It always returns
// mvu.DoNothing() — feeds has no async side-effects yet.
func Update(model Model, msg mvu.Message) (Model, mvu.Command) {
	switch m := msg.(type) {
	case SelectFeed:
		model.selectedFeed = m.Feed
		model.currentPage = 1 // new feed: reset to the first page.
	case SelectArticle:
		model.selectedArticle = m.Article
	case SetPage:
		model.currentPage = m.Page
	case SetSort:
		model.sort = m.Sort
	case ToggleSection:
		// Single-open policy: opening a section replaces the open set with
		// just that index; clicking the already-open section collapses it.
		if model.openSections[m.Idx] {
			model.openSections = map[int]bool{}
		} else {
			model.openSections = map[int]bool{m.Idx: true}
		}
	case SelectTab:
		model.selectedTab = m.Idx
	case ToggleShare:
		model.shareOpen = !model.shareOpen
	case CloseShare:
		model.shareOpen = false
	case SetSplitRatio:
		model.splitRatio = m.Ratio
	case OpenAddFeed:
		model.addFeedOpen = true
		model.addFeedError = false
	case CloseAddFeed:
		model.addFeedOpen = false
		model.addFeedError = false
	case SubmitFeed:
		if strings.TrimSpace(m.URL) == "" {
			// Empty submit: raise the alert, keep the modal open, append
			// nothing. URL validation beyond non-empty is out of scope.
			model.addFeedError = true
			break
		}
		model.feeds = appendFeed(model.feeds, m.URL)
		model.addFeedOpen = false
		model.addFeedError = false
	case ConfirmDelete:
		model.feeds = deleteFeed(model.feeds, m.Feed)
		if model.selectedFeed == m.Feed {
			model.selectedFeed = firstFeedID(model.feeds)
			model.currentPage = 1
		}
	}
	return model, mvu.DoNothing()
}

// appendFeed synthesises a feed entry from a submitted URL and appends it to
// the first group (the group set is fixed; new feeds join an existing group
// rather than spawning a section — see G5.2d). The label is the URL itself
// and the FeedID is derived from it so the entry is addressable. groups is
// copied before mutation so the previous Model's slice is never aliased.
func appendFeed(groups []feedGroup, url string) []feedGroup {
	url = strings.TrimSpace(url)
	out := cloneGroups(groups)
	if len(out) == 0 {
		out = []feedGroup{{Title: "Feeds"}}
	}
	entry := feedEntry{ID: FeedID("added:" + url), Label: url}
	out[0].Entries = append(append([]feedEntry(nil), out[0].Entries...), entry)
	return out
}

// deleteFeed removes the entry with the given FeedID from whichever group
// holds it, leaving the (possibly now-empty) group in place so the section
// count stays fixed. groups is copied before mutation.
func deleteFeed(groups []feedGroup, id FeedID) []feedGroup {
	out := cloneGroups(groups)
	for gi := range out {
		kept := out[gi].Entries[:0:0]
		for _, e := range out[gi].Entries {
			if e.ID != id {
				kept = append(kept, e)
			}
		}
		out[gi].Entries = kept
	}
	return out
}

// firstFeedID returns the first feed of the first non-empty group, or the
// empty FeedID if every group is empty (nothing left to select).
func firstFeedID(groups []feedGroup) FeedID {
	for _, g := range groups {
		if len(g.Entries) > 0 {
			return g.Entries[0].ID
		}
	}
	return ""
}

// cloneGroups deep-copies the group slice and each group's entry slice so a
// reducer mutation never aliases the previous Model's feed tree.
func cloneGroups(groups []feedGroup) []feedGroup {
	out := make([]feedGroup, len(groups))
	for i, g := range groups {
		g.Entries = append([]feedEntry(nil), g.Entries...)
		out[i] = g
	}
	return out
}
