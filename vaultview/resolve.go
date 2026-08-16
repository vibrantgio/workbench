// resolve.go implements wikilink resolution as pure functions over the
// scanned index. The rules are the documented Obsidian semantics,
// reimplemented in-house: a link body names a file part (optionally with
// a path, conventionally without ".md"), then a heading path or a block
// id. File resolution tries the part relative to the linking note's
// directory first, then against the vault root, then as a unique basename
// anywhere below the root; two or more basename hits refuse rather than
// guess. Heading paths descend the target's heading list by title,
// case-insensitively, and an ambiguous title at any level refuses the
// same way. Block ids are file-unique by the stamp contract, so they
// either exist or they don't.
//
// The resolver answers over the line scanner's index only — "does this
// anchor exist". The block index the viewport actually lands on is
// computed from the parsed blocks by AnchorBlock, so the scroll target
// and the rendered content can never disagree.

package main

import (
	"fmt"
	"path"
	"strings"

	"github.com/vibrantgio/markdown"
)

// Ref is a parsed wikilink body: the file part as written (empty for a
// same-file link), the heading path, and the block id without its caret.
// The alias after the first pipe is display-only and already dropped.
type Ref struct {
	File     string
	Headings []string
	BlockID  string
}

// ParseRef splits a raw wikilink body into its parts.
func ParseRef(body string) Ref {
	target, _, _ := strings.Cut(body, "|")
	parts := strings.Split(target, "#")
	r := Ref{File: strings.TrimSpace(parts[0])}
	for _, p := range parts[1:] {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "^") {
			r.BlockID = p[1:]
		} else {
			r.Headings = append(r.Headings, p)
		}
	}
	return r
}

// Resolved names the note a wikilink lands on: its vault-relative path
// and the validated anchor, if the link carried one.
type Resolved struct {
	Path     string
	Headings []string
	BlockID  string
}

// ResolveErr explains a refused resolution. Reason is user-facing toast
// text; Candidates lists the vault-relative paths an ambiguous file part
// matched, for a future chooser surface.
type ResolveErr struct {
	Reason     string
	Candidates []string
}

func (e *ResolveErr) Error() string { return e.Reason }

// Resolve resolves one raw wikilink body written in the note at from
// (vault-relative) against the index. It never guesses: a file part that
// matches nothing or more than one basename refuses, and so does a
// heading path that is absent or ambiguous in the target.
func Resolve(idx *Index, from, body string) (Resolved, *ResolveErr) {
	ref := ParseRef(body)
	target := from
	if ref.File != "" {
		p, err := resolveFile(idx, from, ref.File)
		if err != nil {
			return Resolved{}, err
		}
		target = p
	}
	f := idx.file(target)
	if f == nil {
		return Resolved{}, &ResolveErr{Reason: fmt.Sprintf("no note %q in this vault", displayTarget(ref, target))}
	}
	if err := checkHeadingPath(f, ref.Headings); err != nil {
		return Resolved{}, err
	}
	if ref.BlockID != "" && !containsString(f.BlockIDs, ref.BlockID) {
		return Resolved{}, &ResolveErr{Reason: fmt.Sprintf("no block ^%s in %q", ref.BlockID, noteTitle(target))}
	}
	return Resolved{Path: target, Headings: ref.Headings, BlockID: ref.BlockID}, nil
}

// displayTarget is what a refusal names: the file part as written, or the
// resolved path for a same-file link.
func displayTarget(ref Ref, target string) string {
	if ref.File != "" {
		return ref.File
	}
	return target
}

