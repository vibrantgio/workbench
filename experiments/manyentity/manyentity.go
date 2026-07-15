// Package manyentity benchmarks three rendering strategies for a 200-node
// force-directed graph to answer DESIGN §"Architectural Limits" concern #2:
// can an op-cache pattern sustain 60 FPS, or is a dedicated scene primitive needed?
//
// Strategy A (Naive):    per-node clip/paint calls — models FRP "every widget re-renders".
// Strategy B (Cached):   per-node op.Record cache — re-records only when position changes.
// Strategy C (Scene):    single batched path for all nodes — scene primitive approach.
package manyentity

import (
	"image"
	"math"
	"math/rand"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"github.com/vibrantgio/traer"
	"image/color"
)

const nodeRadius = 4

var (
	nodeColor = color.NRGBA{R: 80, G: 130, B: 230, A: 255}
	edgeColor = color.NRGBA{R: 150, G: 150, B: 200, A: 100}
)

// Graph is a force-directed graph backed by traer.ParticleSystem.
type Graph struct {
	*traer.ParticleSystem
}

// NewGraph creates a random force-directed graph with n nodes.
func NewGraph(n int) *Graph {
	ps := traer.NewParticleSystem(0.0, 0.3)
	root := ps.NewDefaultParticle()
	root.Fixed = true
	g := &Graph{ps}
	for i := 1; i < n; i++ {
		g.addNode()
	}
	return g
}

func (g *Graph) addNode() {
	p := g.NewDefaultParticle()
	n := len(g.Particles)
	if n <= 1 {
		return
	}
	q := g.Particles[rand.Intn(n-1)]
	for _, r := range g.Particles[:n-1] {
		if r != p {
			g.NewAttraction(p, r, -1000, 20)
		}
	}
	g.NewSpring(p, q, 0.2, 0.2, 20)
	p.Position = traer.Vec3{
		X: q.Position.X + 2.0*rand.Float64() - 1.0,
		Y: q.Position.Y + 2.0*rand.Float64() - 1.0,
	}
}

type projFn func(traer.Vec3) f32.Point

func (g *Graph) project(size image.Point) projFn {
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64
	for _, p := range g.Particles {
		if p.Position.X < minX {
			minX = p.Position.X
		}
		if p.Position.X > maxX {
			maxX = p.Position.X
		}
		if p.Position.Y < minY {
			minY = p.Position.Y
		}
		if p.Position.Y > maxY {
			maxY = p.Position.Y
		}
	}
	cx := (minX + maxX) / 2
	cy := (minY + maxY) / 2
	ex := maxX - minX
	ey := maxY - minY
	if ex < 1 {
		ex = 1
	}
	if ey < 1 {
		ey = 1
	}
	scale := math.Min(float64(size.X-40)/ex, float64(size.Y-40)/ey)
	scx := float64(size.X) / 2
	scy := float64(size.Y) / 2
	return func(v traer.Vec3) f32.Point {
		return f32.Pt(
			float32((v.X-cx)*scale+scx),
			float32((v.Y-cy)*scale+scy),
		)
	}
}

// drawEdges adds all spring edges to ops as a single open path.
func drawEdges(ops *op.Ops, g *Graph, proj projFn) {
	if len(g.Springs) == 0 {
		return
	}
	var pen f32.Point
	path := clip.Path{}
	path.Begin(ops)
	for _, s := range g.Springs {
		a := proj(s.A.Position)
		b := proj(s.B.Position)
		path.Move(a.Sub(pen))
		pen = a
		path.Line(b.Sub(pen))
		pen = b
	}
	paint.FillShape(ops, edgeColor, clip.Outline{Path: path.End()}.Op())
}

// drawNode draws one node square into ops at the given screen point.
func drawNode(ops *op.Ops, pt f32.Point) {
	r := image.Rect(
		int(pt.X)-nodeRadius, int(pt.Y)-nodeRadius,
		int(pt.X)+nodeRadius, int(pt.Y)+nodeRadius,
	)
	st := clip.Rect(r).Push(ops)
	paint.Fill(ops, nodeColor)
	st.Pop()
}

// RenderNaive draws each node with its own independent clip/paint call sequence.
// Models the FRP "200 separate widget closures, each re-rendering from scratch" scenario.
func RenderNaive(ops *op.Ops, g *Graph, size image.Point) {
	proj := g.project(size)
	drawEdges(ops, g, proj)
	for _, p := range g.Particles {
		drawNode(ops, proj(p.Position))
	}
}

// NodeCache holds per-node op.Record state for strategy B.
type NodeCache struct {
	nops  []*op.Ops
	calls []op.CallOp
	prev  []f32.Point
	valid []bool
}

// NewNodeCache allocates per-node recording buffers for n nodes.
func NewNodeCache(n int) *NodeCache {
	c := &NodeCache{
		nops:  make([]*op.Ops, n),
		calls: make([]op.CallOp, n),
		prev:  make([]f32.Point, n),
		valid: make([]bool, n),
	}
	for i := range c.nops {
		c.nops[i] = new(op.Ops)
	}
	return c
}

// cacheThreshold is the minimum screen-pixel displacement that invalidates a cached op.
const cacheThreshold = float32(0.5)

// Render draws nodes using per-node op caching.
// A node is re-recorded only when its projected position changes by more than cacheThreshold px.
func (c *NodeCache) Render(ops *op.Ops, g *Graph, size image.Point) {
	proj := g.project(size)
	drawEdges(ops, g, proj)
	for i, p := range g.Particles {
		if i >= len(c.nops) {
			break
		}
		pt := proj(p.Position)
		dx := pt.X - c.prev[i].X
		dy := pt.Y - c.prev[i].Y
		if !c.valid[i] || dx*dx+dy*dy > cacheThreshold*cacheThreshold {
			nops := c.nops[i]
			nops.Reset()
			macro := op.Record(nops)
			drawNode(nops, pt)
			c.calls[i] = macro.Stop()
			c.prev[i] = pt
			c.valid[i] = true
		}
		c.calls[i].Add(ops)
	}
}

// RenderScene draws all nodes as a single batched path — the scene primitive approach.
func RenderScene(ops *op.Ops, g *Graph, size image.Point) {
	proj := g.project(size)
	drawEdges(ops, g, proj)

	var pen f32.Point
	path := clip.Path{}
	path.Begin(ops)
	s := float32(nodeRadius)
	for _, p := range g.Particles {
		pt := proj(p.Position)
		tl := f32.Pt(pt.X-s, pt.Y-s)
		tr := f32.Pt(pt.X+s, pt.Y-s)
		br := f32.Pt(pt.X+s, pt.Y+s)
		bl := f32.Pt(pt.X-s, pt.Y+s)
		path.Move(tl.Sub(pen))
		pen = tl
		path.Line(tr.Sub(pen))
		pen = tr
		path.Line(br.Sub(pen))
		pen = br
		path.Line(bl.Sub(pen))
		pen = bl
		path.Line(tl.Sub(pen))
		pen = tl
		path.Close()
	}
	paint.FillShape(ops, nodeColor, clip.Outline{Path: path.End()}.Op())
}
