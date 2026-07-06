package main

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/cadence/table"
	"github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/prism/tokens"
)

var goldenUpdate = flag.Bool("golden.update", false, "overwrite golden images with current output")

// TestBuildLayersConstructsWithoutPanic mirrors the G5.1a smoke test
// pattern: drives buildLayers with a seeded model and a single-shot theme
// observable, collects one widget emission from each layer, and fails if any
// subscription panics or completes with an error.
func TestBuildLayersConstructsWithoutPanic(t *testing.T) {
	layers := buildLayers(rx.Of(initialModel()))(rx.Of(theme.Default()))
	if len(layers) != 2 {
		t.Fatalf("buildLayers returned %d layers; want 2 (backdrop, shell)", len(layers))
	}
	for i, layer := range layers {
		got, err := collectOne(layer)
		if err != nil {
			t.Errorf("layer %d subscribe: %v", i, err)
			continue
		}
		if got == nil {
			t.Errorf("layer %d produced no widget", i)
		}
	}
}

// TestInitialModelSeeds verifies the seed state the former rx.Subject
// controllers guaranteed: the default feed selected, page 1, Published-desc
// sort, and the first accordion section open.
func TestInitialModelSeeds(t *testing.T) {
	m := initialModel()
	if m.selectedFeed != defaultFeedID() {
		t.Errorf("initialModel.selectedFeed = %q; want %q", m.selectedFeed, defaultFeedID())
	}
	if m.currentPage != 1 {
		t.Errorf("initialModel.currentPage = %d; want 1", m.currentPage)
	}
	if m.sort != (table.Sort{Column: colPublished, Asc: false}) {
		t.Errorf("initialModel.sort = %+v; want Published-desc", m.sort)
	}
	if !m.openSections[0] {
		t.Errorf("initialModel.openSections[0] = false; want true (first section seeded open)")
	}
}

// TestUpdateSelectFeedResetsPage verifies SelectFeed advances the selected
// feed and resets the table to page 1 so the user never lands on an
// out-of-range slice for the new feed.
func TestUpdateSelectFeedResetsPage(t *testing.T) {
	m := initialModel()
	m, _ = Update(m, SetPage{Page: 5})
	if m.currentPage != 5 {
		t.Fatalf("precondition: SetPage(5) -> currentPage = %d; want 5", m.currentPage)
	}
	m, _ = Update(m, SelectFeed{Feed: "bbc"})
	if m.selectedFeed != "bbc" {
		t.Errorf("after SelectFeed: selectedFeed = %q; want bbc", m.selectedFeed)
	}
	if m.currentPage != 1 {
		t.Errorf("after SelectFeed: currentPage = %d; want 1 (reset)", m.currentPage)
	}
}

// TestUpdateSelectArticle verifies SelectArticle records the clicked row.
func TestUpdateSelectArticle(t *testing.T) {
	m, _ := Update(initialModel(), SelectArticle{Article: "hn-03"})
	if m.selectedArticle != "hn-03" {
		t.Errorf("after SelectArticle: selectedArticle = %q; want hn-03", m.selectedArticle)
	}
}

// TestUpdateSetPageAndSort verifies SetPage and SetSort advance their fields.
func TestUpdateSetPageAndSort(t *testing.T) {
	m, _ := Update(initialModel(), SetPage{Page: 3})
	if m.currentPage != 3 {
		t.Errorf("after SetPage(3): currentPage = %d; want 3", m.currentPage)
	}
	want := table.Sort{Column: colTitle, Asc: true}
	m, _ = Update(m, SetSort{Sort: want})
	if m.sort != want {
		t.Errorf("after SetSort: sort = %+v; want %+v", m.sort, want)
	}
}

// TestUpdateToggleSectionSingleOpen verifies the single-open reducer policy:
// opening a section replaces the open set with just that index, and clicking
// the already-open section collapses it.
func TestUpdateToggleSectionSingleOpen(t *testing.T) {
	m := initialModel() // section 0 is open
	if !m.openSections[0] {
		t.Fatal("precondition: section 0 must start open")
	}
	m, _ = Update(m, ToggleSection{Idx: 1})
	if !m.openSections[1] {
		t.Error("after ToggleSection(1): section 1 should be open")
	}
	if m.openSections[0] {
		t.Error("after ToggleSection(1): section 0 should have closed (single-open)")
	}
	m, _ = Update(m, ToggleSection{Idx: 1})
	if m.openSections[1] {
		t.Error("after second ToggleSection(1): section 1 should be closed")
	}
	if len(m.openSections) != 0 {
		t.Errorf("expected all sections closed; got %v", m.openSections)
	}
}

