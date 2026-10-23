package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ScanVault is the one-call synchronous walk: every *.md below root,
// read and scanned in one pass. The application does not scan a vault
// this way — it lists first and streams the contents in batches
// (scan.go) — and this stands here because that is the only thing left
// that wants it: it is the single-walk answer the streaming scan's
// result is measured against, note for note.
//
// Unreadable subtrees and files are skipped rather than failing the
// scan; only an unreadable root is an error. The hidden iCloud
// placeholders are dot-files and are skipped with the rest of them,
// where the listing phase lists them under the name the vault shows.
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
		idx.add(f)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return idx, nil
}

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

	got := mustListDir(t, root)
	if len(got) != 2 {
		t.Fatalf("ListDir returned %d rows, want 2 (plain, vault): %v", len(got), got)
	}
	// The parent is NOT a row: the trail above the browser is how one goes
	// up, which is what the platform's open panel does.
	for _, e := range got {
		if e.Name == ".." {
			t.Errorf("ListDir returned a parent row %+v; the trail is how one goes up", e)
		}
	}
	plain, vault := got[0], got[1]
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

	got := mustListDir(t, root)
	if len(got) != 1 {
		t.Fatalf("ListDir returned %d rows, want 1 (Diarizer): %v", len(got), got)
	}
	row := got[0]
	if row.Name != "Diarizer" || !row.IsVault {
		t.Errorf("symlinked vault row = %+v, want the .obsidian marker under its link name", row)
	}
}

// TestListDirSortsNamesAsTheFileBrowserDoes holds the browser's listing in
// the one order every name-ordered list in the library sorts by
// (theme/system/naming), rather than the byte order os.ReadDir answers in: a
// run of digits reads as the number it spells, and case does not part a name
// from its neighbours.
func TestListDirSortsNamesAsTheFileBrowserDoes(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"Vault 10", "vault 2", "vault 1", "Vault 3", "apples"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	for _, e := range mustListDir(t, root) {
		got = append(got, e.Name)
	}
	want := []string{"apples", "vault 1", "vault 2", "Vault 3", "Vault 10"}
	if strings.Join(got, " | ") != strings.Join(want, " | ") {
		t.Errorf("listing is %v, want %v", got, want)
	}
}

// mustListDir lists a folder for the browser and fails the test when the
// read fails.
func mustListDir(t *testing.T, dir string) []DirEntry {
	t.Helper()
	entries, err := ListDir(dir)
	if err != nil {
		t.Fatalf("ListDir(%s) failed: %v", dir, err)
	}
	return entries
}
