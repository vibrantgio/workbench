// frame.go is the vault window's chrome and its column composition: one
// toolbar row across the top — the rail toggle, the vault's name, and the
// two vault actions — over the folder rail, the note column and the
// backlinks aside, the last two parted by a draggable divider.
//
// The composition is app-local rather than the vocabulary's three-column
// shell for one reason: that shell pins its top slot to a full navbar
// band (ControlHeight plus twice the vertical control padding, 52 dp at
// the comfortable density), and this window's whole point is that its
// chrome is a single tight row. Everything else here — full-height
// columns, a divider that tracks an absolute aside width, the op order
// that makes Tab follow the reading order — is the shell's arrangement,
// kept deliberately recognisable.
//
// Where the row sits is a platform fact, not a taste. Under the
// full-size-content treatment the top strip of the window belongs to the
// native title bar: it is where the window control buttons live, and
// clicks that land in it are the system's — window drag, double-click
// zoom — never the application's. Content drawn there paints but cannot
// be pressed. So the toolbar row, every part of which is pressable, seats
// itself immediately below that strip rather than inside it, and the
// strip carries the window buttons alone. Moving the row up beside the
// buttons needs the buttons themselves moved down to meet it, which is
// the window's business rather than the layout's.

package main

import (
	"image"
	"path"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"

	complayout "github.com/vibrantgio/components/layout"
	"github.com/vibrantgio/mvu"
)

// Frame layout constants. The aside bounds and divider width follow the
// three-column shell's, so the two compositions resize alike.
const (
	frameEdgeDp     = 12
	frameGapDp      = 16
	frameDividerDp  = 6
	frameAsideDp    = 320
	frameMinAsideDp = 160
	frameMaxAsideDp = 640

	toggleBarWDp   = 14
	toggleBarHDp   = 2
	toggleBarGapDp = 3
)

// toolbarHeight is the chrome row's height: one LabelLarge line box with
// the smallest spacing step above and below. It deliberately does not
// take the density's control padding — this is a title row, not a row of
// controls, and every dp it spends is a dp the document does not get.
func toolbarHeight(tok themeTokens) unit.Dp {
	return unit.Dp(tok.typ.LabelLarge.LineHeight + 2*tok.sp.S1)
}

// frameState is the vault frame's per-subscription state: the toolbar's
// clickables and the aside divider's drag. It is touched only on the
// frame goroutine.
type frameState struct {
	toggleClick widget.Clickable
	vaultClick  widget.Clickable
	rescanClick widget.Clickable
	switchClick widget.Clickable

	dividerTag struct{}
	asideW     unit.Dp
	pressX     float32
	startW     unit.Dp
	dragging   bool
}

// vaultFrame composes the vault screen from its three column streams and
// the model: a toolbar row over the columns. The columns arrive as widget
// streams so a theme change re-renders them; the toolbar reads the token
// and model snapshots at frame time.
func vaultFrame(
	loadModel func() Model,
	loadTok func() themeTokens,
	sidebar, aside rx.Observable[layout.Widget],
	main layout.Widget,
) rx.Observable[layout.Widget] {
	columns := rx.CombineLatest2(sidebar, aside)
	return rx.Defer(func() rx.Observable[layout.Widget] {
		st := &frameState{asideW: frameAsideDp}
		return rx.Map(columns, func(next rx.Tuple2[layout.Widget, layout.Widget]) layout.Widget {
			sbW, asW := next.First, next.Second
			return func(gtx layout.Context) layout.Dimensions {
				return st.layout(gtx, loadModel(), loadTok(), sbW, asW, main)
			}
		})
	})
}

