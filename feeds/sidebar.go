package main

import (
	"image"
	"image/color"
	"sort"
	"strconv"
	"sync/atomic"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/components/icons"
	"github.com/vibrantgio/components/keyed"
	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/pointershape"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/components/toast"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/patterns/notifications"
	"github.com/vibrantgio/patterns/pane"
	"github.com/vibrantgio/patterns/popover"
	patsidebar "github.com/vibrantgio/patterns/sidebar"
	vgcolor "github.com/vibrantgio/theme/color"
	"github.com/vibrantgio/theme/system/naming"
	"github.com/vibrantgio/theme/theme"
	"github.com/vibrantgio/theme/tokens"
)

// feedsPaneColumnDp is what the rail claims of the window: the panel plus the
// margin of the window's own plane standing to its leading side. The content
// beside it begins at the panel's trailing rim with no gap, which is the one
// side a pane is not set in from.
//
// The panel's own width is patsidebar.ExpandedWidth and not a number of this
// application's: patterns/sidebar took the reading and owns it.
const (
	feedsPaneColumnDp = pane.MarginDp + patsidebar.ExpandedWidth
	trashColWDp       = 24 // trailing trash-icon hit area, hover-revealed
)

// feedsRowTailGapDp is the air a row keeps between the end of its name and
// whatever stands at its trailing end, so a long name never runs into the
// count. The row's height, its three columns, the pill's inset from the
// rail's edges and its corner are all patterns/sidebar's — a list of feeds
// is a chrome rail, so its rows are the sidebar's rows and not the
// platform's list rows.
const feedsRowTailGapDp = 8

// sortRailEntries puts each group's feeds in the order the platform's file
// browser sorts names, so a feed added during the session stands where its
// name puts it rather than at the end of its group.
func sortRailEntries(groups []feedGroup) {
	for gi := range groups {
		entries := groups[gi].Entries
		sort.SliceStable(entries, func(i, j int) bool {
			return naming.Less(entries[i].Label, entries[j].Label)
		})
	}
}

// feedsSidebar returns the rail observable: the window's leading chrome
// column, set into the window as a PANE, holding one section per feed group
// and the group's feeds under it.
//
// openSectionsObs streams the current open-section map from the MVU model;
// feedsObs streams the current (mutable) feed tree. The section count is
// fixed — added feeds join an existing group and deletions leave the
// (possibly empty) section in place — so the per-section view state is built
// once and each section renders the CURRENT entries from a per-section atomic
// cell updated by every feedsObs emission. A heading click emits a
// ToggleSection message; entry clicks emit SelectFeed; the hover-revealed
// trash icon opens a per-row delete-confirm popover whose confirm fires
// ConfirmDelete + a toast.
//
// The feed tree lives in the Model, so add/delete mutate it; feedEntryListBody
// reads the live slice each frame and keys its per-entry view state by
// FeedID via components/keyed so add/delete never re-binds a clickable to the
// wrong row.
func feedsSidebar(
	th rx.Observable[theme.Theme],
	openSectionsObs rx.Observable[map[int]bool],
	feedsObs rx.Observable[[]feedGroup],
	selectedFeedObs rx.Observable[FeedID],
	popArb *popover.Arbiter,
) rx.Observable[layout.Widget] {
	// The fixed section count drives the per-section entry cells. The set of
	// groups (titles, count) never changes; only their Entries do.
	groups := hardCodedGroups()
	sectionCells := make([]atomic.Value, len(groups))
	for i := range sectionCells {
		sectionCells[i].Store(groups[i].Entries)
	}

	// The open feed, mirrored for the section bodies. A section's rows are a
	// static layout.Widget slot, so they cannot be handed the selection
	// in-band; they read it from this cell at frame time, the same
	// layer-boundary hand-off the entry list already uses for its entries.
	// The cell is written in the fold below, BEFORE the emitted layout.Widget
	// can be laid out, so no frame paints last selection's pill.
	var selectedCell atomic.Value
	selectedCell.Store(FeedID(""))
	loadSelected := func() FeedID {
		id, _ := selectedCell.Load().(FeedID)
		return id
	}

	// The rail's keyboard, allocated once per subscription — this function
	// body is the window — and read on every frame. Its cursor is what the
	// rows read to say which of them the arrows stand on.
	keys := newRailKeys()

	sections := make([]railSection, len(groups))
	for i, g := range groups {
		cell := &sectionCells[i]
		sections[i] = railSection{
			Title: g.Title,
			Rows: feedEntryRows(th, func() []feedEntry {
				if e, ok := cell.Load().([]feedEntry); ok {
					return e
				}
				return nil
			}, loadSelected, keys, popArb),
		}
	}

	// The heading click targets, one per section, built once: the section
	// count never changes, so a plain slice is stable across every emission.
	headings := make([]gesture.Click, len(groups))

	// The rail's scroll position. A rail taller than the window would
	// otherwise lose its foot, so its column is a scroll area: the state is
	// allocated once per subscription — this function body is the window —
	// and read on every frame.
	railScroll := list.NewState()

	colorsObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.PlatformColors] {
		return t.Platform
	})
	typObs := rx.SwitchMap(th, func(t theme.Theme) rx.Observable[tokens.Typography] {
		return t.Typography
	})
	return rx.Map(
		rx.CombineLatest5(openSectionsObs, feedsObs, colorsObs, selectedFeedObs, typObs),
		func(n rx.Tuple5[map[int]bool, []feedGroup, tokens.PlatformColors, FeedID, tokens.Typography]) layout.Widget {
			open, feeds, c, typ := n.First, n.Second, n.Third, n.Fifth
			selectedCell.Store(n.Fourth)
			for i := range sectionCells {
				if i < len(feeds) {
					sectionCells[i].Store(feeds[i].Entries)
				} else {
					sectionCells[i].Store([]feedEntry(nil))
				}
			}
			// The run the arrows walk, rebuilt on every emission: a
			// collapsed section contributes nothing, so the arrows step
			// straight past it and the pointer alone brings its rows back.
			kb := railKeyboard{Keys: keys, Rows: railRowRun(len(sections), feeds, open), Listing: n.Fourth}
			return func(gtx layout.Context) layout.Dimensions {
				return drawRailColumn(gtx, c, typ, sections, headings, open, railScroll, kb)
			}
		},
	)
}

