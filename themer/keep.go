package main

import (
	stdcolor "image/color"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/brand"
)

// KeepTheme writes the chosen seed where a kept theme lives, off the render
// goroutine. Like a picture being read it never fails the command: a disk
// that will not take the file comes back as a KeepFailed message, because
// the window is still worth looking at afterwards.
//
// Two things are written and both are names for a derivation, not the
// derivation's output: the seed the palette is generated from, and the base
// the syntax colours are derived from. The generators reproduce themselves
// exactly from what they were given, so the two names are the whole theme —
// recording the ramps or the token inks beside them would freeze derivations
// that are still allowed to improve.
func KeepTheme(path string, seed stdcolor.NRGBA, base, source string) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) { return keepTheme(path, seed, base, source), nil })
}

// keepTheme is KeepTheme's body as a plain function, so the whole path from
// a press to a file is testable without a message loop.
func keepTheme(path string, seed stdcolor.NRGBA, base, source string) mvu.Message {
	if path == "" {
		return KeepFailed{Reason: "nowhere to keep it: this machine has no config directory"}
	}
	if err := brand.SaveTo(path, brand.Brand{Seed: seed, Base: base, Source: source}); err != nil {
		return KeepFailed{Reason: "could not keep " + hexOf(seed) + ": " + err.Error()}
	}
	return SeedKept{Seed: seed, Base: base}
}