// TestUpdateSelectTab verifies SelectTab advances the detail pane's tab
// index, including out-of-range values (stored as-is; cadence/tabs renders
// them as "no tab selected" per its documented contract).
func TestUpdateSelectTab(t *testing.T) {
	m := initialModel()
	if m.selectedTab != tabReader {
		t.Fatalf("initialModel.selectedTab = %d; want %d (Reader)", m.selectedTab, tabReader)
	}
	m, _ = Update(m, SelectTab{Idx: tabRaw})
	if m.selectedTab != tabRaw {
		t.Errorf("after SelectTab(Raw): selectedTab = %d; want %d", m.selectedTab, tabRaw)
	}
	m, _ = Update(m, SelectTab{Idx: 99})
	if m.selectedTab != 99 {
		t.Errorf("after SelectTab(99): selectedTab = %d; want 99 (stored as-is)", m.selectedTab)
	}
}

// TestUpdateShareOpenClose verifies the ToggleShare flip and that CloseShare
// is idempotent when the popover is already closed.
func TestUpdateShareOpenClose(t *testing.T) {
	m := initialModel()
	if m.shareOpen {
		t.Fatal("initialModel.shareOpen = true; want false (popover starts closed)")
	}
	m, _ = Update(m, ToggleShare{})
	if !m.shareOpen {
		t.Error("after ToggleShare: shareOpen = false; want true")
	}
	m, _ = Update(m, ToggleShare{})
	if m.shareOpen {
		t.Error("after second ToggleShare: shareOpen = true; want false")
	}
	m, _ = Update(m, CloseShare{})
	if m.shareOpen {
		t.Error("CloseShare on a closed popover flipped it open; want idempotent false")
	}
	m, _ = Update(m, ToggleShare{})
	m, _ = Update(m, CloseShare{})
	if m.shareOpen {
		t.Error("after ToggleShare then CloseShare: shareOpen = true; want false")
	}
}

// TestUpdateSetSplitRatio verifies the divider position lands in the model.
func TestUpdateSetSplitRatio(t *testing.T) {
	m := initialModel()
	if m.splitRatio != initialSplitRatio {
		t.Fatalf("initialModel.splitRatio = %v; want %v", m.splitRatio, initialSplitRatio)
	}
	m, _ = Update(m, SetSplitRatio{Ratio: 0.42})
	if m.splitRatio != 0.42 {
		t.Errorf("after SetSplitRatio(0.42): splitRatio = %v; want 0.42", m.splitRatio)
	}
}

// ----- G5.2d CRUD reducer tests -----

// feedCount counts every feed entry across all groups in a model.
func feedCount(m Model) int {
	n := 0
	for _, g := range m.feeds {
		n += len(g.Entries)
	}
	return n
}

func hasFeed(m Model, id FeedID) bool {
	for _, g := range m.feeds {
		for _, e := range g.Entries {
			if e.ID == id {
				return true
			}
		}
	}
	return false
}

// TestUpdateOpenCloseAddFeed verifies the modal open flag and that opening
// clears a stale alert.
func TestUpdateOpenCloseAddFeed(t *testing.T) {
	m := initialModel()
	if m.addFeedOpen {
		t.Fatal("initialModel.addFeedOpen = true; want false")
	}
	m, _ = Update(m, SubmitFeed{URL: ""}) // raise the alert
	if !m.addFeedError {
		t.Fatal("precondition: empty submit should set addFeedError")
	}
	m, _ = Update(m, OpenAddFeed{})
	if !m.addFeedOpen {
		t.Error("after OpenAddFeed: addFeedOpen = false; want true")
	}
	if m.addFeedError {
		t.Error("OpenAddFeed should clear a stale addFeedError")
	}
	m, _ = Update(m, CloseAddFeed{})
	if m.addFeedOpen || m.addFeedError {
		t.Errorf("after CloseAddFeed: open=%v error=%v; want both false", m.addFeedOpen, m.addFeedError)
	}
}

// TestUpdateSubmitFeedEmptyRaisesAlert verifies an empty URL raises the modal
// alert, appends nothing, and leaves the modal open.
func TestUpdateSubmitFeedEmptyRaisesAlert(t *testing.T) {
	m := initialModel()
	m, _ = Update(m, OpenAddFeed{})
	before := feedCount(m)
	m, _ = Update(m, SubmitFeed{URL: "   "}) // whitespace-only is empty
	if !m.addFeedError {
		t.Error("empty submit did not set addFeedError")
	}
	if !m.addFeedOpen {
		t.Error("empty submit closed the modal; it should stay open")
	}
	if got := feedCount(m); got != before {
		t.Errorf("empty submit changed feed count %d -> %d; want unchanged", before, got)
	}
}

