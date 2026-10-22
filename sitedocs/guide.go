// guide.go loads the application guide — the workbench root's llms.txt —
// and extracts the heading outline the docs tree shows. The Docs shell is
// one markdown document of that file (never split into per-section pages);
// the outline is its ## / ### skeleton, each entry carrying the block
// index that markdown.Document.ScrollToBlock / NewDocumentAt seat the
// reader at.
//
// Loading order: a checkout carries the file at ../llms.txt relative to
// the sitedocs directory (or ./llms.txt when the process runs from the
// workbench root); a `go run …@latest` install has no checkout, so the
// canonical raw URL is fetched instead and the bytes are kept in memory —
// nothing is written to disk. Tests exercise loadGuideFrom with fixture
// paths and a stub fetch; they never touch the network.

package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vibrantgio/markdown"
)

// guideCanonicalURL is the raw form of the guide as published on the
// workbench repository's default branch — the fallback source for
// installs that run outside a checkout.
const guideCanonicalURL = "https://raw.githubusercontent.com/vibrantgio/workbench/master/llms.txt"

// guidePaths are the checkout locations tried in order: the app normally
// runs with the sitedocs directory as its working directory (../llms.txt),
// but a `go run ./sitedocs` from the workbench root lands on ./llms.txt.
func guidePaths() []string {
	return []string{"../llms.txt", "llms.txt"}
}

// guideUnavailableLine is what the Docs column says when neither source
// answered. It is this app's own sentence about what is missing, it names
// no control, and it carries nothing the system said.
const guideUnavailableLine = "This guide can't be loaded."

// guideFailure is a guide that could not be read: the location it was
// looked for at, which the column stands under its line, and the system's
// own text saying why the last attempt failed, which goes to the log and
// nowhere else.
type guideFailure struct {
	Path string
	Err  string
}

// loadGuide returns the guide's markdown source: the checkout file if one
// is beside the process, the canonical raw URL otherwise. With neither
// answering it returns no source and the failure the column shows.
func loadGuide() ([]byte, *guideFailure) {
	return loadGuideFrom(guidePaths(), fetchGuide)
}

// loadGuideFrom is loadGuide's testable core: paths are tried in order,
// then fetch. A nil fetch skips the network leg, which is how tests
// guarantee no request is ever made.
//
// The cause of a failed read is written to the log here, with the
// location, because it is the only place that holds both and because the
// window never shows it.
func loadGuideFrom(paths []string, fetch func() ([]byte, error)) ([]byte, *guideFailure) {
	cause := "no source to read"
	for _, p := range paths {
		b, err := os.ReadFile(p)
		switch {
		case err != nil:
			cause = err.Error()
		case len(b) == 0:
			cause = fmt.Sprintf("read %s: the file is empty", p)
		default:
			return b, nil
		}
	}
	if fetch != nil {
		b, err := fetch()
		switch {
		case err != nil:
			cause = err.Error()
		case len(b) == 0:
			cause = fmt.Sprintf("fetch %s: the response is empty", guideCanonicalURL)
		default:
			return b, nil
		}
	}
	fail := &guideFailure{Path: guideLocation(paths), Err: cause}
	fmt.Fprintf(os.Stderr, "sitedocs: the guide at %s could not be loaded: %s\n", fail.Path, fail.Err)
	return nil, fail
}

// guideLocation is the location the column names and the log repeats:
// where the guide sits in a checkout, resolved against the process's own
// directory so the reader is told a place rather than a relative step.
// With no candidate path the canonical URL is the only location there is.
func guideLocation(paths []string) string {
	if len(paths) == 0 {
		return guideCanonicalURL
	}
	if abs, err := filepath.Abs(paths[0]); err == nil {
		return abs
	}
	return paths[0]
}

// fetchGuide downloads the canonical raw guide. The bytes live in process
// memory for the app's lifetime; nothing is cached to disk.
func fetchGuide() ([]byte, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(guideCanonicalURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: %s", guideCanonicalURL, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// outlineEntry is one ## section of the guide: its title, the index of
// its heading block in the parsed document, and its ### children. Deeper
// headings do not enter the outline and the lone # title is skipped — the
// document itself is the root.
type outlineEntry struct {
	Title    string
	Block    int
	Children []outlineEntry
}

// guideOutline extracts the ##/### outline from a parsed document. The
// entries come from the document's actual headings — never a hardcoded
// list — so the tree follows the file wherever it goes. A ### before the
// first ## has no parent row to disclose under and is dropped; a ## with
// no ### simply has no children.
func guideOutline(blocks []markdown.Block) []outlineEntry {
	var out []outlineEntry
	for i, b := range blocks {
		h, ok := b.(*markdown.Heading)
		if !ok {
			continue
		}
		switch h.Level {
		case 2:
			out = append(out, outlineEntry{Title: headingText(h), Block: i})
		case 3:
			if len(out) == 0 {
				continue
			}
			last := &out[len(out)-1]
			last.Children = append(last.Children, outlineEntry{Title: headingText(h), Block: i})
		}
	}
	return out
}

// headingText flattens a heading's inline spans to the plain text the
// outline row shows.
func headingText(h *markdown.Heading) string {
	var sb strings.Builder
	for _, s := range h.Spans {
		sb.WriteString(s.Text)
	}
	return strings.TrimSpace(sb.String())
}