// railSection is one group as the rail draws it: the heading's name and the
// rows that stand under it while the section is open.
type railSection struct {
	Title string
	Rows  railRowSet
}

// railRowSet is one section's rows as the rail's scrolling column draws
// them: how many the section currently holds, the drawing of one of them,
// and the drain of every row's pointer click.
//
// The rows are handed over one at a time rather than as one layout.Widget
// for the whole run because each is a ROW of the scrolling column: the column
// then brings the cursor's row into view by that row rather than by its
// section, which is the difference between a reader seeing the row the
// arrows walked to and seeing the head of the section holding it.
//
// Drain stands apart from Row for the reason the headings' clicks do: a row
// scrolled out of view lays out nothing, and a click it has already taken
// would be dropped with it.
type railRowSet struct {
	Count func() int
	Row   func(gtx layout.Context, i int) layout.Dimensions
	Drain func(gtx layout.Context)
}

// railRow is one row of the rail as its keyboard walks them: one feed
// standing under an open section.
//
// A section's heading is not a row of the walk. The platform's sidebar never
// stops the arrows on one, so a heading is operated by the pointer alone and
// a section a click collapsed is reopened by a click.
type railRow struct {
	Section int
	// Index is where in its section the row stands, which is railBlock's own
	// convention for the blocks the column scrolls.
	Index int
	// ID is the feed the row opens.
	ID FeedID
}

// railRowRun is the rail's rows in reading order across its sections, which
// is the order the arrows walk them in: the feeds of every open section, one
// section after another. sections is how many the rail draws; a collapsed
// one contributes nothing, and so does a section the feed tree no longer
// carries.
func railRowRun(sections int, feeds []feedGroup, open map[int]bool) []railRow {
	run := make([]railRow, 0, len(feeds))
	for i := range sections {
		if !open[i] || i >= len(feeds) {
			continue
		}
		for j, e := range feeds[i].Entries {
			run = append(run, railRow{Section: i, Index: j, ID: e.ID})
		}
	}
	return run
}

// railKeyboard is what the rail's column answers the keys with: the rail's
// own tag and cursor, which live as long as the window; the run of rows the
// arrows walk, rebuilt on every emission; and the feed the table is listing,
// which is where a reader who has not moved the keys yet starts from.
type railKeyboard struct {
	Keys    *railKeys
	Rows    []railRow
	Listing FeedID
}

// railAnswer is what the rail's keys ask of the window: the feed a key
// opened, and the section a key collapsed.
type railAnswer struct {
	Feed FeedID
	// Opened says Feed carries a feed to open.
	Opened bool
	// Section is the section a key collapsed, -1 when no key touched one.
	Section int
}

// railKeys is the rail's keyboard: the one focus tag the whole rail takes and
// the row the arrows stand on.
//
// One tag for the rail, never one per row — a row a collapsed section does
// not draw has no tag to focus, so per-row tags can only ever reach what is
// on screen. components/list takes a single tag for the same reason, and this
// is that list's shape with the rail's heading blocks kept: the blocks are
// what the column scrolls, the rows are what the keys walk.
//
// The cursor is the feed itself rather than a position, so collapsing a
// section, adding a feed or deleting one never leaves it on a row the reader
// did not walk to. The empty feed is the cursor of a rail the keys have not
// landed on.
type railKeys struct {
	// tag's address is the rail's event tag. The field carries a byte because
	// a zero-size field can share an address with its neighbour, which would
	// break tag identity.
	tag    struct{ _ byte }
	cursor FeedID
	// moved says the last update walked the cursor, which is when the column
	// scrolls to bring the row into view. A wheel is otherwise left alone.
	moved bool
}

