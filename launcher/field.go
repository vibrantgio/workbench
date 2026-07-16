package main

import (
	"image"
	"math"
	"sync/atomic"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/noise"
	"github.com/vibrantgio/prism/tokens"

	"github.com/vibrantgio/seen"
	seencolor "github.com/vibrantgio/seen/color"
	seengio "github.com/vibrantgio/seen/context/gio"
	"github.com/vibrantgio/seen/face"
	"github.com/vibrantgio/seen/layer/bsort"
	"github.com/vibrantgio/seen/quaternion"
	"github.com/vibrantgio/seen/shape"
	"github.com/vibrantgio/seen/viewport"
)

// The animated triangle field: a seen 3D triangular patch, tilted back,
// displaced per-vertex by time-evolving simplex noise, and coloured from a
// fixed spatial hue field keyed to the live prism theme. Ported from the
// seenbgdemo vertical slice (its centred variant, whose symmetric overfill
// margins cover every window edge — the top-left variant left the top edge
// bare under noise displacement).

// Field geometry and motion, tuned by eye in seenbgdemo — see that repo's
// patch.go for the full derivation of every constant.
const (
	triangleSizePx = 70.0   // world size of one triangle; on-screen size is constant at every window size
	cameraDist     = 2200.0 // fixed perspective distance, decoupled from window height
	pushback       = -700.0 // world Z of the patch centre; past the tilted patch's half-depth (no near-plane clipping)

	// Over-fill margins: the tilted patch must extend past every screen edge,
	// including under the ±noiseAmplitude vertex drift. Verified in seenbgdemo
	// from 1000×680 up to a 3008×1692 (6K) display.
	coverMarginX = 1.4
	coverMarginY = 1.5
	growStep     = 256.0 // dp of slack per regrow, so a resize drag rebuilds in chunks

	noiseSpeed     = 5e-4 // time scale of the vertex drift
	noiseAmplitude = 0.15 // Z displacement scale
)

// fieldPalette is the resolved colour recipe for the triangle fills: a smooth
// spatial hue field centred on hue, swinging ±spread, at fixed sat/lit.
type fieldPalette struct {
	hue    float64
	spread float64
	sat    float64
	lit    float64
}

// paletteFrom derives the field's palette from the prism colour tokens: the
// hue family follows the theme's Primary, and the value range keeps the field
// a quiet backdrop — deep tones on a dark background, pastel on a light one —
// so the hero text and cards floating on it stay readable.
func paletteFrom(c tokens.ColorTokens) fieldPalette {
	hue, _, _ := rgbToHSL(c.Primary)
	_, _, backLit := rgbToHSL(c.Background)
	if backLit < 0.5 {
		return fieldPalette{hue: hue, spread: 0.10, sat: 0.38, lit: 0.30}
	}
	return fieldPalette{hue: hue, spread: 0.10, sat: 0.45, lit: 0.80}
}

// Field owns one seen scene and its animation. All fields except pending are
// touched only on the events thread (construction happens before the first
// frame; after that, mutation happens inside the animation tick and the
// widget's resize callback). SetColors may be called from any rx goroutine —
// it only stores into pending; the tick applies it.
type Field struct {
	ctx     *seengio.Context
	scene   *seen.Scene
	view    layout.Widget
	pending atomic.Pointer[fieldPalette]

	pal                fieldPalette
	patch              seen.Object
	halfNx, halfNy     float64
	coveredW, coveredH float64
	hueField           *noise.Simplex3D
}

// NewField builds the animated field for the given window (the same
// app.Window mvu's event loop pumps — seen invalidates it to drive frames).
func NewField(window *app.Window, width, height unit.Dp) *Field {
	f := &Field{
		ctx:      seengio.NewContext(window),
		scene:    seen.NewDefaultScene(),
		pal:      fieldPalette{hue: 0.58, spread: 0.10, sat: 0.40, lit: 0.40}, // pre-theme placeholder
		hueField: noise.NewSimplex3D(1),
	}
	f.scene.ShowBackfaces = true
	f.fit(float64(width), float64(height))

	// Animate: apply any newly-arrived palette, then displace each vertex with
	// time-evolving 3D noise, indexed from the patch centre so regrowth never
	// makes the ripple pattern jump.
	noiser := noise.NewSimplex3D(0)
	f.ctx.Animate().OnBefore(func(t, dt time.Duration) {
		if pal := f.pending.Swap(nil); pal != nil {
			f.pal = *pal
			f.recolor()
		}
		tms := float64(t.Milliseconds())
		faces := f.patch.Faces()
		for i, surf := range faces {
			for j, p := range surf.Points {
				faces[i].Points[j].Z = noiser.Noise((p.X-f.halfNx)/8.0, (p.Y-f.halfNy)/8.0, tms*noiseSpeed) * noiseAmplitude
			}
			// Direct point mutation must mark the face dirty or the coordinate
			// cache ignores the change.
			faces[i].Dirty = true
		}
	}).Start()

	view := seengio.Widget(f.ctx, func(w, h unit.Dp) {
		f.scene.Viewport = viewport.Center(0, 0, float64(w), float64(h), cameraDist)
		// Grow the field when the viewport outgrows what the patch covers.
		if float64(w)*coverMarginX > f.coveredW || float64(h)*coverMarginY > f.coveredH {
			f.fit(float64(w), float64(h))
		}
	})

	// Transform isolation: the identity Offset push/pop discards anything the
	// seen widget adds to the op list, so the background can never disturb the
	// layers drawn above it (see seenbgdemo patch.go for the full rationale).
	f.view = func(gtx layout.Context) layout.Dimensions {
		defer op.Offset(image.Point{}).Push(gtx.Ops).Pop()
		return view(gtx)
	}
	return f
}

