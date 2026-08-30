// Command vaultview is a desktop viewer for folders of markdown notes
// written in the Obsidian style: YAML-ish frontmatter at the top,
// [[wikilinks]] between notes, ^block-ids stamped at the end of a
// paragraph. It opens a vault, renders a note, and follows the links.
// The one write it makes is a GFM task marker: clicking a checkbox
// flips `[ ]` to `[x]` or `[x]`/`[X]` to `[ ]` in the file, one
// character, before the click's handler returns. Nothing else is
// writable.
//
// # Choosing a vault
//
// A vault path given on the command line wins. Without one the last
// vault opened is used, and on a first run — or when the remembered path
// has stopped being a directory — an in-app folder browser asks. Every
// successful open is remembered, so the next launch needs no argument.
// The remembered path is one line of text in vaultview/vault under
// $XDG_CONFIG_HOME, or under ~/.config when that variable is unset.
//
// # What a link means
//
// This is the contract the viewer holds itself to when a wikilink is
// clicked: the behaviour Obsidian describes, restated as the rules this
// program implements over the index the vault scan produced.
//
// A link body is written between double brackets and has three parts,
// all optional after the first: a file part, then a heading path, then a
// block id.
//
//	[[Note]]              [[Note|what to show instead]]
//	[[Folder/Note]]       [[Note#Section#Subsection]]
//	[[Note#^block-id]]    [[#Section in this same note]]
//	![[Note]]             an embed; here it navigates like any link
//
// Everything after the first vertical bar is display text and takes no
// part in resolution. The file part may carry folders and conventionally
// omits the .md extension. A block id is letters, digits and hyphens.
//
// The vault root is the folder that was opened. When the viewer is
// pointed at a single file instead, the root is the nearest folder above
// it holding a .obsidian directory; failing that the top of the
// surrounding checkout; failing that the file's own folder.
//
// A file part resolves in three steps, and stops at the first that
// answers:
//
//  1. as written, relative to the folder of the note the link is in;
//  2. as written, against the vault root;
//  3. as a file name — no folders — anywhere below the root.
//
// Steps 1 and 2 also try the name with .md appended, since links name
// notes without the extension. Step 3 is where a vault can be
// ambiguous: when two or more notes share the name, the viewer refuses
// rather than guessing, and asks which one was meant. Comparisons are
// exact; the heading match below is the one exception.
//
// A heading path descends the target's headings by title, one segment at
// a time, matching without regard to case. Each matched heading narrows
// the search to its own section, so a subsection is found under its
// parent and not elsewhere in the note. A title that is absent refuses;
// a title that appears twice in the same section refuses too — the same
// refusal a duplicated file name earns.
//
// A block id matches a stamped id in the target note. Ids are unique
// within their file, so a block id either exists or it does not; there
// is no ambiguous case.
//
// A link with no file part resolves inside the note it was written in.
//
// Code is never a link. A wikilink inside a fenced code block or inline
// code span does not resolve, does not count as a backlink, and is not
// drawn as a link: it is a code sample, not navigation.
//
// A link the viewer cannot follow stays drawn as a link and says why it
// refused when it is clicked, rather than silently reading as prose.
//
// Where a link lands inside the target is computed from the parsed note,
// never from the scan: the scan only answers whether an anchor exists,
// so the scroll position and the rendered text cannot disagree.
//
// # Freshness
//
// The viewer watches no files. Instead, following a link checks the
// target's timestamp and re-reads the note when it changed on disk, so
// an edit made elsewhere shows up the moment it is opened; moving back
// and forward through the history keeps what was already on screen,
// scroll position included. Notes appearing, disappearing or being
// renamed change the vault's shape rather than one note's content, and
// the Rescan affordance at the foot of the folder rail re-walks the
// vault for those.
//
// # What it does not do
//
// There is no editor, no graph view, and no full-text search. The find
// field above the tree filters by note name over the index the scan
// already built; it reads no file and searches no prose. An embed is
// drawn as an ordinary link rather than pulling the other note inline.
package main