func newRailKeys() *railKeys { return &railKeys{} }

// Focus is the rail's keyboard focus tag: the tag a click on a row hands the
// keys to, and the tag the traversal keys are filtered on.
func (k *railKeys) Focus() event.Tag { return &k.tag }

// Select puts the cursor on the feed a click landed on, wherever the run
// carries it. The column does not move: the row was visible enough to be
// clicked.
func (k *railKeys) Select(id FeedID) { k.cursor = id }

// settle keeps the cursor on a row the rail actually shows: the row it
// stands on while the run still carries it, and otherwise the feed the table
// is listing — a reader who puts the keys on the rail without having moved
// them starts from the feed in front of them.
func (k *railKeys) settle(rows []railRow, listing FeedID) {
	if runIndex(rows, k.cursor) >= 0 {
		return
	}
	if runIndex(rows, listing) >= 0 {
		k.cursor = listing
		return
	}
	k.cursor = ""
}

// step walks the cursor to the given feed and records whether the column must
// bring its row into view.
func (k *railKeys) step(id FeedID) {
	if id != k.cursor {
		k.moved = true
	}
	k.cursor = id
}

// update drains the rail's traversal keys and answers what they ask of the
// window. The arrows and Home/End walk the cursor over the rows the open
// sections show, and nothing wraps, as nothing wraps in a platform list.
//
// Left is the platform's sidebar key: it collapses the section the cursor
// stands in and lands on the nearest row still visible. Right answers
// nothing — a section a pointer collapsed is reopened by a pointer, which is
// what the platform's sidebar does. A section's disclosure is model state and
// opening a feed is the caller's semantics, so both are answered here rather
// than acted on.
func (k *railKeys) update(gtx layout.Context, rows []railRow, listing FeedID) railAnswer {
	k.settle(rows, listing)
	k.moved = false
	tag := k.Focus()
	answer := railAnswer{Section: -1}
	// A flipped disclosure leaves a run this frame does not have, so one
	// disclosure a frame is all the outline keys can honestly answer: a
	// second would be answered against the run the first replaced.
	folded := false
	for {
		e, ok := gtx.Event(
			key.FocusFilter{Target: tag},
			key.Filter{Focus: tag, Name: key.NameUpArrow},
			key.Filter{Focus: tag, Name: key.NameDownArrow},
			key.Filter{Focus: tag, Name: key.NameLeftArrow},
			key.Filter{Focus: tag, Name: key.NameHome},
			key.Filter{Focus: tag, Name: key.NameEnd},
			key.Filter{Focus: tag, Name: key.NameReturn},
			key.Filter{Focus: tag, Name: key.NameEnter},
		)
		if !ok {
			return answer
		}
		ke, isKey := e.(key.Event)
		if !isKey || ke.State != key.Press || len(rows) == 0 {
			continue
		}
		at := runIndex(rows, k.cursor)
		switch ke.Name {
		case key.NameUpArrow:
			switch {
			case at < 0:
				at = len(rows) - 1
			case at > 0:
				at--
			}
		case key.NameDownArrow:
			switch {
			case at < 0:
				at = 0
			case at < len(rows)-1:
				at++
			}
		case key.NameHome:
			at = 0
		case key.NameEnd:
			at = len(rows) - 1
		case key.NameLeftArrow:
			if at < 0 || folded {
				continue
			}
			answer.Section, folded = rows[at].Section, true
			k.step(railLanding(rows, rows[at].Section))
			continue
		case key.NameReturn, key.NameEnter:
			if at < 0 {
				continue
			}
			answer.Feed, answer.Opened = rows[at].ID, true
			continue
		default:
			continue
		}
		k.step(rows[at].ID)
	}
}

// railLanding is the feed the cursor lands on when the given section
// collapses: the nearest row the rail still shows, which is the row standing
// above the section's own rows, or the row below them when the section
// stands first. A rail whose only rows were that section's leaves the cursor
// nowhere.
func railLanding(rows []railRow, section int) FeedID {
	for i, r := range rows {
		if r.Section != section {
			continue
		}
		if i > 0 {
			return rows[i-1].ID
		}
		for _, below := range rows[i:] {
			if below.Section != section {
				return below.ID
			}
		}
		return ""
	}
	return ""
}

// runIndex answers where in the run the given feed stands, or -1 when the run
// does not carry it. The empty feed stands nowhere.
func runIndex(rows []railRow, id FeedID) int {
	if id == "" {
		return -1
	}
	for i, r := range rows {
		if r.ID == id {
			return i
		}
	}
	return -1
}

