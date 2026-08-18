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
// Only the seed is written. The palette on screen is generated from it, and
// the generator reproduces itself exactly from the colour it pinned, so the
// colour is the whole theme — recording the ramps beside it would freeze a
// derivation that is still allowed to improve.
func KeepTheme(path string, seed stdcolor.NRGBA, source string) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) { return keepTheme(path, seed, source), nil })
}

// keepTheme is KeepTheme's body as a plain function, so the whole path from
// a press to a file is testable without a message loop.
func keepTheme(path string, seed stdcolor.NRGBA, source string) mvu.Message {
	if path == "" {
		return KeepFailed{Reason: "nowhere to keep it: this machine has no config directory"}
	}
	if err := brand.SaveTo(path, brand.Brand{Seed: seed, Source: source}); err != nil {
		return KeepFailed{Reason: "could not keep " + hexOf(seed) + ": " + err.Error()}
	}
	return SeedKept{Seed: seed}
}