// layout draws the toolbar row, then the columns below it, in the order
// they read: rail, note, divider, aside.
func (f *frameState) layout(gtx layout.Context, m Model, tok themeTokens, sb, as, main layout.Widget) layout.Dimensions {
	size := gtx.Constraints.Max
	paint.FillShape(gtx.Ops, tok.col.Surface, clip.Rect{Max: size}.Op())

	barH := gtx.Dp(toolbarHeight(tok))
	if barH > size.Y {
		barH = size.Y
	}
	bgtx := gtx
	bgtx.Constraints = layout.Exact(image.Pt(size.X, barH))
	f.layoutToolbar(bgtx, m, tok)

	rowH := size.Y - barH
	if rowH < 0 {
		rowH = 0
	}

	// The rail sizes its own width; hidden, it takes none and the note
	// column absorbs what it left.
	sbW := 0
	if rowH > 0 && !m.SidebarHidden && sb != nil {
		st := op.Offset(image.Pt(0, barH)).Push(gtx.Ops)
		sgtx := gtx
		sgtx.Constraints = layout.Constraints{Max: image.Pt(size.X, rowH)}
		sbW = sb(sgtx).Size.X
		st.Pop()
		if sbW > size.X {
			sbW = size.X
		}
	}

	f.processDividerDrag(gtx)

	dividerW := gtx.Dp(unit.Dp(frameDividerDp))
	if dividerW < 1 {
		dividerW = 1
	}
	asidePx := gtx.Dp(f.asideW)
	if avail := size.X - sbW - dividerW; asidePx > avail {
		asidePx = avail
	}
	if asidePx < 0 {
		asidePx = 0
	}
	mainW := size.X - sbW - dividerW - asidePx
	if mainW < 0 {
		mainW = 0
	}

	if main != nil && rowH > 0 {
		st := op.Offset(image.Pt(sbW, barH)).Push(gtx.Ops)
		mgtx := gtx
		mgtx.Constraints = layout.Exact(image.Pt(mainW, rowH))
		main(mgtx)
		st.Pop()
	}

	// The divider's hit area is registered in frame-local coordinates —
	// no offset transform pushed — so drag deltas measure against a
	// stable origin even as the divider itself moves.
	dividerRect := image.Rect(sbW+mainW, barH, sbW+mainW+dividerW, barH+rowH)
	paint.FillShape(gtx.Ops, tok.col.Divider, clip.Rect(dividerRect).Op())
	area := clip.Rect(dividerRect).Push(gtx.Ops)
	event.Op(gtx.Ops, &f.dividerTag)
	pointer.CursorColResize.Add(gtx.Ops)
	area.Pop()

	if as != nil && rowH > 0 {
		st := op.Offset(image.Pt(sbW+mainW+dividerW, barH)).Push(gtx.Ops)
		agtx := gtx
		agtx.Constraints = layout.Exact(image.Pt(asidePx, rowH))
		as(agtx)
		st.Pop()
	}

	return layout.Dimensions{Size: size}
}

// processDividerDrag tracks the aside divider. The aside keeps an
// absolute width, so a window resize leaves it alone and the note column
// absorbs the change.
func (f *frameState) processDividerDrag(gtx layout.Context) {
	scale := gtx.Metric.PxPerDp
	if scale <= 0 {
		scale = 1
	}
	for {
		e, ok := gtx.Event(pointer.Filter{
			Target: &f.dividerTag,
			Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel,
		})
		if !ok {
			break
		}
		pe, ok := e.(pointer.Event)
		if !ok {
			continue
		}
		switch pe.Kind {
		case pointer.Press:
			f.pressX = pe.Position.X
			f.startW = f.asideW
			f.dragging = true
		case pointer.Drag:
			if f.dragging {
				// The aside sits trailing of the divider, so dragging
				// right shrinks it.
				f.asideW = clampAside(f.startW - unit.Dp((pe.Position.X-f.pressX)/scale))
			}
		case pointer.Release, pointer.Cancel:
			f.dragging = false
		}
	}
}

func clampAside(w unit.Dp) unit.Dp {
	if w < frameMinAsideDp {
		return frameMinAsideDp
	}
	if w > frameMaxAsideDp {
		return frameMaxAsideDp
	}
	return w
}

