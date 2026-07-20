// markdown.go renders message bodies through the vibrantgio/markdown
// module's chat subset: inline styles (bold/italic/code/links/
// strikethrough) on prism/richtext plus fenced code blocks. Every other
// block construct — headings, lists, blockquotes, tables, images, rules —
// degrades to plain paragraphs preserving its inline runs, so a chat bubble
// never grows document chrome. Link clicks open in the system browser.

package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/vibrantgio/markdown"
)

// msgRow is one rendered history entry: the message plus, for user and
// assistant rows, the parsed markdown document the bubble lays out. Error
// and status rows keep the plain label (Doc == nil).
type msgRow struct {
	Msg Message
	Doc *markdown.Document
}

// docCache maps history rows onto their markdown Documents across model
// emissions, so a stable message keeps its Document — and with it the
// richtext link interaction state — while the model re-emits around it.
// Rows is called at emission scope only (inside the view's combine Map);
// the frame never touches the cache, only the msgRow slice it produced.
type docCache struct {
	docs map[string]*markdown.Document
}

func newDocCache() *docCache {
	return &docCache{docs: map[string]*markdown.Document{}}
}

// Rows resolves the visible history to message rows. Each user/assistant
// row's key is its index plus content, so a streaming delta re-parses just
// the row it grew while every settled row hits the cache. Keys absent from
// the new history are dropped, so deltas and chat switches never leak
// Documents.
func (c *docCache) Rows(history []Message) []msgRow {
	next := make(map[string]*markdown.Document, len(history))
	rows := make([]msgRow, len(history))
	for i, msg := range history {
		rows[i] = msgRow{Msg: msg}
		if msg.Role != RoleUser && msg.Role != RoleAssistant {
			continue
		}
		key := rowKey(i, msg)
		doc := c.docs[key]
		if doc == nil {
			doc = markdown.NewDocument(degrade(markdown.Parse(messageSource(msg))))
		}
		next[key] = doc
		rows[i].Doc = doc
	}
	c.docs = next
	return rows
}

// rowKey identifies one history row for the cache: position, role, content,
// and citations (which arrive incrementally during a stream).
func rowKey(i int, msg Message) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d\x00%s\x00%s", i, msg.Role, msg.Content)
	for _, cit := range msg.Citations {
		b.WriteString("\x00" + cit.Title + "\x01" + cit.URL)
	}
	return b.String()
}

// messageSource is the markdown source a message row parses: the body plus,
// for answers with web-search citations, a trailing Sources list. Bare URLs
// autolink (GFM), so each source line is clickable.
func messageSource(msg Message) []byte {
	var b strings.Builder
	b.WriteString(msg.Content)
	if len(msg.Citations) > 0 {
		b.WriteString("\n\nSources:\n")
		for _, c := range msg.Citations {
			b.WriteString("\n- ")
			// xAI titles inline citations with their bare marker number;
			// a title that adds nothing over the URL is dropped.
			if c.Title != "" && strings.Trim(c.Title, "0123456789") != "" {
				b.WriteString(c.Title + " — ")
			}
			b.WriteString(c.URL)
		}
	}
	return []byte(b.String())
}

// degrade maps a parsed block tree onto the chat subset: paragraphs and
// code blocks pass through, everything else flattens to plain paragraphs
// preserving its inline runs — headings lose their scale, blockquotes their
// bar, list items keep a textual marker, table rows join their cells, images
// fall back to their alt text, and rules (no inline content) drop.
func degrade(blocks []markdown.Block) []markdown.Block {
	var out []markdown.Block
	for _, b := range blocks {
		switch b := b.(type) {
		case *markdown.Paragraph, *markdown.CodeBlock:
			out = append(out, b)
		case *markdown.Heading:
			out = append(out, &markdown.Paragraph{Spans: b.Spans})
		case *markdown.Blockquote:
			out = append(out, degrade(b.Blocks)...)
		case *markdown.List:
			out = append(out, degradeList(b)...)
		case *markdown.Table:
			out = append(out, degradeTable(b)...)
		case *markdown.Image:
			alt := b.Alt
			if alt == "" {
				alt = b.URL
			}
			out = append(out, &markdown.Paragraph{Spans: []markdown.Span{{Text: alt}}})
		case *markdown.Rule:
			// A rule carries no inline runs; nothing to degrade to.
		}
	}
	return out
}

// degradeList flattens a list's items into paragraphs, prefixing each item's
// first paragraph with a textual marker ("• " or "3. "). Nested lists
// recurse through the item content, their items keeping their own markers.
func degradeList(l *markdown.List) []markdown.Block {
	var out []markdown.Block
	for i, item := range l.Items {
		marker := "• "
		if l.Ordered {
			marker = fmt.Sprintf("%d. ", l.Start+i)
		}
		blocks := degrade(item.Blocks)
		if len(blocks) == 0 {
			continue
		}
		if p, ok := blocks[0].(*markdown.Paragraph); ok {
			p.Spans = append([]markdown.Span{{Text: marker}}, p.Spans...)
		} else {
			blocks = append([]markdown.Block{&markdown.Paragraph{Spans: []markdown.Span{{Text: marker}}}}, blocks...)
		}
		out = append(out, blocks...)
	}
	return out
}

// degradeTable flattens a table into one paragraph per row (header first),
// cells joined with a " | " separator, inline runs preserved.
func degradeTable(t *markdown.Table) []markdown.Block {
	var out []markdown.Block
	rows := append([][]*markdown.TableCell{t.Header}, t.Rows...)
	for _, row := range rows {
		var spans []markdown.Span
		for ci, cell := range row {
			if ci > 0 {
				spans = append(spans, markdown.Span{Text: " | "})
			}
			spans = append(spans, cell.Spans...)
		}
		if len(spans) > 0 {
			out = append(out, &markdown.Paragraph{Spans: spans})
		}
	}
	return out
}

// openURL opens an absolute web URL in the system browser — the app-layer
// OS-open handler behind richtext's OnLinkClick. Non-http(s) destinations
// are ignored.
func openURL(url string) {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
