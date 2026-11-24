package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/mvu"
)

// newBoard is the file the app makes when the file it opens does not exist:
// three empty columns, written as the Obsidian Kanban plugin writes a board,
// with the front matter and the settings block that the plugin reads.
const newBoard = "---\n\nkanban-plugin: board\n\n---\n\n" +
	"## To Do\n\n\n\n" +
	"## In Progress\n\n\n\n" +
	"## Done\n\n\n\n" +
	"\n\n%% kanban:settings\n```\n{\"kanban-plugin\":\"board\"}\n```\n%%"

// stamp is what the app compares to see that the file changed on disk: its
// modification time and its size.
type stamp struct {
	mod  int64 // nanoseconds since the Unix epoch
	size int64
}

func stampOf(info fs.FileInfo) stamp { return stamp{mod: info.ModTime().UnixNano(), size: info.Size()} }

// openBoard reads the file at path. It writes [newBoard] there first when no
// file exists at path.
func openBoard(path string) (string, stamp, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", stamp{}, err
		}
		s, err := writeAtomic(path, newBoard)
		return newBoard, s, err
	}
	if err != nil {
		return "", stamp{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", stamp{}, err
	}
	return string(data), stampOf(info), nil
}

// writeAtomic writes data to a new file in path's folder and renames it over
// path, so that a reader sees the old file or the new one and never a part.
// It gives the stamp of the file it wrote. The new file keeps the old file's
// permissions.
func writeAtomic(path, data string) (stamp, error) {
	mode := fs.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return stamp{}, err
	}
	tmp := f.Name()
	_, werr := f.WriteString(data)
	if werr == nil {
		werr = f.Sync()
	}
	cerr := f.Close()
	if werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(tmp, mode)
	}
	if werr == nil {
		werr = os.Rename(tmp, path)
	}
	if werr != nil {
		os.Remove(tmp)
		return stamp{}, werr
	}
	info, err := os.Stat(path)
	if err != nil {
		return stamp{}, err
	}
	return stampOf(info), nil
}

// boardFile is the file the board is kept in. It serialises the app's
// writes and its reads of the stamp, so that the poll never reads the app's
// own write again and an older write never replaces a newer one.
type boardFile struct {
	path string

	mu sync.Mutex
	// seen is the stamp of the file as the app last read or wrote it.
	seen stamp
	// wrote is the sequence number of the last write that reached the disk.
	wrote int
}

// newBoardFile is the file at path, as the app read it with the stamp seen.
func newBoardFile(path string, seen stamp) *boardFile {
	return &boardFile{path: path, seen: seen}
}

// write writes data, the board's content at change number seq. A write
// whose seq is not above the last one written does nothing, because a newer
// content is on the disk already.
func (f *boardFile) write(seq int, data string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if seq <= f.wrote {
		return nil
	}
	s, err := writeAtomic(f.path, data)
	if err != nil {
		return err
	}
	f.seen, f.wrote = s, seq
	return nil
}

// poll reads the file when its stamp differs from the one the app last
// read or wrote. It reports false when the file did not change or cannot be
// read.
func (f *boardFile) poll() (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	info, err := os.Stat(f.path)
	if err != nil || stampOf(info) == f.seen {
		return "", false
	}
	data, err := os.ReadFile(f.path)
	if err != nil {
		return "", false
	}
	f.seen = stampOf(info)
	return string(data), true
}

// FileChanged is the file's content, read again because the file changed
// on disk.
type FileChanged struct {
	Data string
}

// pollInterval is how often the app looks at the file's stamp.
const pollInterval = time.Second

// watch polls f every interval and gives a [FileChanged] for each change
// on disk that the app did not write itself.
func watch(f *boardFile, interval time.Duration) rx.Observable[mvu.Message] {
	reads := rx.Map(rx.Ticker(interval, interval), func(time.Time) mvu.Message {
		data, ok := f.poll()
		if !ok {
			return nil
		}
		return FileChanged{Data: data}
	})
	return reads.Filter(func(m mvu.Message) bool { return m != nil })
}

// save is the command that writes data, the board's content at change
// number seq, to f.
func save(f *boardFile, seq int, data string) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		return nil, f.write(seq, data)
	})
}
