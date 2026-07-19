// docs_content.go holds the copy and code samples the docs pages
// render, one entry per sidebar link. Centralising the strings here
// keeps docs.go structural and lets a content review touch a single
// file. Code samples only show real API — every symbol referenced
// exists in the module it is attributed to.

package main

// docsPageDef binds a route identifier to its page content. docsPages
// is the single source of truth the router and sidebar both consume,
// so a link can never point at a page that does not exist.
type docsPageDef struct {
	ID      string
	Content docsPageContent
}

// docsPages returns every docs page in sidebar order.
func docsPages() []docsPageDef {
	return []docsPageDef{
		{pagePrismGettingStarted, prismGettingStartedContent()},
		{pagePrismTokens, prismTokensContent()},
		{pagePrismPrimitives, prismPrimitivesContent()},
		{pageCadencePatterns, cadencePatternsContent()},
		{pageCadenceShells, cadenceShellsContent()},
		{pageSpectrumWindow, spectrumWindowContent()},
		{pageSpectrumTheme, spectrumThemeContent()},
		{pagePulseMotion, pulseMotionContent()},
		{pagePulseEffects, pulseEffectsContent()},
		{pageMVULoop, mvuLoopContent()},
		{pageMVUWindow, mvuWindowContent()},
	}
}

// ---- Prism ---------------------------------------------------------------

func prismGettingStartedContent() docsPageContent {
	return docsPageContent{
		Layer: "Prism",
		Title: "Getting started",
		Paragraphs: []string{
			"Vibrant Gio is a design system for building native desktop applications in Go with Gio. Five layers stack on each other: Prism (tokens and primitives), Cadence (application patterns), Spectrum (platform glue), Pulse (motion), and MVU (the reactive runtime).",
			"Each layer is its own Go module under github.com/vibrantgio — add only the ones you need. Every visual component consumes the same Prism theme observable, so a light/dark or accent change flows through the whole tree without manual wiring.",
		},
		Codes: []docsCodeSample{
			{
				Caption: "Install the layers you need",
				Lines: []string{
					"go get github.com/vibrantgio/prism@latest",
					"go get github.com/vibrantgio/cadence@latest",
					"go get github.com/vibrantgio/mvu@latest",
				},
			},
			{
				Caption: "Bootstrap a themed window",
				Lines: []string{
					"mvuWin := mvu.NewWindow(app.Title(\"My App\"))",
					"w := specwin.New(mvuWin, specsystem.LiveTheme(5*time.Second))",
					"w.Render(buildLayers).Wait()",
				},
			},
		},
	}
}

func prismTokensContent() docsPageContent {
	return docsPageContent{
		Layer: "Prism",
		Title: "Tokens & theme",
		Paragraphs: []string{
			"prism/tokens holds the semantic scales: ColorTokens pairs every ground with its \"On\" foreground (Surface/OnSurface, Primary/OnPrimary, …) plus TypeScale, SpacingScale, RadiusScale, MotionScale and ElevationScale. DefaultLight and DefaultDark ship ready to use.",
			"prism/theme carries one observable per token category. Components subscribe to exactly the categories they consume, so a theme change re-emits only the widgets it affects.",
		},
		Codes: []docsCodeSample{
			{
				Caption: "Consume a token category",
				Lines: []string{
					"colors := rx.SwitchMap(th, func(t theme.Theme)",
					"  rx.Observable[tokens.ColorTokens] {",
					"    return t.Color",
					"})",
				},
			},
			{
				Caption: "Always pair grounds with their On colour",
				Lines: []string{
					"paint.ColorOp{Color: c.OnSurface} // text on Surface",
					"paint.ColorOp{Color: c.OnPrimary} // text on Primary",
				},
			},
		},
	}
}

