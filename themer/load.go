package main

import (
	"image"
	"os"
	"path/filepath"

	// The formats a picture is likely to arrive in. Decoders register
	// themselves, so importing them for the side effect is what makes
	// image.Decode able to answer at all.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	xdraw "golang.org/x/image/draw"

	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/theme/imageseed"
)

// previewMax is the longest edge the on-screen picture is reduced to. It is
// a display budget, not an extraction one: extraction reads the full-size
// image on its own stride, and this is only what gets uploaded to the GPU
// every frame.
const previewMax = 1024

// LoadImage reads, decodes and extracts in one command, off the render
// goroutine. It never fails the command: every way this can go wrong comes
// back as an ImageRejected message, because a stray file dragged onto the
// window must not take the application down with it.
func LoadImage(path string) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) { return loadImage(path), nil })
}

// loadImage is LoadImage's body as a plain function, so the whole path from
// a file to a candidate row is testable without a message loop.
func loadImage(path string) mvu.Message {
	f, err := os.Open(path)
	if err != nil {
		return ImageRejected{Path: path, Reason: "could not read " + filepath.Base(path)}
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return ImageRejected{Path: path, Reason: filepath.Base(path) + " is not an image this reads (PNG, JPEG and GIF are)"}
	}
	candidates := imageseed.Extract(img)
	if len(candidates) == 0 {
		return ImageRejected{Path: path, Reason: filepath.Base(path) + " has no visible colour in it"}
	}
	return ImageLoaded{Path: path, Preview: preview(img), Candidates: candidates}
}

// preview shrinks a decoded picture to something worth uploading every
// frame, keeping its aspect ratio and never enlarging a small one. The
// interpolation is the cheap bilinear kernel: this is a thumbnail beside the
// colours it produced, not a photograph anyone will pixel-peep.
func preview(img image.Image) *image.NRGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil
	}
	if w > previewMax || h > previewMax {
		if w >= h {
			w, h = previewMax, max(1, h*previewMax/b.Dx())
		} else {
			w, h = max(1, w*previewMax/b.Dy()), previewMax
		}
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	xdraw.ApproxBiLinear.Scale(out, out.Bounds(), img, b, xdraw.Src, nil)
	return out
}