// drawRailColumn lays the panel's own column out: its top strip, and under
// it the sections — each a heading block and, while the section is open, the
// rows beneath it — in a scroll area with the platform's overlay scrollbar.
//
// The strip is the panel's own and not the content's band: it is cut to clear
// the window's control buttons where the window keeps them, which is
// patterns/pane's arithmetic and not this window's. It stands outside the
// scroll area, since the buttons it clears do not move when the reader
// scrolls.
//
// The sections scroll because a rail with every section open is taller than
// the window it stands in, and a column whose foot never comes into view has
// entries the reader cannot open. The bar OVERLAYS rather than reserving a
// gutter: the rail's rows run the panel's full width and the platform floats
// a sidebar's bar over them.
//
// Nothing is drawn in the strip. The window's name is already the navbar's
// brand on the other side of the panel, and this window has neither a
// control that sends the rail away nor an action of the rail's own, so the
// strip stands empty under the window's three control buttons.
func drawRailColumn(
	gtx layout.Context,
	colors tokens.PlatformColors,
	typ tokens.Typography,
	sections []railSection,
	headings []gesture.Click,
	open map[int]bool,
	scroll *list.State,
	kb railKeyboard,
) layout.Dimensions {
	size := gtx.Constraints.Max
	// Every heading answers its click here rather than inside the scroll
	// area: a section scrolled out of view lays out no block at all, and a
	// click it has already taken would be dropped with it.
	for i := range headings {
		for {
			e, ok := headings[i].Update(gtx.Source)
			if !ok {
				break
			}
			if e.Kind == gesture.KindClick {
				mvu.MessageOp{Message: ToggleSection{Idx: i}}.Add(gtx.Ops)
			}
		}
	}
	// The rail's own keys, drained here for the same reason: a row scrolled
	// out of view is not laid out, and the keys are the rail's and not any
	// one row's.
	answer := kb.Keys.update(gtx, kb.Rows, kb.Listing)
	if answer.Opened {
		mvu.MessageOp{Message: SelectFeed{Feed: answer.Feed}}.Add(gtx.Ops)
	}
	if answer.Section >= 0 {
		mvu.MessageOp{Message: ToggleSection{Idx: answer.Section}}.Add(gtx.Ops)
	}
	// And every row's click, for the third time the same reason: a row the
	// column scrolled past lays out nothing this frame, and a click it took
	// before it went would be dropped with it.
	for i := range sections {
		if sections[i].Rows.Drain != nil {
			sections[i].Rows.Drain(gtx)
		}
	}
	strip := min(gtx.Dp(unit.Dp(pane.StripDp)), size.Y)
	if strip >= size.Y {
		return layout.Dimensions{Size: size}
	}
	headH := gtx.Dp(patsidebar.SectionHeight)
	blocks := railBlocks(sections, open)
	// The bar rides the panel's own fill, which is what shows through an
	// overlay thumb.
	bar := scrollbar.FromTokens(colors, pane.Surface(colors))

	stk := op.Offset(image.Pt(0, strip)).Push(gtx.Ops)
	cgtx := gtx
	cgtx.Constraints = layout.Exact(image.Pt(size.X, size.Y-strip))
	// The rail's focus target covers the scrolling column, registered UNDER
	// the rows so a row's own pointer target keeps priority over it.
	area := clip.Rect{Max: cgtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, kb.Keys.Focus())
	area.Pop()
	if kb.Keys.moved {
		if at, ok := railRowBlock(blocks, kb.Rows, kb.Keys.cursor); ok {
			scroll.Reveal(at)
		}
	}
	list.LayoutScrollbar(cgtx, scroll, bar, list.Overlay, blocks,
		func(gtx layout.Context, b railBlock) layout.Dimensions {
			if b.Row >= 0 {
				if sections[b.Section].Rows.Row == nil {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
				}
				return sections[b.Section].Rows.Row(gtx, b.Row)
			}
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, headH))
			return drawRailHeading(gtx, colors, typ, sections[b.Section].Title,
				open[b.Section], &headings[b.Section])
		})
	stk.Pop()
	return layout.Dimensions{Size: size}
}

// railBlock is one block of the rail's scrolling column: a section's heading,
// or one row standing under an open one.
//
// A row is a block of its own, not part of a block holding its section's whole
// run, because the column scrolls by blocks: a run-sized block brings a
// section's HEAD into view when the cursor walks to a row at its foot, which
// is a row the reader cannot see. One block per row makes the cursor's row
// the very thing components/list is asked to reveal, and leaves the rail's
// composition where it was — the blocks stack in the same order at the same
// heights, so the column draws the same pixels.
type railBlock struct {
	Section int
	// Row is where in its section the row stands, or -1 when the block is
	// the section's heading.
	Row int
}

