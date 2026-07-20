package main

// Tests for the message-body markdown pipeline: representative bodies parse
// into the chat subset — inline styles and code fences pass through, every
// other block degrades to plain paragraphs preserving its inline runs — and
// the doc cache keeps a stable message's Document across emissions.

import (
	"testing"

	"github.com/vibrantgio/markdown"
)

// parseBody runs a message body through the same pipeline MessageRow
// renders: parse, then degrade to the chat subset.
func parseBody(body string) []markdown.Block {
	return degrade(markdown.Parse([]byte(body)))
}

// paragraph asserts the block is a paragraph and returns it.
func paragraph(t *testing.T, b markdown.Block) *markdown.Paragraph {
	t.Helper()
	p, ok := b.(*markdown.Paragraph)
	if !ok {
		t.Fatalf("block is %T, want *markdown.Paragraph", b)
	}
	return p
}

// TestInlineStylesParagraph parses a representative chat message with every
// inline style: bold, italic, code, link, strikethrough.
func TestInlineStylesParagraph(t *testing.T) {
	blocks := parseBody("Use **bold**, *italic*, `code`, [a link](https://gioui.org) and ~~gone~~.")
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(blocks))
	}
	spans := paragraph(t, blocks[0]).Spans

	find := func(text string) markdown.Span {
		for _, s := range spans {
			if s.Text == text {
				return s
			}
		}
		t.Fatalf("no span %q in %#v", text, spans)
		return markdown.Span{}
	}
	if s := find("bold"); !s.Bold {
		t.Errorf("bold span = %+v, want Bold", s)
	}
	if s := find("italic"); !s.Italic {
		t.Errorf("italic span = %+v, want Italic", s)
	}
	if s := find("code"); !s.Code {
		t.Errorf("code span = %+v, want Code", s)
	}
	if s := find("a link"); s.URL != "https://gioui.org" {
		t.Errorf("link span URL = %q, want https://gioui.org", s.URL)
	}
	if s := find("gone"); !s.Strikethrough {
		t.Errorf("strikethrough span = %+v, want Strikethrough", s)
	}
}

// TestCodeFencePassesThrough keeps a fenced code block — with its language —
// alongside the prose.
func TestCodeFencePassesThrough(t *testing.T) {
	blocks := parseBody("Try this:\n\n```go\nfunc main() {}\n```\n")
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2 (paragraph + code block)", len(blocks))
	}
	cb, ok := blocks[1].(*markdown.CodeBlock)
	if !ok {
		t.Fatalf("second block is %T, want *markdown.CodeBlock", blocks[1])
	}
	if cb.Language != "go" || cb.Code != "func main() {}" {
		t.Errorf("code block = %+v, want language go with the fence body", cb)
	}
}

// TestBlockquoteDegradesToParagraph flattens a quoted message to a plain
// paragraph that keeps its inline styling.
func TestBlockquoteDegradesToParagraph(t *testing.T) {
	blocks := parseBody("> quoted **wisdom** here\n")
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(blocks))
	}
	spans := paragraph(t, blocks[0]).Spans
	bold := false
	for _, s := range spans {
		if s.Text == "wisdom" && s.Bold {
			bold = true
		}
	}
	if !bold {
		t.Errorf("degraded quote spans %#v lost the bold run", spans)
	}
}

