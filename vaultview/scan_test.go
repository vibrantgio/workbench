package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestScanSource(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		headings []Heading
		blockIDs []string
		links    []string
	}{
		{
			name: "headings collected with level and title",
			src:  "# Top\n\nprose\n\n## Second level\n###### Deep\n",
			headings: []Heading{
				{Level: 1, Title: "Top"},
				{Level: 2, Title: "Second level"},
				{Level: 6, Title: "Deep"},
			},
		},
		{
			name:  "wikilinks collected raw, alias and heading path intact",
			src:   "See [[Other Note|the alias]] and [[Folder/Deep#Sec]].\nAn embed ![[img.png]] counts too.\n",
			links: []string{"Other Note|the alias", "Folder/Deep#Sec", "img.png"},
		},
		{
			name:  "fenced wikilink contributes nothing",
			src:   "before\n\n```\n[[not-a-link]]\n# not-a-heading\n^not-an-id\n```\n\nafter [[real]]\n",
			links: []string{"real"},
		},
		{
			name:  "tilde fences hide content the same way",
			src:   "~~~text\n[[not-a-link]]\n~~~\n[[real]]\n",
			links: []string{"real"},
		},
		{
			name:  "a longer closing fence closes a shorter opener",
			src:   "````\n[[not-a-link]]\n`````\n[[real]]\n",
			links: []string{"real"},
		},
		{
			name:  "an unterminated fence hides the rest of the note",
			src:   "```\n[[not-a-link]]\n^nope\n",
			links: nil,
		},
		{
			name:  "inline code is never an edge",
			src:   "a `[[not-a-link]]` b [[real]]\n",
			links: []string{"real"},
		},
		{
			name:     "block id tails, inline and on their own line",
			src:      "A paragraph. ^para-1\n\n| a | b |\n\n^table-id\n",
			blockIDs: []string{"para-1", "table-id"},
		},
		{
			name:     "an invalid block id is not collected",
			src:      "No underscores. ^bad_id\n",
			blockIDs: nil,
		},
		{
			name: "frontmatter contributes nothing",
			src:  "---\ntitle: Has [[link-in-frontmatter]]\n---\n# Real\n",
			headings: []Heading{
				{Level: 1, Title: "Real"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ScanSource([]byte(tc.src))
			if !reflect.DeepEqual(got.Headings, tc.headings) {
				t.Errorf("Headings = %v, want %v", got.Headings, tc.headings)
			}
			if !reflect.DeepEqual(got.BlockIDs, tc.blockIDs) {
				t.Errorf("BlockIDs = %v, want %v", got.BlockIDs, tc.blockIDs)
			}
			if !reflect.DeepEqual(got.Links, tc.links) {
				t.Errorf("Links = %v, want %v", got.Links, tc.links)
			}
		})
	}
}

func TestScanVaultWalksBelowRootSkippingDotDirectories(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.md", "# A\n[[b]]\n")
	write("sub/b.md", "# B\n")
	write("sub/notes.txt", "not markdown")
	write(".obsidian/workspace.md", "# hidden\n")
	write(".hidden/c.md", "# hidden\n")
	write(".dotfile.md", "# hidden\n")

	idx, err := ScanVault(root)
	if err != nil {
		t.Fatalf("ScanVault: %v", err)
	}
	var paths []string
	for _, f := range idx.Files {
		paths = append(paths, f.Path)
	}
	want := []string{"a.md", "sub/b.md"}
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("scanned files = %v, want %v", paths, want)
	}
	if got := idx.Files[0].Links; !reflect.DeepEqual(got, []string{"b"}) {
		t.Errorf("a.md links = %v, want [b]", got)
	}
}

func TestListDir(t *testing.T) {
	root := t.TempDir()
	mk := func(rel string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(rel)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	touch := func(rel string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("vault/.obsidian")
	touch("vault/note.md")
	mk("plain")
	touch("plain/one.md")
	touch("plain/two.md")
	touch("plain/other.txt")
	mk(".hidden")
	touch("stray.md")

	got := ListDir(root)
	if len(got) != 3 {
		t.Fatalf("ListDir returned %d rows, want 3 (parent, plain, vault): %v", len(got), got)
	}
	if !got[0].Up || got[0].Name != ".." {
		t.Errorf("first row = %+v, want the parent row", got[0])
	}
	plain, vault := got[1], got[2]
	if plain.Name != "plain" || plain.IsVault || plain.MDCount != 2 {
		t.Errorf("plain row = %+v, want 2 notes and no vault marker", plain)
	}
	if vault.Name != "vault" || !vault.IsVault {
		t.Errorf("vault row = %+v, want the .obsidian marker", vault)
	}
	for i, e := range got {
		if e.Idx != i {
			t.Errorf("row %d carries Idx %d", i, e.Idx)
		}
	}
}

func TestScanVaultFollowsSymlinkRoot(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real-vault")
	if err := os.MkdirAll(filepath.Join(real, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, content := range map[string]string{
		"a.md":     "# A\n[[b]]\n",
		"sub/b.md": "# B\n",
	} {
		if err := os.WriteFile(filepath.Join(real, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(base, "linked-vault")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	idx, err := ScanVault(link)
	if err != nil {
		t.Fatalf("ScanVault through symlink: %v", err)
	}
	if idx.Root != link {
		t.Errorf("Index.Root = %q, want the path as given %q", idx.Root, link)
	}
	var paths []string
	for _, f := range idx.Files {
		paths = append(paths, f.Path)
	}
	want := []string{"a.md", "sub/b.md"}
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("scanned files = %v, want %v", paths, want)
	}
	// The paths must resolve when joined onto the symlinked root.
	for _, p := range want {
		if _, err := os.Stat(filepath.Join(link, filepath.FromSlash(p))); err != nil {
			t.Errorf("note %s does not resolve through the link: %v", p, err)
		}
	}
}

func TestListDirShowsSymlinkedFolders(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "elsewhere", "real-vault")
	if err := os.MkdirAll(filepath.Join(real, ".obsidian"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "note.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "browse")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(root, "Diarizer")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "loose.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	got := ListDir(root)
	if len(got) != 2 {
		t.Fatalf("ListDir returned %d rows, want 2 (parent, Diarizer): %v", len(got), got)
	}
	row := got[1]
	if row.Name != "Diarizer" || !row.IsVault {
		t.Errorf("symlinked vault row = %+v, want the .obsidian marker under its link name", row)
	}
}
