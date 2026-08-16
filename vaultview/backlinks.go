// backlinks.go computes the reverse link edges the aside panel lists.
// A backlink is earned by RESOLUTION, not by string match: each note's
// raw outgoing wikilink bodies run through the resolver, and a link
// counts only when it actually resolves to the current note — a link
// that resolves elsewhere, refuses as ambiguous, or names nothing
// contributes no edge. Pure functions over the scanned index.

package main

// Backlinks returns the vault-relative paths of every note holding at
// least one wikilink that resolves to the note at current, one entry per
// citing note however many of its links land there, in the index's file
// order. The current note never cites itself — a same-file [[#Heading]]
// hop is navigation within the note, not an edge into it. A nil index
// yields nothing.
func Backlinks(idx *Index, current string) []string {
	if idx == nil || current == "" {
		return nil
	}
	var out []string
	for i := range idx.Files {
		f := &idx.Files[i]
		if f.Path == current {
			continue
		}
		for _, body := range f.Links {
			res, err := Resolve(idx, f.Path, body)
			if err == nil && res.Path == current {
				out = append(out, f.Path)
				break
			}
		}
	}
	return out
}