// TestBlocksDegradeToParagraphs maps the remaining unsupported constructs
// down: headings flatten, list items keep textual markers, table rows join
// their cells, images fall back to alt text, rules drop.
func TestBlocksDegradeToParagraphs(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		wantFirst  string // first span text of the first block
		wantBlocks int
	}{
		{"heading", "# Title\n", "Title", 1},
		{"bullet list", "- alpha\n- beta\n", "• ", 2},
		{"ordered list", "3. alpha\n4. beta\n", "3. ", 2},
		{"table", "| a | b |\n|---|---|\n| c | d |\n", "a", 2},
		{"image", "![alt text](pic.png)\n", "alt text", 1},
		{"rule", "---\n", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			blocks := parseBody(tc.body)
			if len(blocks) != tc.wantBlocks {
				t.Fatalf("got %d blocks, want %d", len(blocks), tc.wantBlocks)
			}
			for _, b := range blocks {
				if _, ok := b.(*markdown.Paragraph); !ok {
					t.Fatalf("degraded block is %T, want *markdown.Paragraph", b)
				}
			}
			if tc.wantBlocks == 0 {
				return
			}
			spans := paragraph(t, blocks[0]).Spans
			if len(spans) == 0 || spans[0].Text != tc.wantFirst {
				t.Errorf("first span = %#v, want text %q", spans, tc.wantFirst)
			}
		})
	}
}

// TestOrderedMarkersCount numbers every item from the list's start.
func TestOrderedMarkersCount(t *testing.T) {
	blocks := parseBody("1. one\n2. two\n3. three\n")
	want := []string{"1. ", "2. ", "3. "}
	if len(blocks) != len(want) {
		t.Fatalf("got %d blocks, want %d", len(blocks), len(want))
	}
	for i, w := range want {
		if spans := paragraph(t, blocks[i]).Spans; spans[0].Text != w {
			t.Errorf("item %d marker = %q, want %q", i, spans[0].Text, w)
		}
	}
}

// TestCitationsAutolink appends an answer's citations as a Sources list
// whose bare URLs autolink, so each source is clickable.
func TestCitationsAutolink(t *testing.T) {
	msg := Message{
		Role:    RoleAssistant,
		Content: "Go 1.26 is out.",
		Citations: []Citation{
			{URL: "https://go.dev/blog", Title: "The Go Blog"},
			{URL: "https://tip.golang.org", Title: "42"}, // numeric title drops
		},
	}
	blocks := degrade(markdown.Parse(messageSource(msg)))
	var links []markdown.Span
	for _, b := range blocks {
		if p, ok := b.(*markdown.Paragraph); ok {
			for _, s := range p.Spans {
				if s.URL != "" {
					links = append(links, s)
				}
			}
		}
	}
	if len(links) != 2 {
		t.Fatalf("got %d link spans %#v, want 2", len(links), links)
	}
	if links[0].URL != "https://go.dev/blog" || links[1].URL != "https://tip.golang.org" {
		t.Errorf("link URLs = %q, %q; want the two citation URLs", links[0].URL, links[1].URL)
	}
}

// TestDocCacheReusesStableRows keeps a stable message's Document pointer
// across emissions (link state must survive), re-parses a row whose content
// changed (a streaming delta), and drops rows that left the history.
func TestDocCacheReusesStableRows(t *testing.T) {
	cache := newDocCache()
	history := []Message{
		{Role: RoleUser, Content: "**question**"},
		{Role: RoleAssistant, Content: "answer so"},
	}
	first := cache.Rows(history)
	if first[0].Doc == nil || first[1].Doc == nil {
		t.Fatal("user/assistant rows got no Document")
	}

	// A streaming delta grows the assistant row; the user row is untouched.
	history[1].Content = "answer so far"
	second := cache.Rows(history)
	if second[0].Doc != first[0].Doc {
		t.Error("stable user row got a new Document; link state would reset")
	}
	if second[1].Doc == first[1].Doc {
		t.Error("grown assistant row kept its old Document; body would render stale")
	}
	if n := len(cache.docs); n != 2 {
		t.Errorf("cache holds %d documents, want 2 (stale delta key dropped)", n)
	}

	// Status rows render as plain labels.
	rows := cache.Rows([]Message{{Role: RoleStatus, Content: "Searching…"}})
	if rows[0].Doc != nil {
		t.Error("status row got a Document; want plain label")
	}
	if n := len(cache.docs); n != 0 {
		t.Errorf("cache holds %d documents after the switch, want 0", n)
	}
}
