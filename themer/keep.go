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
// What is written is names, not the colours they stand for: the seed the
// palette is generated from, the base the code is drawn in under each
// appearance, and the typeface fenced code wears. The palette generator
// reproduces itself exactly from the seed and a base is a registry entry
// read as its author left it, so the names are the whole theme — recording
// the ramps or the token colours beside them would freeze a derivation that
// is still allowed to improve.
//
// Both bases go, not the one the window happens to be showing. The pair is the
// choice; keeping half of it would leave the other appearance on a default
// nobody picked the moment somebody flipped the scheme.
//
// follows is the one theme here that names no colour: the file records that
// the theme colour follows the system, and every application that adopts the
// brand derives from what the platform reports instead of from anything on
// disk. The seed and the source go unwritten with it, because neither is
// true of a colour the platform is still free to change.
func KeepTheme(path string, seed stdcolor.NRGBA, follows bool, bases highlight.BasePair, mono, source string) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) { return keepTheme(path, seed, follows, bases, mono, source), nil })
}

// keepTheme is KeepTheme's body as a plain function, so the whole path from
// a press to a file is testable without a message loop.
func keepTheme(path string, seed stdcolor.NRGBA, follows bool, bases highlight.BasePair, mono, source string) mvu.Message {
	if path == "" {
		return KeepFailed{Reason: "nowhere to keep it: this machine has no config directory"}
	}
	kept := brand.Brand{
		Seed:         seed,
		FollowSystem: follows,
		Base:         brand.BasePair{Light: bases.Light, Dark: bases.Dark},
		Mono:         mono,
		Source:       source,
	}
	if err := brand.SaveTo(path, kept); err != nil {
		return KeepFailed{Reason: "could not keep " + keptName(seed, follows) + ": " + err.Error()}
	}
	return SeedKept{Seed: seed, Follows: follows, Bases: bases, Mono: mono}
}

// keptName is what a failure calls the theme it could not write: the colour,
// where there is one, and what was asked for instead where there is not.
func keptName(seed stdcolor.NRGBA, follows bool) string {
	if follows {
		return "the system's colour"
	}
	return hexOf(seed)
}