// railBlocks is the column the rail scrolls, in reading order: every
// section's heading, each followed by one block per row while the section
// stands open.
func railBlocks(sections []railSection, open map[int]bool) []railBlock {
	blocks := make([]railBlock, 0, 2*len(sections))
	for i := range sections {
		blocks = append(blocks, railBlock{Section: i, Row: -1})
		if !open[i] || sections[i].Rows.Count == nil {
			continue
		}
		for r := range sections[i].Rows.Count() {
			blocks = append(blocks, railBlock{Section: i, Row: r})
		}
	}
	return blocks
}

// railRowBlock answers which block of the rail's column IS the row the cursor
// stands on, and whether the column holds that row at all.
func railRowBlock(blocks []railBlock, rows []railRow, cursor FeedID) (int, bool) {
	at := runIndex(rows, cursor)
	if at < 0 {
		return 0, false
	}
	row := rows[at]
	for i, b := range blocks {
		if b.Row == row.Index && b.Section == row.Section {
			return i, true
		}
	}
	return 0, false
}

// drawRailHeading draws one section's heading: the group's name as a small
// label in the platform's secondary colour, and at the trailing end the
// control that collapses the rows beneath it. Both are patterns/sidebar's
// drawing, at the metrics measured off the platform's own panel; the whole
// block answers the click, because a heading and the control that collapses
// it are one thing.
//
// The block is not a row. It takes no pill, the rail's selection never rests
// on it, and no line parts it from the rows below — the platform parts a
// section from what stands above it by air alone. Its target is a pointer
// gesture like a row's, so the rail stays one focus target and Tab steps past
// the whole column rather than through its headings.
func drawRailHeading(
	gtx layout.Context,
	colors tokens.PlatformColors,
	typ tokens.Typography,
	title string,
	open bool,
	click *gesture.Click,
) layout.Dimensions {
	size := gtx.Constraints.Max
	fg := vgcolor.Flatten(patsidebar.SectionForeground(colors), pane.Surface(colors))
	mark := patsidebar.PaintDisclosure(gtx, open, size,
		vgcolor.Flatten(patsidebar.DisclosureForeground(colors), pane.Surface(colors)))
	room := size.X - mark.Size.X
	if room < 0 {
		room = 0
	}
	patsidebar.PaintSection(gtx, typ.Shaper(), title, patsidebar.SectionStyle(typ),
		image.Pt(room, size.Y), fg)
	patsidebar.RowTarget(gtx, click, size, title)
	return layout.Dimensions{Size: size}
}

// feedEntryRows returns the rows one rail section holds, each drawn as a row
// of the rail's own scrolling column. entriesFn yields the section's CURRENT
// entries each frame (read from the per-section model cell). Entry clicks
// emit SelectFeed; hovering a row reveals a trash icon whose click toggles a
// per-row delete-confirm popover, whose confirm fires ConfirmDelete + a
// "Feed deleted" toast.
//
// All per-entry view state (the row's pointer target, the trash clickable,
// the confirm clickable, the per-row open flag) is keyed by FeedID so
// add/delete never re-binds state to the wrong row.
func feedEntryRows(
	th rx.Observable[theme.Theme],
	entriesFn func() []feedEntry,
	selectedFn func() FeedID,
	keys *railKeys,
	popArb *popover.Arbiter,
) railRowSet {
	loadTok := mirrorTokens(th)

	// Per-FeedID view state, stable across list mutation.
	rowClicks := keyed.Defer(func(FeedID) *gesture.Click { return &gesture.Click{} })
	trashClicks := keyed.Defer(func(FeedID) *widget.Clickable { return &widget.Clickable{} })
	confirmClicks := keyed.Defer(func(FeedID) *widget.Clickable { return &widget.Clickable{} })
	// hover is per-row pointer hover state; ephemeral, lives in the closure.
	// gesture.Hover only filters Enter/Leave (never Press), so it does NOT
	// swallow the select press the way a full-row widget.Clickable would —
	// click-to-select stays intact under the hover-reveal trash gutter.
	hovers := keyed.Defer(func(FeedID) *gesture.Hover { return &gesture.Hover{} })
	// The per-row delete-confirm popovers. Each holds its open flag as a
	// plain bool on the frame goroutine — ephemeral interaction state, not
	// model state, keyed by FeedID — and patterns/popover reads it during
	// layout through Props.OpenNow. They all share this window's Arbiter, so
	// opening one row's confirm dismisses whichever row had it open.
	popovers := keyed.Defer(func(id FeedID) *deleteConfirm {
		return newDeleteConfirm(th, id, trashClicks.For(id), confirmClicks.For(id), popArb)
	})

	return railRowSet{
		Count: func() int { return len(entriesFn()) },
		Drain: func(gtx layout.Context) {
			for _, e := range entriesFn() {
				rc := rowClicks.For(e.ID)
				for {
					ev, ok := rc.Update(gtx.Source)
					if !ok {
						break
					}
					if ev.Kind != gesture.KindClick {
						continue
					}
					keys.Select(e.ID)
					// A row's target is a pointer gesture and takes no
					// keyboard of its own, so the click asks for the rail's:
					// the arrows then walk the rail from the row the reader
					// landed on, and that row wears the accent pill.
					gtx.Execute(key.FocusCmd{Tag: keys.Focus()})
					mvu.MessageOp{Message: SelectFeed{Feed: e.ID}}.Add(gtx.Ops)
				}
			}
		},
		Row: func(gtx layout.Context, i int) layout.Dimensions {
			entries := entriesFn()
			rowH := gtx.Dp(patsidebar.RowHeight)
			if i < 0 || i >= len(entries) {
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
			}
			e := entries[i]
			// The accent pill goes where the platform puts it: on the row the
			// keys stand on, while the rail holds them. The open feed — the
			// one whose articles the table is listing — keeps its own pill in
			// the grey state, which is what the rail shows while the keys are
			// in the table beside it. The rail is one focus target, so its own
			// tag is the whole answer to which of the two a row wears.
			onCursor := gtx.Focused(keys.Focus()) && e.ID == keys.cursor
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, rowH))
			return drawFeedEntryRow(gtx, loadTok(), e, e.ID == selectedFn() || onCursor, !onCursor,
				rowClicks.For(e.ID), hovers.For(e.ID), popovers.For(e.ID),
				gtx.Dp(unit.Dp(trashColWDp)))
		},
	}
}

