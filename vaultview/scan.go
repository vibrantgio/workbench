// scan.go is the fence-aware index scanner: one pass of a small line
// scanner per file — never a full markdown parse — collecting per file
// its headings, block ids and outgoing wikilinks. Lines inside fenced
// code blocks and inline code spans contribute nothing, honouring the
// rule that code is never a link edge.

package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/vibrantgio/markdown/obsidian"
)

// Heading is one heading the scanner collected: its level (1–6) and its
// title text, in document order so heading paths can be resolved.
type Heading struct {
	Level int
	Title string
}

// FileScan is the scanner's record for one note.
type FileScan struct {
	Path     string    // vault-relative, forward slashes
	Headings []Heading // in document order
	BlockIDs []string  // ^id anchors, without the caret
	Links    []string  // raw wikilink bodies, as written between the brackets
}

// Index is the scanned vault: every *.md below the root, in walk order.
type Index struct {
	Root  string
	Files []FileScan
}

// ScanVault walks *.md files below root, skipping dot-directories, and
// scans each one. Unreadable subtrees and files are skipped rather than
// failing the scan; only an unreadable root is an error.
func ScanVault(root string) (*Index, error) {
	idx := &Index{Root: root}
	// A vault given as a symlink walks as its target — WalkDir would
	// otherwise see the root as a non-directory and find nothing. The
	// index keeps the path as given; the notes resolve through the link.
	walkRoot := root
	if r, err := filepath.EvalSymlinks(root); err == nil {
		walkRoot = r
	}
	err := filepath.WalkDir(walkRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if path == walkRoot {
				return walkErr
			}
			return nil
		}
		if d.IsDir() {
			if path != walkRoot && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") || !strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(walkRoot, path)
		if err != nil {
			return nil
		}
		f := ScanSource(src)
		f.Path = filepath.ToSlash(rel)
		idx.Files = append(idx.Files, f)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return idx, nil
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