// TestUpdateSubmitFeedNonEmptyAppends verifies a non-empty URL appends a feed,
// clears the alert, and closes the modal.
func TestUpdateSubmitFeedNonEmptyAppends(t *testing.T) {
	m := initialModel()
	m, _ = Update(m, OpenAddFeed{})
	before := feedCount(m)
	m, _ = Update(m, SubmitFeed{URL: "https://example.com/feed.xml"})
	if got := feedCount(m); got != before+1 {
		t.Errorf("non-empty submit feed count %d -> %d; want +1", before, got)
	}
	if m.addFeedOpen {
		t.Error("non-empty submit left the modal open; it should close")
	}
	if m.addFeedError {
		t.Error("non-empty submit left addFeedError set")
	}
	if !hasFeed(m, FeedID("added:https://example.com/feed.xml")) {
		t.Error("appended feed not found in model")
	}
}

// TestUpdateConfirmDeleteRemovesFeed verifies a delete removes the entry and
// leaves the (fixed) group count intact.
func TestUpdateConfirmDeleteRemovesFeed(t *testing.T) {
	m := initialModel()
	groupsBefore := len(m.feeds)
	if !hasFeed(m, "hn") {
		t.Fatal("precondition: fixture must contain feed hn")
	}
	before := feedCount(m)
	m, _ = Update(m, ConfirmDelete{Feed: "hn"})
	if hasFeed(m, "hn") {
		t.Error("ConfirmDelete did not remove feed hn")
	}
	if got := feedCount(m); got != before-1 {
		t.Errorf("delete feed count %d -> %d; want -1", before, got)
	}
	if len(m.feeds) != groupsBefore {
		t.Errorf("group count changed %d -> %d; delete should leave sections in place", groupsBefore, len(m.feeds))
	}
}

// TestUpdateConfirmDeleteSelectedReselects verifies deleting the SELECTED feed
// falls selection back to the first remaining feed and resets the page.
func TestUpdateConfirmDeleteSelectedReselects(t *testing.T) {
	m := initialModel()
	sel := m.selectedFeed // default = first feed of first group
	m, _ = Update(m, SetPage{Page: 4})
	m, _ = Update(m, ConfirmDelete{Feed: sel})
	if m.selectedFeed == sel {
		t.Errorf("selection stayed on deleted feed %q", sel)
	}
	if m.selectedFeed != firstFeedID(m.feeds) {
		t.Errorf("after deleting selected: selectedFeed = %q; want first remaining %q", m.selectedFeed, firstFeedID(m.feeds))
	}
	if m.currentPage != 1 {
		t.Errorf("after deleting selected: currentPage = %d; want 1 (reset)", m.currentPage)
	}
}

// TestUpdateConfirmDeleteNonSelectedKeepsSelection verifies deleting a feed
// other than the selected one leaves selection untouched.
func TestUpdateConfirmDeleteNonSelectedKeepsSelection(t *testing.T) {
	m := initialModel()
	sel := m.selectedFeed
	var other FeedID
	for _, g := range m.feeds {
		for _, e := range g.Entries {
			if e.ID != sel {
				other = e.ID
			}
		}
	}
	if other == "" {
		t.Skip("fixture has only one feed")
	}
	m, _ = Update(m, ConfirmDelete{Feed: other})
	if m.selectedFeed != sel {
		t.Errorf("deleting non-selected feed changed selection %q -> %q", sel, m.selectedFeed)
	}
}

// TestReducerDoesNotAliasPreviousModelFeeds verifies the append/delete
// reducers copy the feed slice rather than mutating the previous Model's
// slice in place (the MVU contract: Update is pure).
func TestReducerDoesNotAliasPreviousModelFeeds(t *testing.T) {
	m0 := initialModel()
	before := feedCount(m0)
	_, _ = Update(m0, SubmitFeed{URL: "https://x.test/a"})
	if got := feedCount(m0); got != before {
		t.Errorf("SubmitFeed mutated the previous model's feeds (%d -> %d)", before, got)
	}
	_, _ = Update(m0, ConfirmDelete{Feed: "hn"})
	if got := feedCount(m0); got != before {
		t.Errorf("ConfirmDelete mutated the previous model's feeds (%d -> %d)", before, got)
	}
}

// TestArticleByID verifies the detail-pane lookup over the fixture: a known
// ID resolves to its article, an unknown ID reports ok=false.
func TestArticleByID(t *testing.T) {
	all := hardCodedArticles()
	a, ok := articleByID(all[0].ID)
	if !ok {
		t.Fatalf("articleByID(%q) reported ok=false for a fixture row", all[0].ID)
	}
	if a.Title != all[0].Title {
		t.Errorf("articleByID(%q).Title = %q; want %q", all[0].ID, a.Title, all[0].Title)
	}
	if _, ok := articleByID("no-such-article"); ok {
		t.Error("articleByID(no-such-article) reported ok=true; want false")
	}
}

