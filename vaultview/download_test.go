package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

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

	model, cmd := Update(model, Navigate{Path: "cloud/Cloudy.md", BlockID: "blk-1"})
	if model.Downloading != "cloud/Cloudy.md" {
		t.Errorf("the window is waiting on %q, want the note that was asked for", model.Downloading)
	}
	if model.Current == "cloud/Cloudy.md" {
		t.Error("the window landed on a note whose file is not here yet")
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
	if model.Notifications.Len() != 0 {
		t.Errorf("a download that worked raised %d notifications", model.Notifications.Len())
	}
}

// TestADownloadThatFailsSaysSoAndStopsWaiting holds the other answer: the
// window takes its waiting state down and reports the failure the way a
// note that will not load is reported.
func TestADownloadThatFailsSaysSoAndStopsWaiting(t *testing.T) {
	_, model := cloudVault(t)
	standInDownload(t, func(path string) mvu.Command {
		return mvu.Do(func() (mvu.Message, error) {
			return desktop.Downloaded{Path: path, Err: "nothing is in the cloud on this platform"}, nil
		})
	})

	model, cmd := Update(model, Navigate{Path: "cloud/Cloudy.md"})
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
	if model.Current == "cloud/Cloudy.md" {
		t.Error("the window landed on a note that never arrived")
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
	model.Downloading = "" // the reader left: another note, another vault
	model.downloadNav = Navigate{}

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