// layoutToolbar draws the chrome row on one baseline: the rail toggle
// leads, the vault's name stands beside it as what this window is
// showing, and the two vault actions trail as a group at the far edge.
//
// The vault's name is the row's own affordance rather than a breadcrumb
// segment: it is window state, and as a crumb it promised a parent to
// climb to that a vault does not have. Pressing it still returns the
// folder tree to its root, which is what the crumb did.
func (f *frameState) layoutToolbar(gtx layout.Context, m Model, tok themeTokens) layout.Dimensions {
	edge := complayout.HSpacer(frameEdgeDp)
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(edge),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return f.layoutRailToggle(gtx, m, tok)
		}),
		layout.Rigid(complayout.HSpacer(tok.sp.S3)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return f.layoutVaultName(gtx, m, tok)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 0)}
		}),
		layout.Rigid(toolbarAction(&f.rescanClick, "Rescan", tok, Rescan{})),
		layout.Rigid(complayout.HSpacer(frameGapDp)),
		layout.Rigid(toolbarAction(&f.switchClick, "Switch vault", tok, SwitchVault{})),
		layout.Rigid(edge),
	)
}

// layoutVaultName draws the open vault's folder name, pressable, in the
// row's own weight.
func (f *frameState) layoutVaultName(gtx layout.Context, m Model, tok themeTokens) layout.Dimensions {
	name := vaultName(m)
	if name == "" {
		return layout.Dimensions{}
	}
	if f.vaultClick.Clicked(gtx) {
		mvu.MessageOp{Message: RootTree{}}.Add(gtx.Ops)
	}
	return f.vaultClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.LabelOp(name).Add(gtx.Ops)
		semantic.EnabledOp(true).Add(gtx.Ops)
		pointer.CursorPointer.Add(gtx.Ops)
		return drawLabel(gtx, tok.shaper, name, tok.typ.TitleSmall, tok.col.Text)
	})
}

// vaultName is the open vault's folder name — what the window is
// showing — empty before a vault is open.
func vaultName(m Model) string {
	if m.Vault == "" {
		return ""
	}
	return path.Base(strings.TrimRight(m.Vault, "/"))
}

// layoutRailToggle draws the rail's show/hide control: three bars, drawn
// rather than typeset, so the mark does not depend on a face carrying it.
func (f *frameState) layoutRailToggle(gtx layout.Context, m Model, tok themeTokens) layout.Dimensions {
	if f.toggleClick.Clicked(gtx) {
		mvu.MessageOp{Message: ToggleSidebar{}}.Add(gtx.Ops)
	}
	label := "Hide the folder rail"
	if m.SidebarHidden {
		label = "Show the folder rail"
	}
	return f.toggleClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.LabelOp(label).Add(gtx.Ops)
		semantic.EnabledOp(true).Add(gtx.Ops)
		pointer.CursorPointer.Add(gtx.Ops)
		fg := tok.col.Text
		if m.SidebarHidden {
			fg = tok.col.Ramps.Neutral.Step(600)
		}
		w := gtx.Dp(unit.Dp(toggleBarWDp))
		h := gtx.Dp(unit.Dp(toggleBarHDp))
		if h < 1 {
			h = 1
		}
		gap := gtx.Dp(unit.Dp(toggleBarGapDp))
		// The mark is centred in a row-tall box, so the whole height of
		// the row is pressable rather than the twelve dp of ink alone.
		total := 3*h + 2*gap
		boxH := max(gtx.Constraints.Max.Y, total)
		top := (boxH - total) / 2
		for i := range 3 {
			y := top + i*(h+gap)
			paint.FillShape(gtx.Ops, fg, clip.Rect(image.Rect(0, y, w, y+h)).Op())
		}
		return layout.Dimensions{Size: image.Pt(w, boxH)}
	})
}

// toolbarAction renders one chrome-row affordance: a pressable label that
// emits its message on the frame the press lands.
func toolbarAction(click *widget.Clickable, label string, tok themeTokens, msg mvu.Message) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if click.Clicked(gtx) {
			mvu.MessageOp{Message: msg}.Add(gtx.Ops)
		}
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp(label).Add(gtx.Ops)
			semantic.EnabledOp(true).Add(gtx.Ops)
			pointer.CursorPointer.Add(gtx.Ops)
			// Full text contrast, not the low-contrast neutral step: a
			// bare label at that step reads as a disabled control rather
			// than a live one, which is what a fresh reviewer called it.
			return drawLabel(gtx, tok.shaper, label, tok.typ.LabelLarge, tok.col.Text)
		})
	}
}