func prismPrimitivesContent() docsPageContent {
	return docsPageContent{
		Layer: "Prism",
		Title: "Primitives",
		Paragraphs: []string{
			"Prism's widget packages are the foundation Cadence builds on: button, input, list, scrollbar and icon, plus a11y helpers, layout utilities, and coordination for cross-widget arbitration (which popover closes when another opens).",
			"prism/keyed gives list items stable identity so per-item state (focus, hover, animation) survives reordering — the same mechanism cadence/table uses for its rows.",
		},
		Codes: []docsCodeSample{
			{
				Caption: "Layout utilities",
				Lines: []string{
					"inset := pllayout.Inset(24)",
					"gap := pllayout.VSpacer(12)",
				},
			},
		},
	}
}

// ---- Cadence -------------------------------------------------------------

func cadencePatternsContent() docsPageContent {
	return docsPageContent{
		Layer: "Cadence",
		Title: "Patterns",
		Paragraphs: []string{
			"Cadence is the pattern layer: accordion, alert, breadcrumb, card, feature, hero, modal, navbar, pagination, popover, pricing, shell, sidebar, table, tabs, testimonial, toast and tooltip.",
			"Every pattern is a callable function consuming the Prism theme observable and returning rx.Observable[layout.Widget], with a static Render variant for golden-image tests. Source is intentionally short — copy a pattern into your app and modify it.",
		},
		Codes: []docsCodeSample{
			{
				Caption: "Subscribe to a pattern",
				Lines: []string{
					"heroObs := hero.Hero(th, hero.Props{",
					"  Title:    \"Hello\",",
					"  Subtitle: \"world\",",
					"})",
				},
			},
			{
				Caption: "Accordion with a single-open reducer",
				Lines: []string{
					"accObs := accordion.Accordion(th, accordion.Props{",
					"  Sections: secs, Open: openObs,",
					"  OnToggle: toggle,",
					"})",
				},
			},
		},
	}
}

func cadenceShellsContent() docsPageContent {
	return docsPageContent{
		Layer: "Cadence",
		Title: "Shells",
		Paragraphs: []string{
			"cadence/shell provides the top-level application layouts. Four variants: SidebarHeaderMain, SplitPane (draggable divider on either axis), ThreeColumn (full-width navbar, sidebar, main, optional resizable aside, optional footer) and StackedPage (a pinned navbar over a shell-owned scroll of page sections).",
			"This app eats the dog food: the landing page is a StackedPage whose sections are the Cadence marketing patterns, and the page you are reading renders in a ThreeColumn shell with the accordion sidebar in the leading column.",
		},
		Codes: []docsCodeSample{
			{
				Caption: "StackedPage — marketing shell",
				Lines: []string{
					"shell.Shell(th, shell.Props{",
					"  Layout:   shell.StackedPage,",
					"  Navbar:   nav,",
					"  Sections: []rx.Observable[layout.Widget]{heroObs, featObs},",
					"})",
				},
			},
			{
				Caption: "ThreeColumn — resizable aside",
				Lines: []string{
					"shell.Shell(th, shell.Props{",
					"  Layout: shell.ThreeColumn,",
					"  Sidebar: sb, Main: main, Aside: comments,",
					"  OnAsideResize: saveWidth,",
					"})",
				},
			},
		},
	}
}

// ---- Spectrum ------------------------------------------------------------

func spectrumWindowContent() docsPageContent {
	return docsPageContent{
		Layer: "Spectrum",
		Title: "Window & system",
		Paragraphs: []string{
			"spectrum/window wraps an MVU window so every rendered layer receives the live per-window theme; spectrum/system reads the OS appearance (dark mode, accent colour) and republishes it as that theme observable.",
			"On macOS each appearance read forks a `defaults` process, so LiveTheme polls at a configurable cadence — 5 s keeps idle cost near zero while still reacting to a dark-mode toggle in under a second.",
		},
		Codes: []docsCodeSample{
			{
				Caption: "A live system-driven theme",
				Lines: []string{
					"th := specsystem.LiveTheme(5 * time.Second)",
					"w := specwin.New(mvuWin, th)",
				},
			},
		},
	}
}