// TestHardCodedBodyVariesByArticle verifies the Reader/Raw body fixture
// interpolates the article (so switching selection visibly changes the
// pane) and carries the paragraph breaks the Reader tab promises to wrap.
func TestHardCodedBodyVariesByArticle(t *testing.T) {
	all := hardCodedArticles()
	b0 := hardCodedBody(all[0])
	b1 := hardCodedBody(all[1])
	if b0 == b1 {
		t.Error("hardCodedBody returned identical text for two different articles")
	}
	if !strings.Contains(b0, all[0].Title) {
		t.Errorf("hardCodedBody does not contain the article title %q", all[0].Title)
	}
	if !strings.Contains(b0, "\n\n") {
		t.Error("hardCodedBody has no paragraph breaks; Reader-tab wrapping is untestable")
	}
}

// TestHardCodedComments verifies the Comments-tab placeholder fixture is
// non-empty with non-empty rows.
func TestHardCodedComments(t *testing.T) {
	cs := hardCodedComments()
	if len(cs) == 0 {
		t.Fatal("hardCodedComments returned no rows")
	}
	for i, c := range cs {
		if c.Author == "" || c.Text == "" {
			t.Errorf("comment[%d] has empty Author or Text: %+v", i, c)
		}
	}
}

// TestHardCodedGroups verifies the fixture shape expected by the sidebar.
func TestHardCodedGroups(t *testing.T) {
	groups := hardCodedGroups()
	if len(groups) != 3 {
		t.Fatalf("want 3 groups, got %d", len(groups))
	}
	names := []string{"Tech", "News", "Personal"}
	for i, g := range groups {
		if g.Title != names[i] {
			t.Errorf("group[%d]: want %q, got %q", i, names[i], g.Title)
		}
		if len(g.Entries) == 0 {
			t.Errorf("group[%d] %q has no entries", i, g.Title)
		}
	}
}

// TestFeedIDsDistinct verifies no duplicate FeedID values in the fixture.
func TestFeedIDsDistinct(t *testing.T) {
	seen := map[FeedID]bool{}
	for _, g := range hardCodedGroups() {
		for _, e := range g.Entries {
			if seen[e.ID] {
				t.Errorf("duplicate FeedID %q", e.ID)
			}
			seen[e.ID] = true
		}
	}
}

// TestArticleFixtureSizeAndUniqueness enforces the G5.2b "≥80 article
// rows" guarantee and confirms ArticleIDs are unique across the fixture.
func TestArticleFixtureSizeAndUniqueness(t *testing.T) {
	arts := hardCodedArticles()
	if len(arts) < 80 {
		t.Fatalf("hardCodedArticles returned %d rows; want ≥80", len(arts))
	}
	feedIDs := map[FeedID]bool{}
	for _, g := range hardCodedGroups() {
		for _, e := range g.Entries {
			feedIDs[e.ID] = true
		}
	}
	seen := map[ArticleID]bool{}
	for _, a := range arts {
		if !feedIDs[a.FeedID] {
			t.Errorf("article %q references unknown feed %q", a.ID, a.FeedID)
		}
		if seen[a.ID] {
			t.Errorf("duplicate ArticleID %q", a.ID)
		}
		seen[a.ID] = true
	}
}

// TestFilterAndSortArticlesFiltersByFeed confirms the pure transform
// drops rows that do not belong to the requested feed.
func TestFilterAndSortArticlesFiltersByFeed(t *testing.T) {
	got := filterAndSortArticles(hardCodedArticles(), "go-blog", "", table.Sort{Column: -1})
	if len(got) == 0 {
		t.Fatal("no go-blog articles returned")
	}
	for _, a := range got {
		if a.FeedID != "go-blog" {
			t.Errorf("got article for feed %q; want only go-blog", a.FeedID)
		}
	}
}

// TestFilterAndSortArticlesFiltersByQuery confirms the lower-cased
// substring match runs against Title and Author together.
func TestFilterAndSortArticlesFiltersByQuery(t *testing.T) {
	got := filterAndSortArticles(hardCodedArticles(), "go-blog", "Generics", table.Sort{Column: -1})
	if len(got) == 0 {
		t.Fatal("expected at least one match for query 'Generics' in go-blog")
	}
	for _, a := range got {
		if !strings.Contains(strings.ToLower(a.Title), "generics") &&
			!strings.Contains(strings.ToLower(a.Author), "generics") {
			t.Errorf("article %q matched neither Title nor Author against 'Generics'", a.Title)
		}
	}
}

