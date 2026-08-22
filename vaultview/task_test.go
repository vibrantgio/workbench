package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/vibrantgio/markdown"
)

func TestToggleTaskWritesTheMarker(t *testing.T) {
	cases := []struct {
		name string
		src  string
		pick int // index in collectTasks
		want string
	}{
		{
			name: "check, no frontmatter",
			src:  "- [ ] open\n",
			want: "- [x] open\n",
		},
		{
			name: "uncheck [x], no frontmatter",
			src:  "- [x] done\n",
			pick: 0,
			want: "- [ ] done\n",
		},
		{
			name: "uncheck [X]",
			src:  "- [X] caps\n",
			want: "- [ ] caps\n",
		},
		{
			name: "frontmatter present",
			src:  "---\ntitle: Tasks\n---\n\n- [ ] open\n",
			want: "---\ntitle: Tasks\n---\n\n- [x] open\n",
		},
		{
			name: "nested item",
			src:  "- [ ] parent\n  - [x] child\n",
			pick: 1,
			want: "- [ ] parent\n  - [ ] child\n",
		},
		{
			name: "CRLF line ending",
			src:  "- [ ] open\r\n",
			want: "- [x] open\r\n",
		},
		{
			name: "surrounding bytes stay put",
			src:  "* [ ]  spaced\n",
			want: "* [x]  spaced\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model, note, full := taskModel(t, "t.md", tc.src)
			held := model.Notes["t.md"]
			seq := model.NavSeq
			items := collectTasks(note.Blocks)
			if tc.pick >= len(items) {
				t.Fatalf("pick %d, have %d tasks", tc.pick, len(items))
			}
			item := items[tc.pick]

			next, cmd := Update(model, ToggleTask{Path: "t.md", Item: item})
			if next.Notes["t.md"] != held {
				t.Fatal("Update(ToggleTask) replaced the note before the write")
			}

			msg, err := cmd.First()
			if err != nil {
				t.Fatalf("toggle command: %v", err)
			}
			got, err := os.ReadFile(full)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("after command, file = %q, want %q", got, tc.want)
			}

			tog, ok := msg.(taskToggled)
			if !ok {
				t.Fatalf("command returned %T, want taskToggled", msg)
			}
			if tog.err != "" {
				t.Fatalf("command err = %q, want none", tog.err)
			}

			next, _ = Update(next, tog)
			if next.Notes["t.md"] == held {
				t.Error("taskToggled mutated the cached *Note in place")
			}
			if next.NavSeq != seq {
				t.Errorf("NavSeq %d → %d; a toggle must not re-seat at the landing anchor", seq, next.NavSeq)
			}
			if next.Toasts.Len() != 0 {
				t.Errorf("successful toggle queued %d toasts", next.Toasts.Len())
			}
			fresh := collectTasks(next.Notes["t.md"].Blocks)
			if tc.pick >= len(fresh) {
				t.Fatalf("reloaded note has %d tasks, want at least %d", len(fresh), tc.pick+1)
			}
			if fresh[tc.pick].Checked == item.Checked {
				t.Errorf("reloaded task Checked = %v, want the flip", fresh[tc.pick].Checked)
			}
		})
	}
}

