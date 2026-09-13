package main

import (
	stdcolor "image/color"
	"time"

	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/brand"
)

// KeepTheme writes the theme colour where a kept theme lives, off the render
// goroutine. Like a picture being read it never fails the command: a disk
// that will not take the file comes back as a KeepFailed message, because the
// window is still worth looking at afterwards.
//
// What is written is names, not the colours they stand for: the theme colour
// or the standing instruction to follow the system, the base code is coloured
// from under each appearance, and the typeface a fence wears. A base is a
// registry entry read exactly as it was written, so the name is the whole
// choice — recording the colours beside it would freeze a palette that is
// still entitled to improve.
//
// Both bases go, not the one the window happens to be showing. The pair is
// the choice; keeping half of it would leave the other appearance on a
// default nobody picked the moment somebody flipped the scheme.
//
// Nothing else in that file is disturbed. The file is one file for every
// application that adopts a brand, and it holds keys this window does not
// set; they are read back and written out again beside these, so a choice
// made elsewhere survives a choice made here.
//
// follows names no colour: the file records that the theme colour follows the
// system, and every application that adopts the brand takes what the platform
// reports instead of anything on disk. The colour and the source go unwritten
// with it, because neither is true of a colour the platform is still free to
// change.
func KeepTheme(path string, seed stdcolor.NRGBA, follows bool, bases highlight.BasePair, mono, source string) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		return keepTheme(path, seed, follows, bases, mono, source), nil
	})
}

// keepTheme is KeepTheme's body as a plain function, so the whole path from a
// press to a file is testable without a message loop.
func keepTheme(path string, seed stdcolor.NRGBA, follows bool, bases highlight.BasePair, mono, source string) mvu.Message {
	if path == "" {
		return KeepFailed{Reason: "nowhere to keep it: this machine has no config directory"}
	}
	kept := brand.KeptFrom(path)
	kept.Seed, kept.FollowSystem, kept.Source = seed, follows, source
	kept.Base = brand.BasePair{Light: bases.Light, Dark: bases.Dark}
	kept.Mono = mono
	// A keep is a new writing of the file, so the timestamp is this one's
	// rather than the one the file came with.
	kept.Saved = time.Time{}
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
