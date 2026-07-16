package main

import "golang.org/x/exp/shiny/materialdesign/icons"

// App is one launchable workbench example. Name doubles as the status key in
// the Model; Dir is the app's directory under the workbench root, which is
// also what `go run ./<Dir>/` takes.
type App struct {
	Name  string
	Dir   string
	Blurb string
	Icon  []byte // IconVG data (see llms.txt §Icons)
}

// Apps is the launch catalogue, in the README's reading order (todos first —
// "start here"). The launcher itself is deliberately absent.
var Apps = []App{
	{
		Name:  "Todos",
		Dir:   "todos",
		Blurb: "The minimal canonical MVU app: pure reducers, prism components, live light/dark theming.",
		Icon:  icons.ActionDone,
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
		Blurb: "Documentation & marketing site: hero, pricing, accordion sidebar, breadcrumbs.",
		Icon:  icons.ActionDescription,
	},
	{
		Name:  "Feeds",
		Dir:   "feeds",
		Blurb: "RSS reading list: sortable article table, tabbed detail split pane, modal CRUD, toasts.",
		Icon:  icons.ActionViewList,
	},
	{
		Name:  "Watchlist",
		Dir:   "watchlist",
		Blurb: "Persistent watchlist editor: JSON storage, context menus, bulk delete with confirms.",
		Icon:  icons.ActionVisibility,
	},
}