// TestFilterAndSortArticlesSortsByPublished confirms the Sortable
// Published column produces a descending order with Asc=false and an
// ascending order with Asc=true.
func TestFilterAndSortArticlesSortsByPublished(t *testing.T) {
	all := hardCodedArticles()
	desc := filterAndSortArticles(all, "hn", "", table.Sort{Column: colPublished, Asc: false})
	if len(desc) < 2 {
		t.Skip("hn fixture too small to test ordering")
	}
	for i := 1; i < len(desc); i++ {
		if desc[i-1].Published.Before(desc[i].Published) {
			t.Errorf("descending sort violated at %d: %v before %v", i, desc[i-1].Published, desc[i].Published)
		}
	}
	asc := filterAndSortArticles(all, "hn", "", table.Sort{Column: colPublished, Asc: true})
	for i := 1; i < len(asc); i++ {
		if asc[i-1].Published.After(asc[i].Published) {
			t.Errorf("ascending sort violated at %d: %v after %v", i, asc[i-1].Published, asc[i].Published)
		}
	}
}

// TestFilterAndSortArticlesSortsByTitle confirms the Sortable Title
// column produces case-insensitive Asc/Desc ordering.
func TestFilterAndSortArticlesSortsByTitle(t *testing.T) {
	asc := filterAndSortArticles(hardCodedArticles(), "lobsters", "", table.Sort{Column: colTitle, Asc: true})
	for i := 1; i < len(asc); i++ {
		if strings.ToLower(asc[i-1].Title) > strings.ToLower(asc[i].Title) {
			t.Errorf("title asc violated at %d: %q > %q", i, asc[i-1].Title, asc[i].Title)
		}
	}
}

// TestPageSliceBounds covers the three page-slice cases: in-range,
// past-the-end (returns nil), and partial last page.
func TestPageSliceBounds(t *testing.T) {
	arts := make([]article, 25)
	for i := range arts {
		arts[i].ID = ArticleID(strings.Repeat("x", i+1))
	}
	if got := pageSlice(arts, 1, 10); len(got) != 10 {
		t.Errorf("page 1: got %d; want 10", len(got))
	}
	if got := pageSlice(arts, 3, 10); len(got) != 5 {
		t.Errorf("page 3 (partial): got %d; want 5", len(got))
	}
	if got := pageSlice(arts, 4, 10); got != nil {
		t.Errorf("page 4 (past end): got %d rows; want nil", len(got))
	}
}

// TestPageCountForRoundsUp confirms ceiling division and the 1-floor.
func TestPageCountForRoundsUp(t *testing.T) {
	cases := []struct {
		n, size, want int
	}{
		{0, 10, 1},
		{1, 10, 1},
		{10, 10, 1},
		{11, 10, 2},
		{84, 10, 9},
	}
	for _, c := range cases {
		arts := make([]article, c.n)
		if got := pageCountFor(arts, c.size); got != c.want {
			t.Errorf("pageCountFor(%d, %d) = %d; want %d", c.n, c.size, got, c.want)
		}
	}
}

// TestArticlesPipelineFiltersByFeed exercises the load-bearing rx chain
// CombineLatest3(selectedFeed, filter, sort) → filterAndSortArticles → Items.
// A regression in this wiring (wrong Subject buffer, projector closing over
// the wrong variable, etc.) would leave the live table empty; the
// pure-function tests above cannot catch that.
func TestArticlesPipelineFiltersByFeed(t *testing.T) {
	selectSend, selectObs := rx.Subject[FeedID](0, 1)
	filterSend, filterObs := rx.Subject[string](0, 1)
	filterSend.Next("")
	_, sortObs := rx.Subject[table.Sort](0, 1)
	sortObs = sortObs.StartWith(table.Sort{Column: colPublished, Asc: false})
	all := hardCodedArticles()
	pipeline := rx.Map(
		rx.CombineLatest3(selectObs, filterObs, sortObs),
		func(t rx.Tuple3[FeedID, string, table.Sort]) []article {
			return filterAndSortArticles(all, t.First, t.Second, t.Third)
		},
	)
	gotChan := make(chan []article, 4)
	_ = pipeline.Subscribe(func(arts []article, _ error, done bool) {
		if done {
			return
		}
		cp := append([]article(nil), arts...)
		select {
		case gotChan <- cp:
		default:
		}
	}, rx.Goroutine)

	selectSend.Next("hn")
	awaitFeed := func(want FeedID) []article {
		deadline := time.Now().Add(500 * time.Millisecond)
		for time.Now().Before(deadline) {
			select {
			case got := <-gotChan:
				if len(got) > 0 && got[0].FeedID == want {
					return got
				}
			case <-time.After(20 * time.Millisecond):
			}
		}
		t.Fatalf("pipeline never emitted articles for feed %q", want)
		return nil
	}
	hn := awaitFeed("hn")
	for _, a := range hn {
		if a.FeedID != "hn" {
			t.Errorf("got article from feed %q; want only hn", a.FeedID)
		}
	}

	selectSend.Next("bbc")
	bbc := awaitFeed("bbc")
	for _, a := range bbc {
		if a.FeedID != "bbc" {
			t.Errorf("got article from feed %q; want only bbc", a.FeedID)
		}
	}

	filterSend.Next("Markets")
	deadline := time.Now().Add(500 * time.Millisecond)
	var filtered []article
	for time.Now().Before(deadline) {
		select {
		case got := <-gotChan:
			match := true
			for _, a := range got {
				if !strings.Contains(strings.ToLower(a.Title), "markets") {
					match = false
					break
				}
			}
			if match && len(got) > 0 {
				filtered = got
			}
		case <-time.After(20 * time.Millisecond):
		}
		if filtered != nil {
			break
		}
	}
	if filtered == nil {
		t.Fatal("pipeline never emitted filtered-by-query articles for bbc")
	}
}

