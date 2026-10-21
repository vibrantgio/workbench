// scan.go is the fence-aware index scanner: one pass of a small line
// scanner per file — never a full markdown parse — collecting per file
// its headings, block ids and outgoing wikilinks. Lines inside fenced
// code blocks and inline code spans contribute nothing, honouring the
// rule that code is never a link edge.
//
// # Two phases
//
// A vault of ten thousand notes on iCloud cannot be read before the
// window opens, so the scan is split. ListVault walks the directory
// entries alone and reads no file: it answers the paths, which is
// everything the tree needs, and the order the contents are read in —
// the folders newest modification first, so the notes worked on most
// recently fill in first. The contents then stream in behind it,
// scanStreams notes at a time, and are folded into the index in
// batches. The one-call synchronous walk the tests measure the
// streaming result against is ScanVault, which lives beside them in
// scan_test.go: nothing the application runs calls it.

package main

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/reactivego/rx"

	"github.com/vibrantgio/markdown/obsidian"
)

// Heading is one heading the scanner collected: its level (1–6) and its
// title text, in document order so heading paths can be resolved.
type Heading struct {
	Level int
	Title string
}

// FileScan is the scanner's record for one note. A record the listing
// has made but the contents phase has not reached yet carries its path
// alone: every reader of the index — the tree, the resolver, the
// backlinks — must cope with a note whose headings, block ids and links
// are still empty, and each of them already answers nothing for a note
// that holds none.
type FileScan struct {
	Path     string    // vault-relative, forward slashes
	Headings []Heading // in document order
	BlockIDs []string  // ^id anchors, without the caret
	Links    []string  // raw wikilink bodies, as written between the brackets

	// Placeholder marks a note iCloud keeps in the cloud: the vault shows
	// `name.md` and the disk holds `.name.md.icloud` in its place. The
	// note is listed under the name the vault shows, so the tree is
	// whole, and it is read after every note that is actually on disk.
	Placeholder bool
}

// Index is the scanned vault: every *.md below the root, in walk order.
type Index struct {
	Root  string
	Files []FileScan

	// slot is path → position in Files, and base is file name → the
	// positions of every note carrying it, in index order. Both are
	// filled when the index is built and never written again: a scan's
	// paths are fixed by its listing, and the contents phase folds each
	// note's scan into the slot its path was listed in. They are what
	// keeps the resolver's two searches — a path, and a name anywhere
	// below the root — off a walk of the whole vault, which the
	// backlinks pane would otherwise pay once per link every time a
	// batch of contents lands. An index built by hand carries neither,
	// and is searched one entry at a time.
	slot map[string]int
	base map[string][]int
}

// add appends a record and records where it sits, by path and by name.
func (idx *Index) add(f FileScan) {
	if idx.slot == nil {
		idx.slot = map[string]int{}
		idx.base = map[string][]int{}
	}
	at := len(idx.Files)
	idx.slot[f.Path] = at
	name := path.Base(f.Path)
	idx.base[name] = append(idx.base[name], at)
	idx.Files = append(idx.Files, f)
}

// named is the vault-relative paths of every note whose file name is
// name, in index order.
func (idx *Index) named(name string) []string {
	if idx.base != nil {
		var out []string
		for _, i := range idx.base[name] {
			out = append(out, idx.Files[i].Path)
		}
		return out
	}
	var out []string
	for i := range idx.Files {
		if path.Base(idx.Files[i].Path) == name {
			out = append(out, idx.Files[i].Path)
		}
	}
	return out
}

// slotOf is where a vault-relative path sits in Files.
func (idx *Index) slotOf(rel string) (int, bool) {
	if idx.slot != nil {
		i, ok := idx.slot[rel]
		return i, ok
	}
	for i := range idx.Files {
		if idx.Files[i].Path == rel {
			return i, true
		}
	}
	return 0, false
}

var (
	headingRe  = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	blockIDRe  = regexp.MustCompile(`\^([A-Za-z0-9-]+)\s*$`)
	wikilinkRe = regexp.MustCompile(`\[\[([^\[\]]+)\]\]`)
	fenceRe    = regexp.MustCompile("^(```+|~~~+)")
)