// Widget returns the field as a plain background widget.
func (f *Field) Widget() layout.Widget { return f.view }

// SetColors re-keys the palette to new theme tokens. Safe from any goroutine;
// the animation tick applies it on the events thread.
func (f *Field) SetColors(c tokens.ColorTokens) {
	pal := paletteFrom(c)
	f.pending.Store(&pal)
}

// fit (re)builds the patch to cover w×h (plus margin and a growStep of slack)
// at a constant triangle size, then makes it the scene's only object with a
// fresh layer. Called once up front and again whenever the viewport grows.
func (f *Field) fit(w, h float64) {
	pw := w*coverMarginX + growStep
	ph := h*coverMarginY + growStep
	nx := math.Round(pw / triangleSizePx / shape.ALTITUDE)
	ny := math.Round(ph / triangleSizePx)

	p := shape.Patch(nx, ny)

	// shape.Patch scales X by ALTITUDE and staggers Y, so the patch's centre is
	// NOT (nx/2, ny/2). Measure the actual bounding box centre.
	fs := p.Faces()
	minX, maxX := fs[0].Points[0].X, fs[0].Points[0].X
	minY, maxY := fs[0].Points[0].Y, fs[0].Points[0].Y
	for _, face := range fs {
		for _, pt := range face.Points {
			minX, maxX = math.Min(minX, pt.X), math.Max(maxX, pt.X)
			minY, maxY = math.Min(minY, pt.Y), math.Max(maxY, pt.Y)
		}
	}
	cx, cy := (minX+maxX)/2, (minY+maxY)/2

	// Pin the patch centre at world (0, 0, pushback). The tilt is applied
	// before the translation (T·R·S), so translate by the *rotated* centre and
	// add the fixed pushback in Z: the centre stays at one constant depth
	// however the field regrows, so regrowth never rescales the triangles.
	rot := quaternion.RotX(-0.35)
	bx, by, bz := rot.Transform(cx*triangleSizePx, cy*triangleSizePx, 0)
	p.SetScale(triangleSizePx, triangleSizePx, triangleSizePx)
	p.SetRotation(rot)
	p.SetTranslation(-bx, -by, pushback-bz)

	f.patch, f.halfNx, f.halfNy = p, cx, cy
	f.coveredW, f.coveredH = pw, ph
	f.recolor()

	f.scene.Group.Children = []seen.Node{p}
	// Fresh layer so the previous patch's cached fragments don't linger.
	f.ctx.SetLayers(bsort.NewLayerForScene(f.scene))
}

// recolor fills every face from the fixed hue field evaluated at the face's
// STABLE position — its object-space barycentre minus the patch centre, the
// same basis the Z-noise is indexed by. A triangle keeps its colour however
// the field regrows; only a theme change repaints (deliberately, all at once).
//
// Position-keyed colour (rather than face-iteration order) is load-bearing:
// shape.Patch emits faces column-major, so growing the field in height remaps
// every column's face indices — order-keyed colour repaints the world on
// every regrow (seenbgdemo colors.go documents the failed alternatives).
const colorFreq = 0.10 // spatial frequency; lower = larger colour regions

func (f *Field) recolor() {
	faces := f.patch.Faces()
	colorFaces(faces, f.halfNx, f.halfNy, f.pal, f.hueField)
}

func colorFaces(faces face.Faces, originX, originY float64, pal fieldPalette, hueField *noise.Simplex3D) {
	for i := range faces {
		var bx, by float64
		for _, p := range faces[i].Points {
			bx += p.X
			by += p.Y
		}
		bx, by = bx/3-originX, by/3-originY
		h := pal.hue + pal.spread*hueField.Noise(bx*colorFreq, by*colorFreq, 0)
		h -= math.Floor(h) // wrap into [0,1)
		faces[i].SetFill(seencolor.ColorHSL(h, pal.sat, pal.lit, 1.0))
		faces[i].Dirty = true
	}
}
