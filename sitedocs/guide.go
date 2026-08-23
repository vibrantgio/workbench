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

// guideUnavailable is the in-memory stand-in shown when neither a checkout
// file nor the network can produce the guide, so the app still opens with
// a document rather than a blank pane.
const guideUnavailable = `# Vibrant Gio — application guide

The guide could not be loaded.

## Where it lives

This app renders llms.txt from the workbench repository: in a checkout it
sits next to the sitedocs directory, and otherwise it is fetched from
` + guideCanonicalURL + `. Neither source answered — check the checkout
location or the network connection and relaunch.
`

// loadGuide returns the guide's markdown source: the checkout file if one
// is beside the process, the canonical raw URL otherwise, and the
// in-memory notice if both fail.
func loadGuide() []byte {
	return loadGuideFrom(guidePaths(), fetchGuide)
}

// loadGuideFrom is loadGuide's testable core: paths are tried in order,
// then fetch, then the unavailable notice. A nil fetch skips the network
// leg, which is how tests guarantee no request is ever made.
func loadGuideFrom(paths []string, fetch func() ([]byte, error)) []byte {
	for _, p := range paths {
		if b, err := os.ReadFile(p); err == nil && len(b) > 0 {
			return b
		}
	}
	if fetch != nil {
		if b, err := fetch(); err == nil && len(b) > 0 {
			return b
		}
	}
	return []byte(guideUnavailable)
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
// headings are not outline material and the lone # title is skipped — the
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