// drawFeedEntryRow paints one feed row: the selected feed's pill (under
// everything), then the three parts the platform draws a sidebar row from —
// the symbol, the name, and at the trailing end the count of the feed's
// unread articles. A hover-revealed trash icon and its delete-confirm
// popover take that trailing column while the pointer is on the row; hover
// holds the row's pointer state, and the icon paints only while hovered (or
// while its confirm popover is open, so the popover never floats over an
// un-hovered row).
//
// The count and the trash never stand together: one thing stands at a row's
// trailing end, and while the pointer is on the row the thing it can operate
// is the one worth showing.
//
// A row takes a fill when it is the open feed — the one whose articles the
// table is listing — or the row the rail's keys stand on, and the fill is
// patterns/sidebar's pill. A sidebar row does not change under the pointer on
// this platform, which the stored reference captures measure, so the hover
// state reveals the trash gutter and paints nothing.
func drawFeedEntryRow(
	gtx layout.Context,
	tok themeTokens,
	e feedEntry,
	filled bool,
	unemphasized bool,
	click *gesture.Click,
	hover *gesture.Hover,
	dc *deleteConfirm,
	trashW int,
) layout.Dimensions {
	size := gtx.Constraints.Max

	// Hover tracking spans the whole row but registers hover Enter/Leave only
	// (gesture.Hover), so it never claims the select press. Register the hover
	// area first so it sits under the label/trash content.
	hovered := hover.Update(gtx.Source) || dc.open
	hoverClip := clip.Rect{Max: size}.Push(gtx.Ops)
	hover.Add(gtx.Ops)
	hoverClip.Pop()

	surface := tok.col.SidebarMaterial
	if filled {
		patsidebar.PaintSelection(gtx, size, tok.col, unemphasized)
		surface = patsidebar.SelectionFill(tok.col, unemphasized)
	}

	// The row's parts stand in the rail's own columns, measured off the
	// platform's panel, not against the pill: the symbol at SymbolInset, the
	// name at LabelInset and the count CountInset in from the trailing edge.
	drawFeedSymbol(gtx, size,
		vgcolor.Flatten(patsidebar.SymbolForeground(tok.col, filled, unemphasized), surface))

	trail := gtx.Dp(patsidebar.CountInset)
	tailW := 0
	switch {
	case hovered:
		// Trash gutter + confirm popover, in the count's own column.
		trX := size.X - trail - trashW
		if trX < 0 {
			trX = 0
		}
		trStk := op.Offset(image.Pt(trX, 0)).Push(gtx.Ops)
		trGtx := gtx
		trGtx.Constraints = layout.Exact(image.Pt(trashW, size.Y))
		dc.layout(trGtx, true)
		trStk.Pop()
		tailW = trashW
	case e.Unread > 0:
		tailW = patsidebar.PaintCount(gtx, tok.shaper, strconv.Itoa(e.Unread),
			tok.typ.BodySmall, size,
			vgcolor.Flatten(patsidebar.CountForeground(tok.col, filled, unemphasized), surface))
	}

	// The name, and with it the SelectFeed click target: it runs from the
	// row's leading edge to whatever the trailing end is spending, so the
	// symbol is part of what the reader clicks.
	gap := 0
	if tailW > 0 {
		gap = gtx.Dp(unit.Dp(feedsRowTailGapDp))
	}
	labelW := size.X - trail - tailW - gap
	if labelW < 0 {
		labelW = 0
	}
	labelGtx := gtx
	labelGtx.Constraints = layout.Exact(image.Pt(labelW, size.Y))
	drawFeedEntry(labelGtx, tok, e.Label, filled, unemphasized, click)

	return layout.Dimensions{Size: size}
}