func spectrumThemeContent() docsPageContent {
	return docsPageContent{
		Layer: "Spectrum",
		Title: "Live theme",
		Paragraphs: []string{
			"spectrum/transition animates token changes so a dark-mode flip cross-fades instead of snapping: ColorTokensTween interpolates a full ColorTokens set over a fixed frame budget.",
			"spectrum/preferences persists the user's explicit appearance choice across launches, stored under the OS config directory for the app.",
		},
		Codes: []docsCodeSample{
			{
				Caption: "Tween between token sets",
				Lines: []string{
					"tw := transition.ColorTokensTween(fromTokens, toTokens, 12)",
				},
			},
			{
				Caption: "Where preferences live",
				Lines: []string{
					"path, err := preferences.Path(\"myapp\")",
				},
			},
		},
	}
}

// ---- Pulse ---------------------------------------------------------------

func pulseMotionContent() docsPageContent {
	return docsPageContent{
		Layer: "Pulse",
		Title: "Motion",
		Paragraphs: []string{
			"pulse/tween interpolates values over a fixed frame budget and pulse/spring integrates critically-damped physics for interruptible motion; pulse/motion composes them into enter/exit choreography for appearing and disappearing widgets.",
			"pulse/conductor is the shared clock: concurrently running animations stay phase-coherent, and frame invalidation stops the moment everything has settled.",
		},
		Codes: []docsCodeSample{
			{
				Caption: "A spring toward a target",
				Lines: []string{
					"s := spring.New(0, 1, spring.Options{})",
				},
			},
		},
	}
}

func pulseEffectsContent() docsPageContent {
	return docsPageContent{
		Layer: "Pulse",
		Title: "Effects",
		Paragraphs: []string{
			"pulse/glow paints vibrancy halos behind accented surfaces and pulse/depth renders soft elevation shadows driven by the prism ElevationLevel token, so visual depth stays consistent with the theme.",
			"pulse/springbutton wraps any clickable with spring-loaded press feedback — the smallest useful composition of the motion primitives.",
		},
		Codes: []docsCodeSample{
			{
				Caption: "Elevation shadow behind a surface",
				Lines: []string{
					"depth.Shadow(gtx, bounds, tokens.Level2)",
				},
			},
			{
				Caption: "Accent halo",
				Lines: []string{
					"glow.Halo(gtx, bounds, glow.Options{})",
				},
			},
		},
	}
}

// ---- MVU -----------------------------------------------------------------

func mvuLoopContent() docsPageContent {
	return docsPageContent{
		Layer: "MVU",
		Title: "The loop",
		Paragraphs: []string{
			"mvu is the Model-View-Update runtime: your application is a model, messages describe what happened, and a pure Update reduces each message into the next model plus an optional Command for async work.",
			"mvu.Loop folds the window's message stream over Update and emits every model; MessageOp lets any widget emit a message from layout code, delivered in the same frame as the click that produced it.",
		},
		Codes: []docsCodeSample{
			{
				Caption: "Run the loop",
				Lines: []string{
					"init := func() (Model, mvu.Command) {",
					"  return initialModel(), mvu.DoNothing()",
					"}",
					"models, runner := mvu.Loop(win.Messages(), init, Update)",
				},
			},
			{
				Caption: "Emit a message from layout code",
				Lines: []string{
					"mvu.MessageOp{Message: SetRoute{Page: pageAbout}}.Add(gtx.Ops)",
				},
			},
		},
	}
}

func mvuWindowContent() docsPageContent {
	return docsPageContent{
		Layer: "MVU",
		Title: "Reactive window",
		Paragraphs: []string{
			"mvu.NewWindow owns the Gio event loop and exposes the message stream; Render composes one or more rx.Observable[layout.Widget] layers, and every layer emission invalidates the window — a theme change or model update repaints without manual wiring.",
			"Layers stack back to front. This app renders a backdrop layer and a shell layer; apps with overlays (modals, toasts, undo bars) append more.",
		},
		Codes: []docsCodeSample{
			{
				Caption: "Compose window layers",
				Lines: []string{
					"w.Render(func(th rx.Observable[theme.Theme])",
					"  []rx.Observable[layout.Widget] {",
					"    return []rx.Observable[layout.Widget]{",
					"      backdropLayer(th), shellLayer(th),",
					"    }",
					"})",
				},
			},
		},
	}
}
