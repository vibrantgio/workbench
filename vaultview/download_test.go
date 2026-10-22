package main

import (
	"image"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/mvu/desktop"
)

// standInDownload puts a command of this test's own in place of the
// platform's for the length of the test, and answers the paths it was
// asked for. Nothing here touches a cloud: what is under test is what
// the window does between the ask and the answer.
func standInDownload(t *testing.T, answer func(path string) mvu.Command) *[]string {
	t.Helper()
	var mu sync.Mutex
	asked := []string{}
	was := platformDownload
	platformDownload = func(path string) mvu.Command {
		mu.Lock()
		asked = append(asked, path)
		mu.Unlock()
		return answer(path)
	}
	t.Cleanup(func() { platformDownload = was })
	return &asked
}

// runCmd subscribes a command the way the loop does and answers the
// messages it emitted, in order.
func runCmd(t *testing.T, cmd mvu.Command) []mvu.Message {
	t.Helper()
	var mu sync.Mutex
	var msgs []mvu.Message
	sub := cmd.Subscribe(rx.GoroutineContext(), func(next mvu.Message, err error, done bool) {
		if done || next == nil {
			return
		}
		mu.Lock()
		msgs = append(msgs, next)
		mu.Unlock()
	})
	if err := sub.Wait(); err != nil {
		t.Fatalf("the command failed: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	return msgs
}

// cloudVault is a vault holding one note on disk and one the cloud has
// not handed over: the hidden stand-in is there and nothing stands at
// the name the vault shows.
func cloudVault(t *testing.T) (root string, model Model) {
	t.Helper()
	root = t.TempDir()
	old := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	aged(t, root, "here.md", "# Here\n\nOn this disk.\n", old)
	aged(t, root, "cloud/.Cloudy.md.icloud", "", old)

	idx, _, err := ListVault(root)
	if err != nil {
		t.Fatal(err)
	}
	model = Model{Screen: screenVault, Vault: root, CurAnchor: -1}
	gen, _ := model.nextScan()
	model, _ = Update(model, vaultListed{vault: root, gen: gen, index: idx})
	if !model.isPlaceholder("cloud/Cloudy.md") {
		t.Fatal("the fixture's cloud note was not listed as one the cloud holds")
	}
	return root, model
}

// readingHere lands the window on the note the disk holds, so the tests
// that leave it for one the cloud holds have a note to come back to and a
// history entry to be measured against.
func readingHere(t *testing.T, model Model) Model {
	t.Helper()
	model, cmd := Update(model, Navigate{Path: "here.md"})
	msgs := runCmd(t, cmd)
	if len(msgs) != 1 {
		t.Fatalf("opening the note on disk emitted %d messages, want the load", len(msgs))
	}
	model, _ = Update(model, msgs[0])
	if model.Current != "here.md" || model.CurrentNote() == nil {
		t.Fatalf("the window is on %q with note %v, want the note on disk", model.Current, model.CurrentNote())
	}
	return model
}

// TestOpeningANoteTheCloudHoldsFetchesItThenLoadsIt walks the whole
// path: the navigation asks for the file instead of reading one that is
// not there, the window says what it is waiting for, and the answer
// loads the note the way any navigation loads one.
func TestOpeningANoteTheCloudHoldsFetchesItThenLoadsIt(t *testing.T) {
	root, model := cloudVault(t)
	full := filepath.Join(root, "cloud", "Cloudy.md")
	body := "# Cloudy\n\nThe cloud handed this over. ^blk-1\n"

	// The stand-in writes the file as the platform would and answers.
	asked := standInDownload(t, func(path string) mvu.Command {
		return mvu.Do(func() (mvu.Message, error) {
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				return nil, err
			}
			return desktop.Downloaded{Path: path}, nil
		})
	})

	model = readingHere(t, model)

	model, cmd := Update(model, Navigate{Path: "cloud/Cloudy.md", BlockID: "blk-1"})
	if model.Downloading != "cloud/Cloudy.md" {
		t.Errorf("the window is waiting on %q, want the note that was asked for", model.Downloading)
	}
	// The window moves to the note it is fetching at once, and has no note
	// for it: the trail names it and the column says what the wait is for.
	if model.Current != "cloud/Cloudy.md" {
		t.Errorf("the window stands on %q, want the note it is fetching", model.Current)
	}
	if model.CurrentNote() != nil {
		t.Error("the window has a note for a file that is not here yet")
	}
	if places := notePlaces(model); len(places) == 0 || places[len(places)-1].title != "Cloudy" {
		t.Errorf("the trail reads %+v, want the note being fetched at its end", places)
	}
	// The history is not touched until the note arrives: the entry would
	// have to come off again if the fetch failed.
	if want := []HistEntry{{Path: "here.md", Anchor: -1}}; !reflect.DeepEqual(model.History, want) {
		t.Errorf("the history reads %+v during the wait, want %+v", model.History, want)
	}
	if got := statusLine(model); got != downloadingLabel {
		t.Errorf("the status bar says %q, want %q", got, downloadingLabel)
	}

	msgs := runCmd(t, cmd)
	if want := []string{full}; !reflect.DeepEqual(*asked, want) {
		t.Fatalf("the platform was asked for %v, want %v", *asked, want)
	}
	if len(msgs) != 1 {
		t.Fatalf("the download emitted %d messages, want one answer", len(msgs))
	}
	done, ok := msgs[0].(desktop.Downloaded)
	if !ok {
		t.Fatalf("the download emitted a %T, want desktop.Downloaded", msgs[0])
	}

	model, cmd = Update(model, done)
	if model.Downloading != "" {
		t.Error("the window is still waiting after the file arrived")
	}
	if model.isPlaceholder("cloud/Cloudy.md") {
		t.Error("the listing still holds the note as one the cloud has not handed over")
	}
	if got := statusLine(model); got == downloadingLabel {
		t.Error("the status bar still says the note is being fetched")
	}

	// The load the answer asked for lands the note, anchor and all.
	loads := runCmd(t, cmd)
	if len(loads) != 1 {
		t.Fatalf("the answer started %d loads, want one", len(loads))
	}
	model, _ = Update(model, loads[0])
	if model.Current != "cloud/Cloudy.md" {
		t.Fatalf("the window is on %q, want the note that was fetched", model.Current)
	}
	note := model.CurrentNote()
	if note == nil || note.Title != "Cloudy" {
		t.Fatalf("the fetched note did not load: %+v", note)
	}
	if model.CurAnchor < 0 {
		t.Error("the landing lost the block the link named")
	}
	want := []HistEntry{{Path: "here.md", Anchor: -1}, {Path: "cloud/Cloudy.md", Anchor: model.CurAnchor}}
	if !reflect.DeepEqual(model.History, want) {
		t.Errorf("the history reads %+v after the note arrived, want %+v", model.History, want)
	}
	if model.Cursor != 1 {
		t.Errorf("the cursor is at %d, want the entry the note arrived at", model.Cursor)
	}
	if model.Notifications.Len() != 0 {
		t.Errorf("a download that worked raised %d notifications", model.Notifications.Len())
	}
}

// TestADownloadThatFailsSaysSoAndStopsWaiting holds the other answer: the
// window takes its waiting state down, returns the reader to the note they
// were on — the trail and the tree's mark with it — and reports the failure
// the way a note that will not load is reported.
func TestADownloadThatFailsSaysSoAndStopsWaiting(t *testing.T) {
	_, model := cloudVault(t)
	standInDownload(t, func(path string) mvu.Command {
		return mvu.Do(func() (mvu.Message, error) {
			return desktop.Downloaded{Path: path, Err: "nothing is in the cloud on this platform"}, nil
		})
	})
	model = readingHere(t, model)
	before := model.History

	model, cmd := Update(model, Navigate{Path: "cloud/Cloudy.md"})
	if model.Current != "cloud/Cloudy.md" {
		t.Fatalf("the window stands on %q during the wait, want the note it is fetching", model.Current)
	}
	msgs := runCmd(t, cmd)
	if len(msgs) != 1 {
		t.Fatalf("the download emitted %d messages, want one answer", len(msgs))
	}
	model, _ = Update(model, msgs[0])

	if model.Downloading != "" {
		t.Error("the window is still waiting after the download failed")
	}
	if model.Notifications.Len() != 1 {
		t.Fatalf("a failed download raised %d notifications, want one", model.Notifications.Len())
	}
	if got := model.Notifications.Items()[0].Text; got == "" {
		t.Error("the notification says nothing about why the note did not arrive")
	}
	if !model.isPlaceholder("cloud/Cloudy.md") {
		t.Error("a download that failed cleared the listing's flag all the same")
	}
	if model.Current != "here.md" {
		t.Errorf("the window stands on %q after the failure, want the note the reader was on", model.Current)
	}
	if places := notePlaces(model); len(places) == 0 || places[len(places)-1].title != "here" {
		t.Errorf("the trail reads %+v after the failure, want the note the reader was on", places)
	}
	if !reflect.DeepEqual(model.History, before) {
		t.Errorf("the history reads %+v after the failure, want the %+v it had", model.History, before)
	}
}

// TestANoteThatArrivesButWillNotReadReturnsTheReader holds the other way
// the move ahead can come to nothing: the platform writes the file and the
// read of it refuses all the same. The window stands on a note it has not
// got, so it goes back to the one the reader was on, as a refused download
// does.
func TestANoteThatArrivesButWillNotReadReturnsTheReader(t *testing.T) {
	_, model := cloudVault(t)
	// A directory where the note should be: the platform answered that it
	// wrote the file and the read of it fails.
	standInDownload(t, func(path string) mvu.Command {
		return mvu.Do(func() (mvu.Message, error) {
			if err := os.Mkdir(path, 0o755); err != nil {
				return nil, err
			}
			return desktop.Downloaded{Path: path}, nil
		})
	})
	model = readingHere(t, model)

	model, cmd := Update(model, Navigate{Path: "cloud/Cloudy.md"})
	msgs := runCmd(t, cmd)
	if len(msgs) != 1 {
		t.Fatalf("the download emitted %d messages, want one answer", len(msgs))
	}
	model, cmd = Update(model, msgs[0])
	loads := runCmd(t, cmd)
	if len(loads) != 1 {
		t.Fatalf("the answer started %d loads, want one", len(loads))
	}
	model, _ = Update(model, loads[0])

	if model.Current != "here.md" {
		t.Errorf("the window stands on %q, want the note the reader was on", model.Current)
	}
	if model.Notifications.Len() != 1 {
		t.Errorf("a note that would not read raised %d notifications, want one", model.Notifications.Len())
	}
	if len(model.History) != 1 {
		t.Errorf("the history reads %+v, want the one note that was read", model.History)
	}
}

// TestBackDuringAFetchReturnsToTheNoteTheReaderWasOn drives the window's
// history while a note is being fetched. The note being fetched is no
// history entry, so the way back from it is the note the reader was on and
// nothing stands ahead of it; the fetch is left behind and its answer,
// arriving after, changes nothing.
func TestBackDuringAFetchReturnsToTheNoteTheReaderWasOn(t *testing.T) {
	root, model := cloudVault(t)
	standInDownload(t, func(string) mvu.Command { return mvu.DoNothing() })
	model = readingHere(t, model)

	model, _ = Update(model, Navigate{Path: "cloud/Cloudy.md"})
	if !model.CanGoBack() {
		t.Error("the way back is shut while the window waits on a note it has not got")
	}
	if model.CanGoForward() {
		t.Error("the window offers a way forward from a note it has not reached")
	}

	back, _ := Update(model, GoBack{})
	if back.Current != "here.md" {
		t.Errorf("back during the wait stands on %q, want the note the reader was on", back.Current)
	}
	if back.Downloading != "" {
		t.Error("back during the wait left the window waiting")
	}
	if back.Cursor != 0 || len(back.History) != 1 {
		t.Errorf("back during the wait moved the history to %+v at %d", back.History, back.Cursor)
	}
	if back.CurrentNote() == nil {
		t.Error("the note the reader was on did not come back with them")
	}

	// The answer the platform gives after that belongs to nothing.
	after, cmd := Update(back, desktop.Downloaded{Path: filepath.Join(root, "cloud", "Cloudy.md")})
	if !reflect.DeepEqual(after, back) {
		t.Error("an answer for a fetch the reader left changed the window")
	}
	if msgs := runCmd(t, cmd); len(msgs) != 0 {
		t.Errorf("it started %d commands too", len(msgs))
	}

	// Forward is inert for the same reason the fetch has no entry.
	fwd, _ := Update(model, GoForward{})
	if fwd.Current != "cloud/Cloudy.md" || fwd.Downloading != "cloud/Cloudy.md" {
		t.Errorf("forward during the wait moved the window to %q (waiting on %q)", fwd.Current, fwd.Downloading)
	}
}

// TestAnAnswerForANoteTheReaderLeftChangesNothing holds the guard every
// late answer needs: the reader moved on, and the download that was in
// flight must not pull the window back to the note they left.
func TestAnAnswerForANoteTheReaderLeftChangesNothing(t *testing.T) {
	root, model := cloudVault(t)
	standInDownload(t, func(path string) mvu.Command {
		return mvu.Do(func() (mvu.Message, error) { return nil, nil })
	})

	model, _ = Update(model, Navigate{Path: "cloud/Cloudy.md"})
	model = leaveDownload(model) // the reader left: another note, another vault

	after, cmd := Update(model, desktop.Downloaded{Path: filepath.Join(root, "cloud", "Cloudy.md")})
	if !reflect.DeepEqual(after, model) {
		t.Error("an answer for a note nothing is waiting on changed the window")
	}
	if msgs := runCmd(t, cmd); len(msgs) != 0 {
		t.Errorf("it started %d commands too", len(msgs))
	}
}

// TestANoteOnDiskIsNeverAskedFor holds the other half of the branch: a
// note the vault has is read, not fetched, and the window never says it
// is waiting on a file it already has.
func TestANoteOnDiskIsNeverAskedFor(t *testing.T) {
	_, model := cloudVault(t)
	asked := standInDownload(t, func(path string) mvu.Command {
		return mvu.Do(func() (mvu.Message, error) { return nil, nil })
	})

	model, cmd := Update(model, Navigate{Path: "here.md"})
	if model.Downloading != "" {
		t.Error("the window says it is fetching a note that is on disk")
	}
	msgs := runCmd(t, cmd)
	if len(*asked) != 0 {
		t.Errorf("the platform was asked for %v; the note is on disk", *asked)
	}
	if len(msgs) != 1 {
		t.Fatalf("the navigation emitted %d messages, want the load", len(msgs))
	}
	if _, ok := msgs[0].(noteLoaded); !ok {
		t.Fatalf("the navigation emitted a %T, want noteLoaded", msgs[0])
	}
}

// fetchingWindow is a window standing on a note the cloud holds: the
// reader was on a note that cites it, and the navigation left them waiting
// on the file. The stand-in never answers, so the window stays in the
// wait for as long as the test reads it.
func fetchingWindow(t *testing.T) Model {
	t.Helper()
	standInDownload(t, func(string) mvu.Command { return mvu.DoNothing() })
	m := Model{Screen: screenVault, Vault: "/v", CurAnchor: -1}
	m.Index = &Index{Root: "/v", Files: []FileScan{
		{Path: "Here.md", Headings: []Heading{{Level: 1, Title: "Here"}}, Links: []string{"Cloudy"}},
		{Path: "guide/Cloudy.md", Placeholder: true},
	}}
	m = cacheNote(m, noteFromSource("Here.md", "# Here\n\nSee [[Cloudy]].\n"))
	m.Current = "Here.md"
	m.History = []HistEntry{{Path: "Here.md", Anchor: -1}}
	m.Cursor = 0
	m, _ = Update(m, Navigate{Path: "guide/Cloudy.md"})
	if m.Downloading != "guide/Cloudy.md" {
		t.Fatalf("the window is waiting on %q, want the note the cloud holds", m.Downloading)
	}
	return m
}

// TestTheWindowStandsOnTheNoteItIsFetching holds what the window shows
// while it waits: it is at the note the reader asked for, so the trail
// names it and the tree marks its row with the folders on the way to it
// open, and it has none of the note, so the inspector draws its empty
// lines in place of an outline and a list of citations.
func TestTheWindowStandsOnTheNoteItIsFetching(t *testing.T) {
	m := fetchingWindow(t)

	if m.Current != "guide/Cloudy.md" {
		t.Fatalf("the window stands on %q, want the note it is fetching", m.Current)
	}
	if m.CurrentNote() != nil {
		t.Fatal("the window has a note for a file that is not here yet")
	}

	// The trail: the vault, the folder on the way, the note being fetched.
	want := []place{
		{title: "v"},
		{title: "guide", path: "guide"},
		{title: "Cloudy", path: "guide/Cloudy.md"},
	}
	if got := notePlaces(m); !reflect.DeepEqual(got, want) {
		t.Errorf("the trail reads %+v, want %+v", got, want)
	}

	// The tree: the row is in the run the rail draws, its folder opened by
	// the landing, and it is the row the view marks (Path == Current).
	if !m.Folds["guide"] {
		t.Errorf("the folds read %v; the landing did not open the way to the note", m.Folds)
	}
	row := visibleNoteRow(m, m.Current)
	if row == nil {
		t.Fatalf("the note being fetched has no row in the tree: %v", TreeRows(m.Index, m.Folds, vaultName(m)))
	}
	if row.Path != m.Current {
		t.Errorf("the tree marks %q, want the note being fetched", row.Path)
	}

	// The column and the bar both say what the window is waiting for.
	if got := statusLine(m); got != downloadingLabel {
		t.Errorf("the status bar says %q, want %q", got, downloadingLabel)
	}

	// The inspector: nothing to draw for a note the window has not got.
	// The citation is really there — with the window merely on the note it
	// is listed — so the empty pane is the rule and not the fixture.
	cited := newAsideView(&docCursor{}).backlinks(Model{Index: m.Index, Current: "guide/Cloudy.md"})
	if len(cited) != 1 || cited[0].Path != "Here.md" {
		t.Fatalf("the note is cited by %+v; this fixture is meant to carry one citation", cited)
	}
	v := newAsideView(&docCursor{})
	if got := v.headings(m); len(got) != 0 {
		t.Errorf("the outline reads %+v for a note that has not been read", got)
	}
	if got := v.backlinks(m); len(got) != 0 {
		t.Errorf("the backlinks read %+v for a note that has not arrived", got)
	}
	var ops op.Ops
	gtx := layout.Context{
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(frameAsideDp, 700)),
		Ops:         &ops,
	}
	if d := v.layout(gtx, m, goldenTokens()); d.Size.X == 0 || d.Size.Y == 0 {
		t.Fatalf("the inspector produced %v with no note to describe", d)
	}

	// And the history is where the reader left it.
	if len(m.History) != 1 || m.History[0].Path != "Here.md" || m.Cursor != 0 {
		t.Errorf("the history reads %+v at %d, want the note the reader was on", m.History, m.Cursor)
	}
}
