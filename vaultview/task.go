// task.go is the one write vaultview makes: flipping a GFM task marker
// in the file on disk. The click is the write — the command splices the
// character and calls WriteFile before returning a message, so a process
// kill after the click still sees the new marker. Reload and viewport
// seating happen off the bytes that just landed.
//
// The library does not write files. It reports which item was activated;
// this file owns the splice, the freshness check, and the reload.

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vibrantgio/markdown"
	"github.com/vibrantgio/markdown/obsidian"
	"github.com/vibrantgio/mvu"
)

const taskChangedOnDisk = "This note changed on disk; the checkbox was not written."

// toggleTaskCmd splices the marker and writes the file before it returns
// a message. The write is the click, not a later flush.
func toggleTaskCmd(vault string, n *Note, item *markdown.ListItem) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		return toggleTask(vault, n, item), nil
	})
}

// toggleTask is the body of the toggle command: freshness, splice, write,
// then a reload of the note that just landed. A stale file is not written;
// the note is re-read and the refusal is the message's err.
func toggleTask(vault string, n *Note, item *markdown.ListItem) taskToggled {
	out := taskToggled{vault: vault, path: n.Path}
	full := filepath.Join(vault, filepath.FromSlash(n.Path))

	if !unchangedOnDisk(vault, n) {
		out.err = taskChangedOnDisk
		fresh, err := LoadNote(vault, n.Path)
		if err != nil {
			out.err = err.Error()
			return out
		}
		out.note = fresh
		return out
	}

	spliced, err := spliceTask(n.Src, item)
	if err != nil {
		out.err = err.Error()
		return out
	}

	mode := os.FileMode(0o644)
	if fi, serr := os.Stat(full); serr == nil {
		mode = fi.Mode().Perm()
	}
	if err := os.WriteFile(full, spliced, mode); err != nil {
		out.err = err.Error()
		return out
	}

	fresh, err := LoadNote(vault, n.Path)
	if err != nil {
		out.err = err.Error()
		return out
	}
	out.note = fresh
	return out
}

// spliceTask returns a copy of src with the GFM task marker at item
// flipped: a check writes 'x', an uncheck writes a space. The opening
// bracket, the closing bracket, and every other byte stay identical.
// item.MarkerOffset is the offset in the body Parse received; the
// frontmatter split maps it onto src.
func spliceTask(src []byte, item *markdown.ListItem) ([]byte, error) {
	if item == nil || !item.Task {
		return nil, fmt.Errorf("not a task item")
	}
	off, err := taskFileOffset(src, item.MarkerOffset)
	if err != nil {
		return nil, err
	}
	out := bytes.Clone(src)
	if item.Checked {
		out[off+1] = ' '
	} else {
		out[off+1] = 'x'
	}
	return out, nil
}

// taskFileOffset maps a MarkerOffset in the body Parse received onto
// src, through the same frontmatter split LoadNote used. The returned
// offset is the opening '[' of the marker; off+1 is the character the
// splice writes.
func taskFileOffset(src []byte, marker int) (int, error) {
	_, body := obsidian.SplitFrontMatter(src)
	off := len(src) - len(body) + marker
	if off < 0 || off+2 >= len(src) {
		return 0, fmt.Errorf("task marker offset %d is out of range", marker)
	}
	if src[off] != '[' || src[off+2] != ']' {
		return 0, fmt.Errorf("task marker not at recorded offset")
	}
	return off, nil
}