// ScanSource runs the fence-aware line scanner over one note's source.
// Frontmatter is split off first and contributes nothing.
func ScanSource(src []byte) FileScan {
	var out FileScan
	_, body := obsidian.SplitFrontMatter(src)
	var fence string // the opening fence marker while inside a fenced block
	for _, raw := range strings.Split(string(body), "\n") {
		line := strings.TrimSuffix(raw, "\r")
		trimmed := strings.TrimLeft(line, " \t")
		if fence != "" {
			if closesFence(trimmed, fence) {
				fence = ""
			}
			continue // fenced lines contribute no links or anchors
		}
		if m := fenceRe.FindString(trimmed); m != "" {
			fence = m
			continue
		}
		if m := headingRe.FindStringSubmatch(line); m != nil {
			out.Headings = append(out.Headings, Heading{Level: len(m[1]), Title: strings.TrimSpace(m[2])})
		}
		visible := stripInlineCode(line)
		for _, lm := range wikilinkRe.FindAllStringSubmatch(visible, -1) {
			out.Links = append(out.Links, lm[1])
		}
		if m := blockIDRe.FindStringSubmatch(visible); m != nil {
			out.BlockIDs = append(out.BlockIDs, m[1])
		}
	}
	return out
}

// closesFence reports whether a line (already left-trimmed) closes the
// fence opened by open: the same fence character repeated at least as
// many times, with nothing but whitespace after.
func closesFence(trimmed, open string) bool {
	ch := open[0]
	n := 0
	for n < len(trimmed) && trimmed[n] == ch {
		n++
	}
	if n < len(open) {
		return false
	}
	return strings.TrimSpace(trimmed[n:]) == ""
}

// stripInlineCode blanks `code` spans out of a line so their content
// contributes no links or anchors. An unpaired backtick opens no span
// and stays literal.
func stripInlineCode(line string) string {
	var b strings.Builder
	for {
		open := strings.IndexByte(line, '`')
		if open < 0 {
			b.WriteString(line)
			return b.String()
		}
		close := strings.IndexByte(line[open+1:], '`')
		if close < 0 {
			b.WriteString(line)
			return b.String()
		}
		b.WriteString(line[:open])
		b.WriteByte(' ')
		line = line[open+1+close+1:]
	}
}

// scanStreams is how many notes the contents phase reads at once. The
// phase is bound by what the filesystem answers rather than by this
// machine's cores — a note iCloud has yet to hand over is a round trip —
// so the number is a small fixed one rather than one taken off GOMAXPROCS.
const scanStreams = 8

// scanBatch is how many scanned notes one delivery carries. The rx
// library this app is built on has no buffer-by-time operator, so the
// batch is by count alone (rx.BufferCount), with the tail flushed when
// the streams complete.
const scanBatch = 32

// icloudSuffix is what macOS appends to the hidden stand-in it leaves on
// disk for a note iCloud has not downloaded.
const icloudSuffix = ".icloud"

// scanJob names one note the contents phase has to read: its
// vault-relative path, and whether the disk holds a placeholder where
// the note should be.
type scanJob struct {
	path        string
	placeholder bool
}

