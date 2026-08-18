# Vault View

A read-only desktop viewer for a folder of markdown notes written in the
Obsidian style: frontmatter at the top, `[[wikilinks]]` between notes,
`^block-ids` stamped at the end of a paragraph. It opens a vault, renders
a note, and follows the links. It never writes to a note.

```sh
cd vaultview && go run .            # opens the vault you used last, or asks
cd vaultview && go run . ~/Notes    # opens that vault, and remembers it
```

## Choosing a vault

A vault path on the command line wins. Without one the last vault opened
is used, and on a first run — or when the remembered path has stopped
being a directory — an in-app folder browser asks: a breadcrumb of where
you are, the child folders as rows, each annotated with the `.obsidian`
marker or its note count, and an **Open this vault** action. Arrows move,
Return descends.

Every successful open is remembered, so the next launch needs no
argument. The remembered path is one line of text in `vaultview/vault`
under `$XDG_CONFIG_HOME`, or under `~/.config` when that variable is
unset. Deleting that file is how you forget a vault.

## Reading

The window is a sidebar down the leading edge and two columns beside it.

- **Left — the tree.** Every note in the vault, folders disclosing on
  click, the note you are reading marked. Above it, a find field:
  type and the hierarchy gives way to the notes whose name matches,
  each with its folder as a quiet annotation. The sidebar is a rounded
  pane floating just inside the window's edge — the window's own buttons
  show through its top strip, where the platform puts them and where they
  stay — and its toggle sits at its top-right corner, on their line. Put
  the pane away and the toggle that brings it back appears at the leading
  end of the top row, on that same line, beside the vault's name which is
  on it too; the buttons do not budge.
- **Middle — the note.** Back and forward, then the trail — vault name,
  folders, note title. The frontmatter is lifted out of the prose into a
  collapsible **Properties** panel; what a plain line-split can read
  shows as key and value, anything it cannot is shown raw. Then the note
  itself: headings, lists, tables, quotes, and code blocks with syntax
  highlighting.
- **Right — outline and backlinks.** Two panes, each scrolling in its own
  right. Above, the current note's headings: the one you are reading is
  marked as the note scrolls, and choosing another moves the note to it
  rather than opening it again. Below, every note whose links resolve to
  this one, a row each; click to go there. A note with no headings says
  so and gives the column to its backlinks.

Back and forward walk the notes you have visited, and each note keeps its
own scroll position, so returning to a note returns you to where you were
in it.

## Following links

Click a wikilink and the viewer goes where Obsidian would go. The rules
it holds itself to — where a file part looks, how a heading path
descends, why code is never a link, what happens when a name is
ambiguous — are stated in full in `doc.go`, which is the contract this
program is written against and tested to. In short:

- A link may name a file, a heading path inside it, or a stamped block
  id; text after a `|` is display only.
- The file part is looked for beside the linking note, then at the vault
  root, then as a note name anywhere in the vault.
- Nothing is ever guessed. Two notes with the same name raise a chooser
  asking which one you meant; a heading or block that is not there says
  so and stays where it is.
- A wikilink inside a code fence or an inline code span is a code
  sample, not navigation: it stays plain text and is no one's backlink.
- Web links open in the system browser. An embed — `![[Note]]` — is
  drawn as an ordinary link rather than pulling the other note inline.

## Freshness

Nothing is watched. Instead, following a link checks the note's timestamp
and re-reads it when the file changed, so an edit made in another
application shows up the moment you open the note; moving back and
forward keeps what is already on screen. **Rescan**, at the foot of the
folder rail, re-walks the vault for the changes one note cannot show — notes added,
renamed or removed while the viewer was open — and reports what it found.

## What it deliberately does not do

There is no editing, no graph view, and no full-text search. The find
field filters by note name over the index the scan already built: it
reads no file and searches no prose. These are not missing features; a
different application would be the honest way to have them.

## How it is built

The canonical MVU shape — one `Model`, pure `Update`, a window rendering
layers — over the design system's vocabulary:

| Piece | Where it comes from |
| --- | --- |
| Loop, commands, click-to-message | `mvu`, `mvu/desktop` |
| Live OS theme, tokens, type | `theme` |
| List, text field, layout, goldens | `components` |
| Breadcrumb, modal, toast | `patterns` |
| Parsing, rendering, highlighting | `markdown`, `markdown/highlight` |
| Frontmatter, wikilink spans, block ids | `markdown/obsidian` |

Three things are the app's own, on purpose. The **folder tree** is an
app-local composition over `components/list` — the design system's
sidebar is deliberately flat, and one nesting sidebar is not yet a
pattern. The **window frame** — the sidebar column, and one tight chrome
row over the two columns beside it — is an app-local arrangement rather
than the three-column shell, because that shell pins its top slot to a
full navbar band and this window spends as little height on chrome as it
can. The **resolver** is
vault semantics rather than rendering: pure functions over the scanned
index, table-tested rule by rule.

The vault scan is a fence-aware line scanner — never a full parse — run
off the render goroutine; a note is parsed only when it is opened. Where
a link lands inside a note is computed from the parsed note, never from
the scan, so the scroll position and the rendered text cannot disagree.

No dependency outside the design system's own stack enters here: no YAML
library, no file watcher, no wikilink library.

## Tests

```sh
go test ./...                      # everything, goldens included
go test . -golden.update           # re-record the golden images
```

The goldens store a rendered note and the tree rail in light and dark
under `testdata/golden`. They shape with a deterministic font set, so
they cannot drift with the host's installed fonts. The window's marks —
the sidebar toggle, the disclosures, the two history controls — are not
typeset at all: they come from the design system's icon set, so the
goldens record the same ink the runtime draws.
