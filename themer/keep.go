package main

import (
	stdcolor "image/color"

	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/brand"
)

// KeepTheme writes the chosen seed where a kept theme lives, off the render
// goroutine. Like a picture being read it never fails the command: a disk
// that will not take the file comes back as a KeepFailed message, because
// the window is still worth looking at afterwards.
//
// What is written is names for a derivation, not the derivation's output: the
// seed the palette is generated from, and the base the syntax colours are
// derived from under each appearance. The generators reproduce themselves
// exactly from what they were given, so the names are the whole theme —
// recording the ramps or the token inks beside them would freeze derivations
// that are still allowed to improve.
//
// Both bases go, not the one the window happens to be showing. The pair is the
// choice; keeping half of it would leave the other appearance on a default
// nobody picked the moment somebody flipped the scheme.
func KeepTheme(path string, seed stdcolor.NRGBA, bases highlight.BasePair, source string) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) { return keepTheme(path, seed, bases, source), nil })
}

// keepTheme is KeepTheme's body as a plain function, so the whole path from
// a press to a file is testable without a message loop.
func keepTheme(path string, seed stdcolor.NRGBA, bases highlight.BasePair, source string) mvu.Message {
	if path == "" {
		return KeepFailed{Reason: "nowhere to keep it: this machine has no config directory"}
	}
	kept := brand.Brand{
		Seed:   seed,
		Base:   brand.BasePair{Light: bases.Light, Dark: bases.Dark},
		Source: source,
	}
	if err := brand.SaveTo(path, kept); err != nil {
		return KeepFailed{Reason: "could not keep " + hexOf(seed) + ": " + err.Error()}
	}
	return SeedKept{Seed: seed, Bases: bases}
}