// ListVault walks the directory entries below root and reads no file. It
// answers the index of paths — every record holding its path alone — and
// the order the contents phase reads them in: the folders newest
// modification first, each folder's notes in walk order, every
// placeholder after all of them.
//
// A folder's modification time is the newest of its own notes' times,
// taken off the directory entries the walk already has, so the folders
// whose notes were written most recently are read first.
//
// Dot-directories and dot-files are skipped, the hidden iCloud
// placeholders excepted: those are listed under the name the vault
// shows.
func ListVault(root string) (*Index, []scanJob, error) {
	idx := &Index{Root: root}
	walkRoot := root
	if r, err := filepath.EvalSymlinks(root); err == nil {
		walkRoot = r
	}
	type folder struct {
		newest time.Time
		jobs   []scanJob
	}
	folders := map[string]*folder{}
	var dirs []string
	var placeholders []scanJob
	err := filepath.WalkDir(walkRoot, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if p == walkRoot {
				return walkErr
			}
			return nil
		}
		if d.IsDir() {
			if p != walkRoot && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		name, placeholder := listedName(d.Name())
		if name == "" {
			return nil
		}
		rel, err := filepath.Rel(walkRoot, p)
		if err != nil {
			return nil
		}
		dir := path.Dir(filepath.ToSlash(rel))
		if dir == "." {
			dir = ""
		}
		job := scanJob{path: path.Join(dir, name), placeholder: placeholder}
		idx.add(FileScan{Path: job.path, Placeholder: placeholder})
		f := folders[dir]
		if f == nil {
			f = &folder{}
			folders[dir] = f
			dirs = append(dirs, dir)
		}
		if info, err := d.Info(); err == nil && info.ModTime().After(f.newest) {
			f.newest = info.ModTime()
		}
		if placeholder {
			placeholders = append(placeholders, job)
		} else {
			f.jobs = append(f.jobs, job)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	// Newest folder first; folders written at the same moment keep the
	// walk's own order, so the listing is the same twice over.
	slices.SortStableFunc(dirs, func(a, b string) int {
		return folders[b].newest.Compare(folders[a].newest)
	})
	order := make([]scanJob, 0, len(idx.Files))
	for _, dir := range dirs {
		order = append(order, folders[dir].jobs...)
	}
	// A placeholder's own path holds no file until iCloud hands the note
	// over, so reading one costs a refusal at best and a download at
	// worst. Every note that is on disk is read before any of them.
	order = append(order, placeholders...)
	return idx, order, nil
}

// listedName is the vault's name for a directory entry, and whether the
// entry is iCloud's placeholder for it. An entry that is not a note —
// another extension, a dot-file that is not a placeholder — answers the
// empty string.
func listedName(entry string) (name string, placeholder bool) {
	name = entry
	if strings.HasPrefix(entry, ".") && strings.HasSuffix(entry, icloudSuffix) {
		name = strings.TrimSuffix(entry[1:], icloudSuffix)
		placeholder = true
	}
	if name == "" || strings.HasPrefix(name, ".") || !strings.EqualFold(filepath.Ext(name), ".md") {
		return "", false
	}
	return name, placeholder
}

// mergeScans returns the index with every scanned note folded into the
// slot its path was listed in. The result is a new index over a new
// Files slice rather than a write into the old one: the aside's backlink
// memo is keyed on the index pointer, so an index folded in place would
// leave the pane showing the citations of a vault that has since been
// read further.
func mergeScans(idx *Index, files []FileScan) *Index {
	if idx == nil {
		return nil
	}
	next := &Index{Root: idx.Root, Files: slices.Clone(idx.Files), slot: idx.slot, base: idx.base}
	for _, f := range files {
		if i, ok := next.slotOf(f.Path); ok {
			next.Files[i] = f
		}
	}
	return next
}

// downloaded returns the index with the placeholder flag taken off one
// note: the platform has written the file, so it is a note on disk like
// any other. The result is a new index over a new Files slice, for the
// reason mergeScans builds one.
func downloaded(idx *Index, rel string) *Index {
	if idx == nil {
		return nil
	}
	i, ok := idx.slotOf(rel)
	if !ok || !idx.Files[i].Placeholder {
		return idx
	}
	next := &Index{Root: idx.Root, Files: slices.Clone(idx.Files), slot: idx.slot, base: idx.base}
	next.Files[i].Placeholder = false
	return next
}

// readScan reads one note and scans it. A read that refuses answers the
// record the listing made: a placeholder's own path holds no file, and a
// note that disappeared between the listing and the read is no reason to
// drop its row out of the tree.
func readScan(root string, job scanJob) FileScan {
	listed := FileScan{Path: job.path, Placeholder: job.placeholder}
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(job.path)))
	if err != nil {
		return listed
	}
	f := ScanSource(src)
	f.Path = job.path
	f.Placeholder = job.placeholder
	return f
}

// dealJobs deals the ordered jobs round-robin over n streams, so every
// stream begins in the newest folder and the notes arrive in much the
// order they were listed in. Dealing in blocks instead would have one
// stream reading the newest folder while seven read the oldest.
func dealJobs(jobs []scanJob, n int) [][]scanJob {
	if n < 1 {
		n = 1
	}
	streams := make([][]scanJob, 0, n)
	for i := 0; i < n && i < len(jobs); i++ {
		streams = append(streams, nil)
	}
	for i, job := range jobs {
		streams[i%len(streams)] = append(streams[i%len(streams)], job)
	}
	return streams
}

// readStream reads one stream's notes in sequence: ConcatMap subscribes
// the next note's read only once the previous one has emitted, so the
// stream has one file open at a time and scanStreams of them run at once.
//
// stop is read between two notes. A scan the reader has left behind —
// another vault opened, a rescan started — stops reading there rather
// than walking the rest of a vault nothing downstream will take.
func readStream(root string, jobs []scanJob, stop *atomic.Bool) rx.Observable[FileScan] {
	return rx.ConcatMap(rx.From(jobs...), func(job scanJob) rx.Observable[FileScan] {
		return rx.Create[FileScan](func(index int) (FileScan, error, bool) {
			if index > 0 || (stop != nil && stop.Load()) {
				return FileScan{}, nil, true
			}
			return readScan(root, job), nil, false
		})
	})
}

// readBatches is the contents phase: the jobs dealt to scanStreams
// streams, the streams merged, and the merged notes gathered into
// batches of scanBatch with the tail flushed at the end.
func readBatches(root string, jobs []scanJob, stop *atomic.Bool) rx.Observable[[]FileScan] {
	streams := dealJobs(jobs, scanStreams)
	observables := make([]rx.Observable[FileScan], len(streams))
	for i, s := range streams {
		observables[i] = readStream(root, s, stop)
	}
	return rx.BufferCount(rx.MergeAll(rx.From(observables...)), scanBatch, scanBatch)
}