// drawFeedSymbol paints a feed row's symbol in the square the sidebar's rows
// keep for one, at the column the platform draws it in.
//
// The mark is the document: the icon set carries no mark that says a feed,
// and a feed is one piece of content this window lists, the way a note is.
func drawFeedSymbol(gtx layout.Context, size image.Point, fg color.NRGBA) {
	// The rail's own painter puts the mark in the rail's own column and fills
	// the square with it, which is what brings it out at the platform's own
	// weight beside the name. The offset arithmetic is stated once, in
	// patterns/sidebar.
	patsidebar.PaintSymbol(gtx, icons.Mark(icons.Document), size, fg)
}

// feedRowForeground is what a feed row's marks wear: the foreground the
// platform pairs with the selection pill on the open feed, and the ordinary
// label on the chrome material everywhere else. Both are flattened onto the
// fill they actually land on.
func feedRowForeground(c tokens.PlatformColors, selected, unemphasized bool) color.NRGBA {
	if selected {
		return vgcolor.Flatten(patsidebar.SelectionLabel(c, unemphasized), patsidebar.SelectionFill(c, unemphasized))
	}
	return vgcolor.Flatten(c.Label, c.SidebarMaterial)
}

// drawFeedEntry lays the row's own click target out and draws the feed's
// name in it, starting at the rail's measured label column.
func drawFeedEntry(
	gtx layout.Context,
	tok themeTokens,
	label string,
	selected bool,
	unemphasized bool,
	click *gesture.Click,
) layout.Dimensions {
	size := gtx.Constraints.Max
	inner := func(gtx layout.Context) layout.Dimensions {
		lead := gtx.Dp(patsidebar.LabelInset)
		room := size.X - lead
		if room < 0 {
			room = 0
		}
		labelGtx := gtx
		labelGtx.Constraints.Min = image.Point{}
		labelGtx.Constraints.Max = image.Pt(room, size.Y)
		mLabel := op.Record(gtx.Ops)
		labelDims := drawLabel(labelGtx, tok.shaper, label, tok.typ.BodySmall, feedRowForeground(tok.col, selected, unemphasized))
		labelCall := mLabel.Stop()
		offY := (size.Y - labelDims.Size.Y) / 2
		if offY < 0 {
			offY = 0
		}
		stk := op.Offset(image.Pt(lead, offY)).Push(gtx.Ops)
		labelCall.Add(gtx.Ops)
		stk.Pop()
		return layout.Dimensions{Size: size}
	}
	gtx.Constraints = layout.Exact(size)
	inner(gtx)
	patsidebar.RowTarget(gtx, click, size, label)
	return layout.Dimensions{Size: size}
}

// deleteConfirm owns one feed row's delete-confirm popover: the trash-icon
// anchor and a "Delete this feed?" confirm surface. Open state is ephemeral
// per-row interaction state — a plain bool this struct owns, written and read
// during layout on the frame goroutine, which patterns/popover reads back
// through Props.OpenNow. Nothing outside the frame ever asks whether a row's
// confirm is open, so nothing outside the frame holds a copy of the answer.
// The trash click toggles it; the confirm click fires ConfirmDelete + a toast
// and closes; OnDismiss closes. The remaining atomic cell carries the THEME's
// re-emissions, which do arrive from another goroutine.
//
// The popover is wrapped in an Exact space (the trash gutter) so its anchor
// centres on the trash icon and the confirm surface sits below it — the same
// anchor-to-space coupling as the Share popover.
type deleteConfirm struct {
	id FeedID
	// open is frame state: only layout writes it and only layout reads it.
	open bool
	cell atomic.Value // latest popover layout.Widget
}