func TestToggleTaskRefusesStaleFile(t *testing.T) {
	src := "- [ ] open\n"
	other := "edited elsewhere\n"
	model, note, full := taskModel(t, "t.md", src)
	held := model.Notes["t.md"]
	item := collectTasks(note.Blocks)[0]

	if err := os.WriteFile(full, []byte(other), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(full, later, later); err != nil {
		t.Fatal(err)
	}

	next, cmd := Update(model, ToggleTask{Path: "t.md", Item: item})
	msg, err := cmd.First()
	if err != nil {
		t.Fatalf("toggle command: %v", err)
	}
	got, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != other {
		t.Fatalf("stale toggle wrote the file: got %q, want the external edit %q", got, other)
	}

	tog, ok := msg.(taskToggled)
	if !ok {
		t.Fatalf("command returned %T, want taskToggled", msg)
	}
	if tog.err != taskChangedOnDisk {
		t.Errorf("err = %q, want %q", tog.err, taskChangedOnDisk)
	}
	if tog.note == nil {
		t.Fatal("stale toggle did not re-read the note")
	}

	next, _ = Update(next, tog)
	if next.Notes["t.md"] == held {
		t.Error("stale reload mutated the cached *Note in place")
	}
	if next.Toasts.Len() != 1 {
		t.Fatalf("toast queue length %d, want 1", next.Toasts.Len())
	}
	if got := next.Toasts.Items()[0].Text; got != taskChangedOnDisk {
		t.Errorf("toast = %q, want %q", got, taskChangedOnDisk)
	}
	if p := firstParagraph(next.Notes["t.md"]); p != "edited elsewhere" {
		t.Errorf("re-read note reads %q, want the external edit", p)
	}
}

func TestRebuildDocumentSeatsAtPreviousViewport(t *testing.T) {
	src := "# T\n\n## One\n\ntext\n\n## Two\n\n- [ ] task\n\n## Three\n\n## Four\n\n## Five\n"
	a := noteFromSource("t.md", src)
	prev := markdown.NewDocumentAt(a.Blocks, 4)
	b := noteFromSource("t.md", strings.Replace(src, "[ ]", "[x]", 1))
	got := rebuildDocument(b, prev)
	if got.Position().First != 4 {
		t.Errorf("rebuilt First = %d, want 4 — the reader jumped", got.Position().First)
	}
	fresh := rebuildDocument(b, nil)
	if fresh.Position().First != 0 {
		t.Errorf("fresh document First = %d, want 0", fresh.Position().First)
	}
}

func TestSpliceTaskKeepsEveryOtherByte(t *testing.T) {
	src := []byte("---\ntitle: T\n---\n\n- [X] caps\n  - [ ] child\n")
	n := noteFromSource("t.md", string(src))
	items := collectTasks(n.Blocks)
	if len(items) != 2 {
		t.Fatalf("got %d tasks, want 2", len(items))
	}

	out, err := spliceTask(src, items[0])
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("---\ntitle: T\n---\n\n- [ ] caps\n  - [ ] child\n")
	if string(out) != string(want) {
		t.Errorf("uncheck [X] = %q, want %q", out, want)
	}

	out, err = spliceTask(src, items[1])
	if err != nil {
		t.Fatal(err)
	}
	want = []byte("---\ntitle: T\n---\n\n- [X] caps\n  - [x] child\n")
	if string(out) != string(want) {
		t.Errorf("check nested = %q, want %q", out, want)
	}

	if string(src) != "---\ntitle: T\n---\n\n- [X] caps\n  - [ ] child\n" {
		t.Error("spliceTask mutated the input bytes")
	}
}

func taskModel(t *testing.T, rel, src string) (Model, *Note, string) {
	t.Helper()
	root := t.TempDir()
	full := writeNote(t, root, rel, src)
	n, err := LoadNote(root, rel)
	if err != nil {
		t.Fatal(err)
	}
	model := Model{Screen: screenVault, Vault: root, CurAnchor: -1}
	model = cacheNote(model, n)
	model.Current = rel
	model.History = []HistEntry{{Path: rel, Anchor: -1}}
	return model, n, full
}

func collectTasks(blocks []markdown.Block) []*markdown.ListItem {
	var out []*markdown.ListItem
	var walk func([]markdown.Block)
	walk = func(bs []markdown.Block) {
		for _, b := range bs {
			switch b := b.(type) {
			case *markdown.List:
				for _, it := range b.Items {
					if it.Task {
						out = append(out, it)
					}
					walk(it.Blocks)
				}
			case *markdown.Blockquote:
				walk(b.Blocks)
			}
		}
	}
	walk(blocks)
	return out
}
