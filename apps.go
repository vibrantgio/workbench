package main

import "golang.org/x/exp/shiny/materialdesign/icons"

// App is one launchable workbench example. Name doubles as the status key in
// the Model; Dir is the app's directory beside this command, and the last
// element of its module path — the two are the same word because the app's
// module is the directory (see launch.go).
type App struct {
	Name  string
	Dir   string
	Blurb string
	Icon  []byte // IconVG data (see llms.txt §Icons)
}

// Module is the app's module path: the repository's path with the app's
// directory appended, which is what Go fetches a release of.
func (a App) Module() string { return rootModule + "/" + a.Dir }

// rootModule is this command's own module path — the repository root, so that
// the path names a program that can be fetched and run.
const rootModule = "github.com/vibrantgio/workbench"

// Apps is the launch catalogue, in the README's reading order (todos first —
// "start here"). This command is deliberately absent from its own catalogue:
// it is the window the cards are in, not a card. Every entry must name a
// directory that exists beside it, and every such app must have an entry.
var Apps = []App{
	{
		Name:  "Todos",
		Dir:   "todos",
		Blurb: "The minimal canonical MVU app: pure reducers, components widgets, live light/dark theming.",
		// A filled clipboard rather than ActionDone's bare check: every
		// other card's icon depicts its app's subject, and a check is this
		// set's glyph for a completed thing, not for an app about them.
		Icon: icons.ActionAssignmentTurnedIn,
	},
	{
		Name:  "Icon Browser",
		Dir:   "iconbrowser",
		Blurb: "Searchable catalogue of the 961 bundled Material icons, filtered live per keystroke.",
		Icon:  icons.ImagePalette,
	},
	{
		Name:  "Site Docs",
		Dir:   "sitedocs",
		Blurb: "Three-tab docs app: the application guide with its outline tree, the live component gallery, the theme palette.",
		Icon:  icons.ActionDescription,
	},
	{
		Name:  "Feeds",
		Dir:   "feeds",
		Blurb: "RSS reading list: sortable article table, tabbed detail split pane, modal CRUD, toasts.",
		Icon:  icons.ActionViewList,
	},
	{
		Name:  "Vault View",
		Dir:   "vaultview",
		Blurb: "Obsidian vault reader: file tree, breadcrumbs, outline and backlinks, markdown notes.",
		Icon:  icons.ActionBook,
	},
	{
		Name:  "Themer",
		Dir:   "themer",
		Blurb: "Drop in a picture and the whole system re-draws in its colour: seeds, palette, syntax.",
		// An eyedropper, not the palette next to it: ImageColorLens, the
		// obvious name, draws the same glyph as Icon Browser's ImagePalette.
		Icon: icons.ImageColorize,
	},
	{
		Name:  "MindChat",
		Dir:   "mindchat",
		Blurb: "OpenAI chat client: streaming completions through the MVU loop, split-pane shell, undo.",
		Icon:  icons.CommunicationChat,
	},
	{
		Name:  "Marketing",
		Dir:   "marketing",
		Blurb: "A fictional SimpleApps landing on an outline field: provenance and authenticity tools.",
		Icon:  icons.ActionViewQuilt,
	},
}