// shellCanvas is the canvas the regression test draws each emitted shell
// widget into, exercising the full layout path (sidebar + navbar + Main).
const (
	shellCanvasW = 1200
	shellCanvasH = 800
)

// TestFeedsShellLayerReEmitsOnModelChange is the GX.10 same-frame-repaint
// regression test, the sister of GX.9's TestDocsShellLayerReEmitsOnModelChange.
// The bug it guards against: the shell layer observable did not re-emit when
// the model changed (selection, page, sort were shunted into atomic mirrors
// disconnected from the layer chain), so a click never reached
// spectrum/window's Invalidate() and the canvas only repainted on the next
// unrelated input event (FEEDBACK-G5.1/G5.2).
//
// Driving the same modelObs the app uses (via an rx.Subject[Model]) and
// asserting feedsShellLayer's returned observable emits a fresh widget on each
// SelectFeed / SetPage / SetSort / ToggleSection is the seam the bug lived on;
// a reducer-only test passes without proving the layer re-emits. (Live
// same-frame repaint is confirmed by running the app — the unit test proves
// the necessary re-emission, not the OS frame timing.)
func TestFeedsShellLayerReEmitsOnModelChange(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))

	send, modelObs := rx.Subject[Model](0, 1, 256)
	shellLayer := feedsShellLayer(rx.Of(theme.Default()), shaper, modelObs)

	emissions := make(chan layout.Widget, 16)
	sub := shellLayer.Subscribe(func(w layout.Widget, _ error, done bool) {
		if !done && w != nil {
			select {
			case emissions <- w:
			default:
			}
		}
	}, rx.Goroutine)
	defer sub.Unsubscribe()

	await := func(what string) layout.Widget {
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			select {
			case w := <-emissions:
				return w
			case <-time.After(10 * time.Millisecond):
			}
		}
		t.Fatalf("shell layer did not re-emit after %s", what)
		return nil
	}
	drain := func() {
		for {
			select {
			case <-emissions:
			default:
				return
			}
		}
	}

	send.Next(initialModel())
	if w := await("initial model"); w != nil {
		drawShellOnce(t, image.Pt(shellCanvasW, shellCanvasH), w)
	}
	drain()

	// Each model-changing message must produce a fresh layer emission, which
	// is precisely what drives the same-frame Invalidate().
	m := initialModel()

	m, _ = Update(m, SelectFeed{Feed: "bbc"})
	send.Next(m)
	if w := await("SelectFeed"); w != nil {
		drawShellOnce(t, image.Pt(shellCanvasW, shellCanvasH), w)
	}
	drain()

	m, _ = Update(m, SetPage{Page: 2})
	send.Next(m)
	if w := await("SetPage"); w != nil {
		drawShellOnce(t, image.Pt(shellCanvasW, shellCanvasH), w)
	}
	drain()

	m, _ = Update(m, SetSort{Sort: table.Sort{Column: colTitle, Asc: true}})
	send.Next(m)
	if w := await("SetSort"); w != nil {
		drawShellOnce(t, image.Pt(shellCanvasW, shellCanvasH), w)
	}
	drain()

	m, _ = Update(m, ToggleSection{Idx: 1})
	send.Next(m)
	if w := await("ToggleSection"); w != nil {
		drawShellOnce(t, image.Pt(shellCanvasW, shellCanvasH), w)
	}
	drain()

	// G5.2c messages: article selection populates the detail pane, tab
	// switching swaps its content, ToggleShare opens the navbar popover —
	// each must re-emit the layer just like the original four.
	m, _ = Update(m, SelectArticle{Article: "bbc-03"})
	send.Next(m)
	if w := await("SelectArticle"); w != nil {
		drawShellOnce(t, image.Pt(shellCanvasW, shellCanvasH), w)
	}
	drain()

	m, _ = Update(m, SelectTab{Idx: tabRaw})
	send.Next(m)
	if w := await("SelectTab"); w != nil {
		drawShellOnce(t, image.Pt(shellCanvasW, shellCanvasH), w)
	}
	drain()

	m, _ = Update(m, ToggleShare{})
	send.Next(m)
	if w := await("ToggleShare"); w != nil {
		drawShellOnce(t, image.Pt(shellCanvasW, shellCanvasH), w)
	}
	drain()

	m, _ = Update(m, SetSplitRatio{Ratio: 0.5})
	send.Next(m)
	if w := await("SetSplitRatio"); w != nil {
		drawShellOnce(t, image.Pt(shellCanvasW, shellCanvasH), w)
	}
}

