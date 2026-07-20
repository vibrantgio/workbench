package main

import (
	"embed"
	"io/fs"

	"github.com/vibrantgio/markdown/svgimage"
)

//go:embed assets
var assetsFS embed.FS

// mdImages serves the bundled vector icons message bodies reference by
// name — e.g. ![OpenAI](openai.svg) — as crisp SVG widgets. Only embedded
// assets resolve; remote image URLs keep falling back to their alt text,
// so message bodies still trigger no network I/O.
var mdImages = svgimage.New(mustSub(assetsFS, "assets"))

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
