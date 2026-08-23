package main

import (
	"image"
	"image/color"
	"math"
	"sync/atomic"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/vibrantgio/noise"
	"github.com/vibrantgio/theme/tokens"

	"github.com/vibrantgio/seen"
	seencolor "github.com/vibrantgio/seen/color"
	seengio "github.com/vibrantgio/seen/context/gio"
	"github.com/vibrantgio/seen/face"
	"github.com/vibrantgio/seen/layer/nsort"
	"github.com/vibrantgio/seen/quaternion"
	"github.com/vibrantgio/seen/shader"
	"github.com/vibrantgio/seen/shape"
)

// The launcher's tilted triangle field, restated as a wireframe: same
// patch, camera, overfill and grow-on-resize, but every face is stroked
// in one Neutral 500 colour (FocusRing) and none is filled. Amplitude
// is lower than the launcher's 0.15 so vertex noise stays sub-pixel on
// a stroke.

const (
	triangleSizePx = 70.0

	cameraDist = 2200.0
	pushback   = -700.0

	coverMarginX = 1.4
	coverMarginY = 1.5
	growStep     = 256.0

	noiseSpeed     = 5e-4
	noiseAmplitude = 0.08

	strokeWidth = 1
	// strokeMix is how much FocusRing remains after blending toward
	// Background. Seen's stroke path drops alpha (Hex is #RRGGBB), so
	// the mesh can only recede in RGB. 0.35 keeps Neutral 500's hue
	// without letting the wireframe become the subject.
	strokeMix = 0.35
)

// Field owns one seen scene and its animation. All fields except pending
// are touched only on the events thread. SetColors may be called from any
// rx goroutine — it only stores into pending; the tick applies it.
type Field struct {
	ctx     *seengio.Context
	scene   *seen.Scene
	view    layout.Widget
	pending atomic.Pointer[color.NRGBA]

	stroke             color.NRGBA
	patch              seen.Object
	halfNx, halfNy     float64
	coveredW, coveredH float64
}

// NewField builds the animated field for the given window (the same
// app.Window mvu's event loop pumps — seen invalidates it to drive frames).
func NewField(window *app.Window, width, height unit.Dp) *Field {
	f := newField(window, width, height)
	f.start()
	return f
}

func newField(window *app.Window, width, height unit.Dp) *Field {
	f := &Field{
		ctx:    seengio.NewContext(window),
		scene:  seen.NewDefaultScene(),
		stroke: quietStroke(tokens.DefaultLight), // pre-theme placeholder
	}
	f.scene.ShowBackfaces = true
	f.scene.Shader = shader.Flat
	f.fit(float64(width), float64(height))

	view := seengio.Widget(f.ctx, func(w, h unit.Dp) {
		f.scene.FitCenter(0, 0, float64(w), float64(h), cameraDist)
		if float64(w)*coverMarginX > f.coveredW || float64(h)*coverMarginY > f.coveredH {
			f.fit(float64(w), float64(h))
		}
	})

	// Transform isolation: the identity Offset push/pop discards anything
	// the seen widget adds to the op list, so the background can never
	// disturb the layers drawn above it.
	f.view = func(gtx layout.Context) layout.Dimensions {
		defer op.Offset(image.Point{}).Push(gtx.Ops).Pop()
		return view(gtx)
	}
	return f
}

func (f *Field) start() {
	noiser := noise.NewSimplex3D(0)
	f.ctx.Animate().OnBefore(func(t, dt time.Duration) {
		f.applyPending()
		tms := float64(t.Milliseconds())
		faces := f.patch.Faces()
		for i, surf := range faces {
			for j, p := range surf.Points {
				faces[i].Points[j].Z = noiser.Noise((p.X-f.halfNx)/8.0, (p.Y-f.halfNy)/8.0, tms*noiseSpeed) * noiseAmplitude
			}
			faces[i].Dirty = true
		}
	}).Start()
}

// Widget returns the field as a plain background widget.
func (f *Field) Widget() layout.Widget { return f.view }

// SetColors re-keys the one stroke colour to new theme tokens. Safe from
// any goroutine; the animation tick applies it on the events thread.
func (f *Field) SetColors(c tokens.ColorTokens) {
	col := quietStroke(c)
	f.pending.Store(&col)
}

// quietStroke mixes FocusRing toward Background so a full-bleed wireframe
// stays one theme colour but reads as a backdrop.
func quietStroke(c tokens.ColorTokens) color.NRGBA {
	s := c.FocusRing()
	g := c.Background
	return color.NRGBA{
		R: mixU8(g.R, s.R, strokeMix),
		G: mixU8(g.G, s.G, strokeMix),
		B: mixU8(g.B, s.B, strokeMix),
		A: 255,
	}
}

func mixU8(a, b uint8, t float64) uint8 {
	return uint8(math.Round(float64(a)*(1-t) + float64(b)*t))
}

func (f *Field) applyPending() {
	if col := f.pending.Swap(nil); col != nil {
		f.stroke = *col
		f.restroke()
	}
}

// fit (re)builds the patch to cover w×h (plus margin and a growStep of
// slack) at a constant triangle size, then makes it the scene's only
// object with a fresh layer.
func (f *Field) fit(w, h float64) {
	pw := w*coverMarginX + growStep
	ph := h*coverMarginY + growStep
	nx := math.Round(pw / triangleSizePx / shape.ALTITUDE)
	ny := math.Round(ph / triangleSizePx)

	p := shape.Patch(nx, ny)

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

	rot := quaternion.RotX(-0.35)
	bx, by, bz := rot.Transform(cx*triangleSizePx, cy*triangleSizePx, 0)
	p.SetScale(triangleSizePx, triangleSizePx, triangleSizePx)
	p.SetRotation(rot)
	p.SetTranslation(-bx, -by, pushback-bz)

	f.patch, f.halfNx, f.halfNy = p, cx, cy
	f.coveredW, f.coveredH = pw, ph
	f.restroke()

	f.scene.Group.Children = []seen.Node{p}
	f.ctx.SetLayers(nsort.NewLayerForScene(f.scene))
}

func (f *Field) restroke() {
	strokeFaces(f.patch.Faces(), f.stroke)
}

func strokeFaces(faces face.Faces, col color.NRGBA) {
	paint := seenFromNRGBA(col)
	for i := range faces {
		faces[i].FillMaterial = nil
		_ = faces[i].SetStroke(paint)
		faces[i].Dirty = true
	}
	faces.SetStrokeWidth(strokeWidth)
}

func seenFromNRGBA(c color.NRGBA) seencolor.Color {
	return seencolor.Color{
		R: float64(c.R) / 255,
		G: float64(c.G) / 255,
		B: float64(c.B) / 255,
		A: float64(c.A) / 255,
	}
}