func newDeleteConfirm(
	th rx.Observable[theme.Theme],
	id FeedID,
	trashClick *widget.Clickable,
	confirmClick *widget.Clickable,
	popArb *popover.Arbiter,
) *deleteConfirm {
	dc := &deleteConfirm{id: id}

	loadTok := mirrorTokens(th)

	anchor := func(gtx layout.Context) layout.Dimensions {
		if trashClick.Clicked(gtx) {
			dc.toggle()
		}
		return trashClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			s := loadTok()
			semantic.LabelOp("Delete feed").Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			sz := gtx.Constraints.Max
			pointershape.OverSize(gtx.Ops, sz, pointer.CursorPointer)
			drawTrashIcon(gtx, sz, feedRowForeground(s.col, false, false))
			return layout.Dimensions{Size: sz}
		})
	}

	content := func(gtx layout.Context) layout.Dimensions {
		s := loadTok()
		if confirmClick.Clicked(gtx) {
			notifications.Notify(gtx, toast.Success, "Feed deleted")
			mvu.MessageOp{Message: ConfirmDelete{Feed: dc.id}}.Add(gtx.Ops)
			dc.close()
		}
		// Override the incoming half-space constraints: the popover sized
		// the anchor space to the tiny trash gutter, so half of it cannot
		// hold a confirm prompt. Size the content ourselves; popover pads it.
		w := gtx.Dp(unit.Dp(deleteConfirmWDp))
		promptH := gtx.Dp(unit.Dp(deleteConfirmRowHDp))
		btnH := gtx.Dp(unit.Dp(deleteConfirmRowHDp))
		drawLabel(gtx, s.shaper, "Delete this feed?", s.typ.BodyMedium,
			vgcolor.Flatten(s.col.Label, s.col.WindowBackground))
		btnStk := op.Offset(image.Pt(0, promptH)).Push(gtx.Ops)
		btnGtx := gtx
		btnGtx.Constraints = layout.Exact(image.Pt(w, btnH))
		confirmClick.Layout(btnGtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp("Confirm delete").Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointershape.OverSize(gtx.Ops, image.Pt(w, btnH), pointer.CursorPointer)
			drawLabel(gtx, s.shaper, "Delete", s.typ.LabelLarge, s.col.SystemRed)
			return layout.Dimensions{Size: image.Pt(w, btnH)}
		})
		btnStk.Pop()
		return layout.Dimensions{Size: image.Pt(w, promptH+btnH)}
	}

	popObs := popover.Popover(th, popover.Props{
		OpenNow:   func() bool { return dc.open },
		Anchor:    anchor,
		Content:   content,
		Placement: popover.Bottom,
		Arbiter:   popArb,
		OnDismiss: func(layout.Context) { dc.close() },
	})
	dc.cell.Store(layout.Widget(nil))
	_ = popObs.Subscribe(rx.GoroutineContext(), func(w layout.Widget, _ error, done bool) {
		if !done && w != nil {
			dc.cell.Store(w)
		}
	})
	return dc
}

// toggle and close run during layout, from the anchor click, the confirm
// click and the arbiter's OnDismiss — all three on the frame goroutine, which
// is what lets open be a plain bool.
func (dc *deleteConfirm) toggle() { dc.open = !dc.open }

func (dc *deleteConfirm) close() { dc.open = false }

// layout draws the trash gutter for one row. When the row is not hovered (and
// the confirm popover is closed), nothing is painted and the gutter is inert.
// When hovered/open the popover renders the trash anchor (and, while open,
// the confirm surface) inside the gutter's Exact space.
func (dc *deleteConfirm) layout(gtx layout.Context, visible bool) layout.Dimensions {
	size := gtx.Constraints.Max
	if !visible {
		return layout.Dimensions{Size: size}
	}
	if w, ok := dc.cell.Load().(layout.Widget); ok && w != nil {
		w(gtx)
	}
	return layout.Dimensions{Size: size}
}

const (
	deleteConfirmWDp    = 132
	deleteConfirmRowHDp = 28
)

// drawTrashIcon paints a minimal trash glyph (a lid line + a body box) into a
// square the size of the gutter, centred, in colour col. clip.Path/Stroke
// only, so it stays golden-deterministic like the other feeds glyphs.
func drawTrashIcon(gtx layout.Context, box image.Point, col color.NRGBA) {
	side := box.X
	if box.Y < side {
		side = box.Y
	}
	pad := gtx.Dp(unit.Dp(6))
	x0 := (box.X-side)/2 + pad
	x1 := box.X - (box.X-side)/2 - pad
	y0 := (box.Y-side)/2 + pad
	y1 := box.Y - (box.Y-side)/2 - pad
	stroke := float32(gtx.Dp(unit.Dp(1)))
	if stroke < 1 {
		stroke = 1
	}
	lidY := y0 + (y1-y0)/5
	// Lid line.
	rect(gtx, image.Rect(x0, lidY, x1, lidY+int(stroke)+1), col)
	// Body outline (four thin rects).
	rect(gtx, image.Rect(x0, lidY, x0+int(stroke)+1, y1), col)
	rect(gtx, image.Rect(x1-int(stroke)-1, lidY, x1, y1), col)
	rect(gtx, image.Rect(x0, y1-int(stroke)-1, x1, y1), col)
}

func rect(gtx layout.Context, r image.Rectangle, col color.NRGBA) {
	paint.FillShape(gtx.Ops, col, clip.Rect(r).Op())
}