// drawShellOnce lays a widget out once on a fresh op buffer so a re-emitted
// shell widget is exercised through its full layout path (catching a panic in
// the composed sidebar/table/pagination), without requiring a GPU.
func drawShellOnce(t *testing.T, size image.Point, w layout.Widget) {
	t.Helper()
	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}
	w(gtx)
}

// ----- golden render of the articles table for one feed (G5.2b
// Measurable: "golden of one feed's table state in light + dark") -----

const (
	canvasW = 900
	canvasH = 360
)

var canvasSize = image.Pt(canvasW, canvasH)

// TestArticlesTableGolden renders the first page of the hn feed with
// Published-desc sort in both light and dark token sets. Sharp radii and
// the static Render path keep the output deterministic — only colour
// pairs distinguish the two goldens.
func TestArticlesTableGolden(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	all := hardCodedArticles()
	rows := filterAndSortArticles(all, "hn", "", table.Sort{Column: colPublished, Asc: false})
	rows = pageSlice(rows, 1, articlesPageSize)

	cases := []struct {
		name   string
		colors tokens.ColorTokens
		bg     color.NRGBA
	}{
		{"hn-page1-light", tokens.DefaultLight, color.NRGBA{R: 240, G: 240, B: 240, A: 255}},
		{"hn-page1-dark", tokens.DefaultDark, color.NRGBA{R: 20, G: 20, B: 20, A: 255}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cols := staticArticleColumns(shaper, tc.colors, tokens.DefaultTypeScale)
			tbl := table.Render(shaper, cols, rows, table.Sort{Column: colPublished, Asc: false},
				tc.colors, tokens.Spacing, tokens.DefaultTypeScale)
			renderGolden(t, tc.name, canvasSize, scene(tbl, tc.bg))
		})
	}
}

// TestArticlesTableLightDarkDiffer confirms that swapping token sets
// produces a pixel-distinguishable render. Guards against a regression
// in which the table ignores its colour inputs.
func TestArticlesTableLightDarkDiffer(t *testing.T) {
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	all := hardCodedArticles()
	rows := filterAndSortArticles(all, "hn", "", table.Sort{Column: colPublished, Asc: false})
	rows = pageSlice(rows, 1, articlesPageSize)
	bg := color.NRGBA{R: 128, G: 128, B: 128, A: 255}

	render := func(colors tokens.ColorTokens) *image.RGBA {
		cols := staticArticleColumns(shaper, colors, tokens.DefaultTypeScale)
		tbl := table.Render(shaper, cols, rows, table.Sort{Column: colPublished, Asc: false},
			colors, tokens.Spacing, tokens.DefaultTypeScale)
		return capture(t, canvasSize, scene(tbl, bg))
	}
	light := render(tokens.DefaultLight)
	dark := render(tokens.DefaultDark)
	if light == nil || dark == nil {
		return
	}
	if n := pixelDiff(light, dark); n == 0 {
		t.Error("light and dark token sets produced identical output; expected colour differences")
	}
}

// ----- inlined golden harness (prism/internal/golden is not importable
// from outside the prism module tree) -----

