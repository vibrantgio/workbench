package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu"
)

// runScan subscribes a command the way the mvu loop does — on the
// goroutine scheduler, so the contents phase really does read several
// notes at once — and collects every message it emits, in order.
func runScan(t *testing.T, cmd mvu.Command) []mvu.Message {
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
		t.Fatalf("scan command: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	return msgs
}

// foldScan folds every message a scan command emits through Update, as
// the loop does.
func foldScan(t *testing.T, model Model, cmd mvu.Command) Model {
	t.Helper()
	for _, msg := range runScan(t, cmd) {
		model, _ = Update(model, msg)
	}
	return model
}

// aged writes a note and stamps its modification time, which is what
// the listing orders the folders by.
func aged(t *testing.T, root, rel, src string, mod time.Time) {
	t.Helper()
	writeNote(t, root, rel, src)
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.Chtimes(full, mod, mod); err != nil {
		t.Fatal(err)
	}
}

// threeFolderVault is the fixture the ordering tests read: three folders
// whose notes were last written days apart, so "newest folder first" has
// one answer and not three.
func threeFolderVault(t *testing.T, perFolder int) string {
	t.Helper()
	root := t.TempDir()
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for i, folder := range []string{"newest", "middle", "oldest"} {
		mod := base.AddDate(0, 0, -i)
		for n := range perFolder {
			rel := fmt.Sprintf("%s/n%02d.md", folder, n)
			aged(t, root, rel, fmt.Sprintf("# %s %d\n\n[[n00]] ^b%02d\n", folder, n, n), mod)
		}
	}
	return root
}

func jobPaths(jobs []scanJob) []string {
	out := make([]string, len(jobs))
	for i, j := range jobs {
		out[i] = j.path
	}
	return out
}

func indexPaths(idx *Index) []string {
	out := make([]string, len(idx.Files))
	for i := range idx.Files {
		out[i] = idx.Files[i].Path
	}
	return out
}

// TestListVaultReadsNothingAndOrdersFoldersNewestFirst holds the two
// promises of the listing phase: it answers every path in the vault, and
// it hands the contents phase an order that begins in the folder written
// most recently.
func TestListVaultReadsNothingAndOrdersFoldersNewestFirst(t *testing.T) {
	root := threeFolderVault(t, 3)

	idx, jobs, err := ListVault(root)
	if err != nil {
		t.Fatalf("ListVault: %v", err)
	}
	// Every path, and nothing read: a listed record carries its path and
	// no headings, block ids or links until the contents phase folds its
	// scan in.
	want := []string{
		"middle/n00.md", "middle/n01.md", "middle/n02.md",
		"newest/n00.md", "newest/n01.md", "newest/n02.md",
		"oldest/n00.md", "oldest/n01.md", "oldest/n02.md",
	}
	if got := indexPaths(idx); !reflect.DeepEqual(got, want) {
		t.Errorf("listing = %v, want the vault's paths in walk order %v", got, want)
	}
	for _, f := range idx.Files {
		if len(f.Headings) != 0 || len(f.BlockIDs) != 0 || len(f.Links) != 0 {
			t.Errorf("%s came out of the listing already scanned; the listing reads no file", f.Path)
		}
	}

	var folders []string
	for _, p := range jobPaths(jobs) {
		if dir := strings.Split(p, "/")[0]; len(folders) == 0 || folders[len(folders)-1] != dir {
			folders = append(folders, dir)
		}
	}
	wantOrder := []string{"newest", "middle", "oldest"}
	if !reflect.DeepEqual(folders, wantOrder) {
		t.Errorf("reading order runs through the folders %v, want %v", folders, wantOrder)
	}
}

// TestListVaultListsAnICloudPlaceholderAndReadsItLast holds the rule for
// a note iCloud has not handed over: the vault shows it, so the tree
// shows it, and nothing reads it until every note that is on disk has
// been read.
func TestListVaultListsAnICloudPlaceholderAndReadsItLast(t *testing.T) {
	root := t.TempDir()
	old := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	aged(t, root, "later/here.md", "# Here\n", old)
	// The disk holds the hidden stand-in and no Cloudy.md beside it.
	aged(t, root, "cloud/.Cloudy.md.icloud", "", old.AddDate(0, 0, 1))

	idx, jobs, err := ListVault(root)
	if err != nil {
		t.Fatalf("ListVault: %v", err)
	}
	if got, want := indexPaths(idx), []string{"cloud/Cloudy.md", "later/here.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("listing = %v, want the placeholder under the name the vault shows %v", got, want)
	}
	if !idx.Files[0].Placeholder {
		t.Error("the placeholder was listed without its flag")
	}
	if idx.Files[1].Placeholder {
		t.Error("a note that is on disk was flagged a placeholder")
	}
	// Its folder is the newer of the two, so only the rule puts it last.
	if got, want := jobPaths(jobs), []string{"later/here.md", "cloud/Cloudy.md"}; !reflect.DeepEqual(got, want) {
		t.Errorf("reading order = %v, want the placeholder last %v", got, want)
	}

	// Reading one: the placeholder's own path holds no file, so the read
	// refuses and the record stays as the listing made it.
	got := readScan(root, scanJob{path: "cloud/Cloudy.md", placeholder: true})
	if got.Path != "cloud/Cloudy.md" || !got.Placeholder {
		t.Errorf("reading the placeholder answered %+v, want the listed record back", got)
	}
	if len(got.Headings) != 0 || len(got.Links) != 0 {
		t.Errorf("reading the placeholder found contents %+v; its path holds no file", got)
	}
}

// TestScanDeliversTheListingBeforeAnyContents walks the whole command and
// holds the order of its messages: the listing first and once, the
// reader's note next, then the contents in batches, then the end. The
// index the batches leave behind must be the one ScanVault answers in a
// single walk, headings, block ids and links alike.
func TestScanDeliversTheListingBeforeAnyContents(t *testing.T) {
	root := threeFolderVault(t, 64)

	model := Model{Screen: screenVault, Vault: root, Scanning: true, CurAnchor: -1}
	gen, stop := model.nextScan()
	msgs := runScan(t, scanCmd(root, "", gen, stop, func(err string) mvu.Message {
		return vaultScanned{vault: root, gen: gen, err: err}
	}))

	listed, batches, ends := 0, 0, 0
	var delivered []string
	for i, msg := range msgs {
		switch m := msg.(type) {
		case vaultListed:
			listed++
			if batches > 0 {
				t.Fatalf("the listing arrived as message %d, after %d batches of contents", i, batches)
			}
			if got, want := len(m.index.Files), 3*64; got != want {
				t.Errorf("the listing holds %d paths, want %d", got, want)
			}
		case vaultNoteRead:
			if listed == 0 {
				t.Fatal("the reader's note arrived before the listing")
			}
			if batches > 0 {
				t.Error("the reader's note arrived after the contents had started")
			}
		case vaultRead:
			if listed == 0 {
				t.Fatalf("a batch of contents arrived as message %d, before the listing", i)
			}
			batches++
			for _, f := range m.files {
				delivered = append(delivered, f.Path)
			}
		case vaultScanned:
			ends++
			if i != len(msgs)-1 {
				t.Errorf("the scan ended at message %d of %d", i, len(msgs))
			}
		default:
			t.Errorf("the scan emitted a %T", msg)
		}
	}
	if listed != 1 || ends != 1 {
		t.Errorf("the scan emitted %d listings and %d endings, want one of each", listed, ends)
	}
	if batches < 2 {
		t.Errorf("192 notes arrived in %d batches; the contents are meant to be delivered as they are read", batches)
	}
	if len(delivered) != 3*64 {
		t.Errorf("the batches carried %d notes, want %d", len(delivered), 3*64)
	}

	// Newest folder first, held where no race can decide it: the oldest
	// folder's notes sit 128 places down the reading order, eight behind
	// each of the eight streams, so none of them can be in the first batch.
	if len(msgs) > 2 {
		first, ok := msgs[2].(vaultRead)
		if !ok {
			t.Fatalf("message 2 is a %T, want the first batch of contents", msgs[2])
		}
		for _, f := range first.files {
			if strings.HasPrefix(f.Path, "oldest/") {
				t.Errorf("the first batch carries %s; the oldest folder is read last", f.Path)
			}
		}
	}

	// The index the scan leaves behind is the index one walk would have
	// built, note for note.
	for _, msg := range msgs {
		model, _ = Update(model, msg)
	}
	want, err := ScanVault(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(model.Index.Files, want.Files) {
		t.Errorf("the streamed index is not ScanVault's: %d files against %d", len(model.Index.Files), len(want.Files))
		for i := range want.Files {
			if i < len(model.Index.Files) && !reflect.DeepEqual(model.Index.Files[i], want.Files[i]) {
				t.Errorf("file %d: %+v, want %+v", i, model.Index.Files[i], want.Files[i])
				break
			}
		}
	}
	if model.Scanning {
		t.Error("the scan's last message left the in-flight flag raised")
	}
	if model.Current != "newest/n00.md" && model.Current == "" {
		t.Error("the scan opened on no note")
	}
}

// TestASwitchedVaultTakesNothingFromTheScanItLeft holds the cancellation:
// a scan's messages carry the generation it was started under, and once
// another vault has been opened under the reader the old scan's batches
// change nothing.
func TestASwitchedVaultTakesNothingFromTheScanItLeft(t *testing.T) {
	root := threeFolderVault(t, 2)

	model := Model{Screen: screenVault, Vault: root, Scanning: true, CurAnchor: -1}
	gen, stop := model.nextScan()
	idx, jobs, err := ListVault(root)
	if err != nil {
		t.Fatal(err)
	}
	model, _ = Update(model, vaultListed{vault: root, gen: gen, index: idx})
	if model.Index == nil {
		t.Fatal("the listing did not land")
	}

	// The reader opens another vault. The scan in flight is told to stop
	// reading, and the generation it carries is no longer the model's.
	other := t.TempDir()
	writeNote(t, other, "only.md", "# Only\n")
	model, _ = Update(model, OpenVault{Path: other})
	if !stop.Load() {
		t.Error("opening another vault left the scan in flight reading")
	}

	held := model.Index
	stale := []mvu.Message{
		vaultListed{vault: root, gen: gen, index: idx},
		vaultNoteRead{vault: root, gen: gen, note: &Note{Path: jobs[0].path}},
		vaultRead{vault: root, gen: gen, files: []FileScan{readScan(root, jobs[0])}},
		vaultScanned{vault: root, gen: gen},
	}
	for _, msg := range stale {
		after, _ := Update(model, msg)
		if after.Index != held || after.Current != model.Current || after.Scanning != model.Scanning {
			t.Errorf("%T from the vault the reader left changed the model", msg)
		}
	}

	// A message from the scan the reader is actually waiting on lands.
	fresh, _, err := ListVault(other)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := Update(model, vaultListed{vault: other, gen: model.ScanGen, index: fresh})
	if after.Index != fresh {
		t.Error("the new vault's own listing was dropped with the old one's")
	}
}

// TestScanStopsReadingWhenTheStopFlagIsRaised holds the other half of
// cancellation: dropping a stale scan's messages stops them being
// believed, and the flag stops them being read, which on a vault held in
// the cloud is the half that costs.
func TestScanStopsReadingWhenTheStopFlagIsRaised(t *testing.T) {
	root := threeFolderVault(t, 8)
	_, jobs, err := ListVault(root)
	if err != nil {
		t.Fatal(err)
	}
	var model Model
	_, stop := model.nextScan()
	stop.Store(true)

	var files []FileScan
	sub := readStream(root, jobs, stop).Subscribe(rx.GoroutineContext(), func(next FileScan, err error, done bool) {
		if !done {
			files = append(files, next)
		}
	})
	if err := sub.Wait(); err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("a stopped stream read %d notes, want none", len(files))
	}
}

// TestMergeScansReplacesTheIndexRatherThanWritingIntoIt holds what the
// aside's backlink memo depends on: it re-runs only when the index
// pointer changes, so a batch folded into the index in place would leave
// the citations pane showing a vault that has since been read further.
func TestMergeScansReplacesTheIndexRatherThanWritingIntoIt(t *testing.T) {
	idx, _, err := ListVault(threeFolderVault(t, 2))
	if err != nil {
		t.Fatal(err)
	}
	before := idx.Files[0]
	next := mergeScans(idx, []FileScan{{Path: before.Path, Headings: []Heading{{Level: 1, Title: "Folded"}}}})
	if next == idx {
		t.Fatal("mergeScans answered the index it was given; the memo would never refresh")
	}
	if !reflect.DeepEqual(idx.Files[0], before) {
		t.Error("mergeScans wrote into the index it was given")
	}
	if got := next.Files[0].Headings; len(got) != 1 || got[0].Title != "Folded" {
		t.Errorf("the merged index holds %v at %s, want the folded scan", got, before.Path)
	}
}

// TestTheBarCountsTheScanOffAgainstTheListing holds what the status bar
// says while the contents are still arriving, and that it gives the line
// count back when they stop.
func TestTheBarCountsTheScanOffAgainstTheListing(t *testing.T) {
	idx := treeIndex("a.md", "b.md", "c.md")
	m := Model{Screen: screenVault, Vault: "/v", Scanning: true, CurAnchor: -1}
	if got := statusLine(m); got != "" {
		t.Errorf("before the listing the bar says %q, want nothing", got)
	}
	m, _ = Update(m, vaultListed{vault: "/v", index: idx})
	if got, want := statusLine(m), "Reading 0 of 3 notes…"; got != want {
		t.Errorf("the bar says %q, want %q", got, want)
	}
	m, _ = Update(m, vaultRead{vault: "/v", files: []FileScan{{Path: "a.md"}, {Path: "b.md"}}})
	if got, want := statusLine(m), "Reading 2 of 3 notes…"; got != want {
		t.Errorf("the bar says %q, want %q", got, want)
	}
	m, _ = Update(m, vaultScanned{vault: "/v"})
	if got := statusLine(m); got != "" {
		t.Errorf("with the scan done and no note open the bar says %q, want nothing", got)
	}
}

// TestTheListingOfATenThousandNoteVaultIsImmediate is the measurement the
// whole phase split exists for: the window must be able to draw the tree
// and the first note without waiting for ten thousand files to be read.
// The number is logged; the assertion carries margin, since a shared
// machine is not a bench.
func TestTheListingOfATenThousandNoteVaultIsImmediate(t *testing.T) {
	root := t.TempDir()
	const notes = 10000
	for i := range notes {
		rel := fmt.Sprintf("f%02d/n%04d.md", i%40, i)
		writeNote(t, root, rel, "# N\n\nbody\n")
	}

	start := time.Now()
	idx, jobs, err := ListVault(root)
	listing := time.Since(start)
	if err != nil {
		t.Fatalf("ListVault: %v", err)
	}
	if len(idx.Files) != notes || len(jobs) != notes {
		t.Fatalf("listed %d paths and %d jobs, want %d of each", len(idx.Files), len(jobs), notes)
	}
	t.Logf("listing %d notes took %v", notes, listing)
	if listing > 500*time.Millisecond {
		t.Errorf("the listing took %v; the window waits on it, and it is meant to be under 200ms", listing)
	}
}

// TestDealJobsSpreadsTheOrderOverEveryStream holds the deal: block
// dealing would have one stream reading the newest folder while the
// others read the oldest, so the order is spread a note at a time.
func TestDealJobsSpreadsTheOrderOverEveryStream(t *testing.T) {
	jobs := make([]scanJob, 20)
	for i := range jobs {
		jobs[i] = scanJob{path: fmt.Sprintf("n%02d.md", i)}
	}
	streams := dealJobs(jobs, scanStreams)
	if len(streams) != scanStreams {
		t.Fatalf("dealt into %d streams, want %d", len(streams), scanStreams)
	}
	var all []string
	for i, s := range streams {
		if s[0].path != jobs[i].path {
			t.Errorf("stream %d starts at %s, want %s", i, s[0].path, jobs[i].path)
		}
		for _, j := range s {
			all = append(all, j.path)
		}
	}
	sort.Strings(all)
	if len(all) != len(jobs) {
		t.Errorf("the deal lost notes: %d of %d", len(all), len(jobs))
	}
	// Fewer notes than streams: no empty stream is dealt.
	if got := len(dealJobs(jobs[:3], scanStreams)); got != 3 {
		t.Errorf("three notes dealt into %d streams, want 3", got)
	}
}