// resolveFile applies the file-resolution order: (1) the part as written
// relative to the linking note's directory, (2) against the vault root —
// in both, ".md" is appended when the link omits it, since wikilinks name
// notes without the extension by convention — then (3) as a unique
// basename anywhere below the root. Comparisons are exact.
func resolveFile(idx *Index, from, file string) (string, *ResolveErr) {
	names := []string{file}
	if !strings.EqualFold(path.Ext(file), ".md") {
		names = append(names, file+".md")
	}
	// (1) relative to the linking note's directory.
	dir := path.Dir(from)
	for _, n := range names {
		if p := path.Clean(path.Join(dir, n)); idx.file(p) != nil {
			return p, nil
		}
	}
	// (2) against the vault root.
	for _, n := range names {
		if p := path.Clean(n); idx.file(p) != nil {
			return p, nil
		}
	}
	// (3) unique basename anywhere below the root.
	base := path.Base(file)
	if !strings.EqualFold(path.Ext(base), ".md") {
		base += ".md"
	}
	var hits []string
	for i := range idx.Files {
		if path.Base(idx.Files[i].Path) == base {
			hits = append(hits, idx.Files[i].Path)
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return "", &ResolveErr{Reason: fmt.Sprintf("no note %q in this vault", file)}
	default:
		return "", &ResolveErr{Reason: fmt.Sprintf("%q matches %d notes", file, len(hits)), Candidates: hits}
	}
}

// checkHeadingPath descends f's heading list by title, case-insensitively.
// Each matched segment narrows the window to its own section — the
// headings after it, up to the first heading at its level or above — and
// the next segment must match inside that window. Absent and ambiguous
// titles both refuse.
func checkHeadingPath(f *FileScan, titles []string) *ResolveErr {
	lo, hi := 0, len(f.Headings)
	for _, title := range titles {
		var hits []int
		for i := lo; i < hi; i++ {
			if strings.EqualFold(f.Headings[i].Title, title) {
				hits = append(hits, i)
			}
		}
		switch len(hits) {
		case 0:
			return &ResolveErr{Reason: fmt.Sprintf("no heading %q in %q", title, noteTitle(f.Path))}
		case 1:
			// descend
		default:
			return &ResolveErr{Reason: fmt.Sprintf("heading %q matches %d sections in %q", title, len(hits), noteTitle(f.Path))}
		}
		at := hits[0]
		level := f.Headings[at].Level
		lo = at + 1
		for j := lo; j < hi; j++ {
			if f.Headings[j].Level <= level {
				hi = j
				break
			}
		}
	}
	return nil
}

// file returns the scan record for a vault-relative path, nil when the
// path is not in the vault.
func (idx *Index) file(rel string) *FileScan {
	for i := range idx.Files {
		if idx.Files[i].Path == rel {
			return &idx.Files[i]
		}
	}
	return nil
}

// noteTitle is a note's display name: the basename without the extension.
func noteTitle(rel string) string {
	base := path.Base(rel)
	return strings.TrimSuffix(base, path.Ext(base))
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// AnchorBlock computes the top-level block index a resolved anchor lands
// on, from the note's parsed blocks — never from the line scanner, so the
// viewport target and the rendered content cannot disagree. A block id
// wins over a heading path when both are present; ok is false when the
// link carried no anchor or the parsed blocks disagree with the scan.
func AnchorBlock(n *Note, headings []string, blockID string) (int, bool) {
	if blockID != "" {
		at, ok := n.Anchors[blockID]
		return at, ok
	}
	if len(headings) > 0 {
		return headingBlockIndex(n.Blocks, headings)
	}
	return 0, false
}

// headingBlockIndex descends the parsed top-level heading blocks by title
// with the same window rule checkHeadingPath applies to the scan, and
// returns the matched heading's block index. The resolver has already
// refused ambiguity, so the first match in the window is the match.
func headingBlockIndex(blocks []markdown.Block, titles []string) (int, bool) {
	lo, hi := 0, len(blocks)
	found := -1
	for _, title := range titles {
		found = -1
		var level int
		for i := lo; i < hi; i++ {
			h, ok := blocks[i].(*markdown.Heading)
			if !ok {
				continue
			}
			if strings.EqualFold(spanText(h.Spans), title) {
				found = i
				level = h.Level
				break
			}
		}
		if found < 0 {
			return 0, false
		}
		lo = found + 1
		for j := lo; j < hi; j++ {
			if h, ok := blocks[j].(*markdown.Heading); ok && h.Level <= level {
				hi = j
				break
			}
		}
	}
	return found, found >= 0
}

// spanText is the plain text of a span run.
func spanText(spans []markdown.Span) string {
	var b strings.Builder
	for _, s := range spans {
		b.WriteString(s.Text)
	}
	return b.String()
}