// collectOne subscribes to obs and returns its first emitted widget. The
// feeds shell layer folds live cadence widget streams (table, pagination,
// textfield) onto its output, so it never completes — a .Wait()-until-done
// would block forever. Instead the first non-nil emission is captured on a
// goroutine subscription that is then unsubscribed, with a timeout guarding
// against a layer that never emits at all.
//
// The subscription is RETRIED on timeout: a freshly subscribed, fully cold
// chain (backdropLayer over rx.Of tokens) intermittently fails to deliver
// within seconds on rx.Goroutine — observed as a full-suite hang once and as
// a ~1-in-3 full-suite -race flake, while 10/10 isolated runs pass. The
// wedge is per-subscription somewhere in reactivego/rx scheduling, so a
// fresh Subscribe sidesteps it. (Logged in FEEDBACK-G5.2.md under G5.2d.)
func collectOne(obs rx.Observable[layout.Widget]) (layout.Widget, error) {
	const (
		attempts = 3
		window   = 2 * time.Second
	)
	for i := 0; ; i++ {
		gotChan := make(chan layout.Widget, 1)
		errChan := make(chan error, 1)
		sub := obs.Subscribe(func(v layout.Widget, err error, done bool) {
			if done {
				select {
				case errChan <- err:
				default:
				}
				return
			}
			if v != nil {
				select {
				case gotChan <- v:
				default:
				}
			}
		}, rx.Goroutine)
		select {
		case w := <-gotChan:
			sub.Unsubscribe()
			return w, nil
		case err := <-errChan:
			sub.Unsubscribe()
			if err != nil {
				return nil, err
			}
			// Completed WITHOUT any emission: a cold token chain always
			// emits before completing, so this is the rx delivery dropout
			// (value lost, completion delivered) — retry like a timeout.
			if i == attempts-1 {
				return nil, nil
			}
		case <-time.After(window):
			sub.Unsubscribe()
			if i == attempts-1 {
				return nil, nil
			}
			// Wedged subscription: retry with a fresh one.
		}
	}
}

func scene(w layout.Widget, bgColor color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, bgColor, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return w(gtx)
	}
}

func capture(t *testing.T, size image.Point, draw layout.Widget) *image.RGBA {
	t.Helper()
	w, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		t.Skipf("headless rendering not supported: %v", err)
		return nil
	}
	defer w.Release()

	var ops op.Ops
	gtx := layout.Context{
		Constraints: layout.Exact(size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Ops:         &ops,
	}
	draw(gtx)
	if err := w.Frame(&ops); err != nil {
		t.Fatalf("Frame: %v", err)
	}
	img := image.NewRGBA(image.Rectangle{Max: size})
	if err := w.Screenshot(img); err != nil {
		t.Fatalf("Screenshot: %v", err)
	}
	return img
}

func renderGolden(t *testing.T, name string, size image.Point, draw layout.Widget) {
	t.Helper()
	img := capture(t, size, draw)
	if img == nil {
		return
	}
	path := filepath.Join("testdata", "golden", name+".png")
	if *goldenUpdate {
		if err := saveImage(path, img); err != nil {
			t.Fatalf("save %s: %v", path, err)
		}
		return
	}
	stored, err := loadImage(path)
	if os.IsNotExist(err) {
		t.Fatalf("%s not found; run go test -golden.update to create", path)
		return
	}
	if err != nil {
		t.Fatalf("load %s: %v", path, err)
		return
	}
	if n := pixelDiff(stored, img); n > 0 {
		actualPath := strings.TrimSuffix(path, ".png") + ".actual.png"
		_ = saveImage(actualPath, img)
		t.Fatalf("%q: %d pixel(s) differ (actual saved to %s)", name, n, actualPath)
	}
}

func pixelDiff(a, b *image.RGBA) int {
	if a.Bounds() != b.Bounds() {
		return -1
	}
	bounds := a.Bounds()
	n := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			off := (y-bounds.Min.Y)*a.Stride + (x-bounds.Min.X)*4
			if a.Pix[off] != b.Pix[off] ||
				a.Pix[off+1] != b.Pix[off+1] ||
				a.Pix[off+2] != b.Pix[off+2] ||
				a.Pix[off+3] != b.Pix[off+3] {
				n++
			}
		}
	}
	return n
}

func saveImage(path string, img *image.RGBA) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	nrgba := &image.NRGBA{Pix: img.Pix, Stride: img.Stride, Rect: img.Rect}
	return png.Encode(f, nrgba)
}

func loadImage(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	decoded, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	switch v := decoded.(type) {
	case *image.RGBA:
		return v, nil
	case *image.NRGBA:
		return &image.RGBA{Pix: v.Pix, Stride: v.Stride, Rect: v.Rect}, nil
	default:
		bounds := decoded.Bounds()
		rgba := image.NewRGBA(bounds)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				rgba.Set(x, y, decoded.At(x, y))
			}
		}
		return rgba, nil
	}
}
