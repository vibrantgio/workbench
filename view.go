package main

import (
	"image"
	"image/color"
	"path/filepath"
	"strings"

	"golang.org/x/exp/shiny/materialdesign/colornames"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/ivg"
	"github.com/vibrantgio/ivg/encode"
	"github.com/vibrantgio/ivg/generate"
	raster "github.com/vibrantgio/ivg/raster/gio"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/prism/input"
	prismtheme "github.com/vibrantgio/prism/theme"
	"github.com/vibrantgio/style"

	openai "github.com/sashabaranov/go-openai"
)

var prismTh = rx.Of(prismtheme.Default())

func View() func(Model) layout.Widget {
	shaper := text.NewShaper(text.WithCollection(style.FontFaces()))
	chathist := ChatHistWidget(shaper)
	chatlist := ChatListWidget(shaper)
	return func(model Model) layout.Widget {
		hist := chathist(model.CurrentChat.History)
		list := chatlist(model.ChatList, model.CurrentChat.Name)
		return func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Max
			layout.N.Layout(gtx, hist)
			layout.NW.Layout(gtx, list)
			return layout.Dimensions{Size: size}
		}
	}
}

func ChatHistWidget(shaper *text.Shaper) func(hist []openai.ChatCompletionMessage) layout.Widget {

	list := layout.List{Axis: layout.Vertical, ScrollToEnd: true, Alignment: layout.Start}
	edit, _ := input.TextField(prismTh, input.TextFieldProps{
		Placeholder:   "Send a message",
		Submit:        true,
		SubmitMessage: func(text string) any { return Prompt{Content: text} },
		Shaper:        shaper,
	}).First()

	return func(chat []openai.ChatCompletionMessage) layout.Widget {

		return func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = ClampWidth(gtx, 0, 794)

			layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return list.Layout(gtx, len(chat), func(gtx layout.Context, index int) layout.Dimensions {
						content := chat[index].Content

						style := style.BodyText1

						textMaterial := Material(gtx.Ops, colornames.Grey200)

						label := widget.Label{Alignment: text.Start, MaxLines: style.MaxLines, Truncator: style.Truncator}

						m := op.Record(gtx.Ops)
						dims := layout.UniformInset(12).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							margin := gtx.Dp(50)
							defer op.Offset(image.Pt(margin, 0)).Push(gtx.Ops).Pop()
							gtx.Constraints.Max.X -= margin
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							dims := label.Layout(gtx, shaper, style.Font, style.Size, content, textMaterial)
							dims.Size.X += margin
							return dims
						})
						foreground := m.Stop()

						cs := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
						var bg color.Color
						var widget layout.Widget
						if chat[index].Role == openai.ChatMessageRoleUser {
							bg = colornames.BlueGrey600
							widget = func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: gtx.Constraints.Max} }
						} else {
							bg = colornames.BlueGrey700
							var err error
							if widget, err = raster.Widget(ChatGPT, 40, 40, raster.WithColors(colornames.White)); err != nil {
								panic(err)
							}
						}
						paint.ColorOp{Color: NRGBA(bg)}.Add(gtx.Ops)
						paint.PaintOp{}.Add(gtx.Ops)
						cs.Pop()

						constraints := gtx.Constraints
						iconSize := gtx.Dp(40)
						gtx.Constraints = layout.Exact(image.Pt(iconSize, iconSize))
						widget(gtx)
						gtx.Constraints = constraints

						foreground.Add(gtx.Ops)

						return dims
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(8).Layout(gtx, edit)
				}),
			)

			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
	}
}

func ChatListWidget(shaper *text.Shaper) func(chats ChatList, current string) layout.Widget {
	clickables := map[string]*widget.Clickable{}
	listState := layout.List{Axis: layout.Vertical, ScrollToEnd: false, Alignment: layout.Start}

	return func(chats ChatList, current string) layout.Widget {
		// Ensure every chat has a persistent Clickable for hover/click state.
		for _, name := range chats {
			if _, ok := clickables[name]; !ok {
				clickables[name] = new(widget.Clickable)
			}
		}

		return func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = ClampWidth(gtx, 0, 260)

			m := op.Record(gtx.Ops)
			dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return SidebarHeader(gtx, shaper)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return listState.Layout(gtx, len(chats), func(gtx layout.Context, index int) layout.Dimensions {
						name := chats[index]
						return ChatRowWidget(gtx, shaper, name, name == current, clickables[name])
					})
				}),
			)
			foreground := m.Stop()
			FillRect(gtx, image.Rectangle{Max: dims.Size}, 0, colornames.BlueGrey900)
			foreground.Add(gtx.Ops)
			return dims
		}
	}
}

// SidebarHeader renders the "CONVERSATIONS" heading at the top of the sidebar.
func SidebarHeader(gtx layout.Context, shaper *text.Shaper) layout.Dimensions {
	label := widget.Label{Alignment: text.Start, MaxLines: 1, Truncator: "…"}
	textMaterial := Material(gtx.Ops, colornames.BlueGrey300)

	m := op.Record(gtx.Ops)
	dims := layout.Inset{Top: unit.Dp(14), Bottom: unit.Dp(10), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return label.Layout(gtx, shaper, style.Caption.Font, style.Caption.Size, "CONVERSATIONS", textMaterial)
		},
	)
	foreground := m.Stop()

	FillRect(gtx, image.Rectangle{Max: dims.Size}, 0, colornames.BlueGrey900)
	// Bottom separator line
	sepRect := image.Rectangle{
		Min: image.Pt(gtx.Dp(12), dims.Size.Y-gtx.Dp(1)),
		Max: image.Pt(dims.Size.X-gtx.Dp(12), dims.Size.Y),
	}
	FillRect(gtx, sepRect, 0, colornames.BlueGrey700)
	foreground.Add(gtx.Ops)
	return dims
}

// ChatRowWidget renders a single chat entry in the sidebar with hover and selection states.
func ChatRowWidget(gtx layout.Context, shaper *text.Shaper, name string, selected bool, clickable *widget.Clickable) layout.Dimensions {
	// Drain pending clicks before Layout — Layout's internal update loop
	// consumes click events and discards them, so Clicked must run first.
	for clickable.Clicked(gtx) {
		mvu.MessageOp{Message: SelectChat{Name: name}}.Add(gtx.Ops)
	}

	displayName := strings.TrimSuffix(name, filepath.Ext(name))
	// Title-case the first letter for a cleaner look.
	if len(displayName) > 0 {
		displayName = strings.ToUpper(displayName[:1]) + displayName[1:]
	}

	var bgColor color.Color
	var textColor color.Color
	switch {
	case selected:
		bgColor = colornames.BlueGrey700
		textColor = colornames.Grey100
	case clickable.Hovered():
		bgColor = colornames.BlueGrey800
		textColor = colornames.Grey300
	default:
		bgColor = colornames.BlueGrey900
		textColor = colornames.Grey500
	}

	label := widget.Label{Alignment: text.Start, MaxLines: 1, Truncator: "…"}

	return clickable.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		textMaterial := Material(gtx.Ops, textColor)

		m := op.Record(gtx.Ops)
		dims := layout.Inset{Top: unit.Dp(11), Bottom: unit.Dp(11), Left: unit.Dp(20), Right: unit.Dp(12)}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				return label.Layout(gtx, shaper, style.Subtitle2.Font, style.Subtitle2.Size, displayName, textMaterial)
			},
		)
		foreground := m.Stop()

		FillRect(gtx, image.Rectangle{Max: dims.Size}, 0, bgColor)
		// Left accent bar for the selected item.
		if selected {
			FillRect(gtx, image.Rectangle{Max: image.Pt(gtx.Dp(3), dims.Size.Y)}, 0, colornames.BlueGrey400)
		}
		foreground.Add(gtx.Ops)
		return dims
	})
}

var ChatGPT = func() []byte {
	// generate ivg data bytes on the fly for the logo.
	enc := &encode.Encoder{}
	// dlog := &ivg.DestinationLogger{Destination: enc}
	dlog := enc
	gen := &generate.Generator{Destination: dlog}
	// Palette that can be referenced from CReg array, gets overidden with colors from by externally set palette.
	pal := ivg.DefaultPalette
	pal[0] = colornames.White
	pal[1] = colornames.Black
	gen.Reset(ivg.ViewBox{MinX: 0, MinY: 0, MaxX: 2406, MaxY: 2406}, pal)
	gen.SetCReg(0, true, ivg.PaletteIndexColor(0)) // CReg[0] => palette[0] (white) selected via adj 2
	gen.SetCReg(0, true, ivg.PaletteIndexColor(1)) // CReg[1] => palette[1] (black) selected via adj 1
	// CSel will now be set to 2
	adj := byte(2)
	gen.SetPathData("M1107.3 299.1c-198 0-373.9 127.3-435.2 315.3C544.8 640.6 434.9 720.2 370.5 833c-99.3 171.4-76.6 386.9 56.4 533.8-41.1 123.1-27 257.7 38.6 369.2 98.7 172 297.3 260.2 491.6 219.2 86.1 97 209.8 152.3 339.6 151.8 198 0 373.9-127.3 435.3-315.3 127.5-26.3 237.2-105.9 301-218.5 99.9-171.4 77.2-386.9-55.8-533.9v-.6c41.1-123.1 27-257.8-38.6-369.8-98.7-171.4-297.3-259.6-491-218.6-86.6-96.8-210.5-151.8-340.3-151.2zm0 117.5-.6.6c79.7 0 156.3 27.5 217.6 78.4-2.5 1.2-7.4 4.3-11 6.1L952.8 709.3c-18.4 10.4-29.4 30-29.4 51.4V1248l-155.1-89.4V755.8c-.1-187.1 151.6-338.9 339-339.2zm434.2 141.9c121.6-.2 234 64.5 294.7 169.8 39.2 68.6 53.9 148.8 40.4 226.5-2.5-1.8-7.3-4.3-10.4-6.1l-360.4-208.2c-18.4-10.4-41-10.4-59.4 0L1024 984.2V805.4L1372.7 604c51.3-29.7 109.5-45.4 168.8-45.5zM650 743.5v427.9c0 21.4 11 40.4 29.4 51.4l421.7 243-155.7 90L597.2 1355c-162-93.8-217.4-300.9-123.8-462.8C513.1 823.6 575.5 771 650 743.5zm807.9 106 348.8 200.8c162.5 93.7 217.6 300.6 123.8 462.8l.6.6c-39.8 68.6-102.4 121.2-176.5 148.2v-428c0-21.4-11-41-29.4-51.4l-422.3-243.7 155-89.3zM1201.7 997l177.8 102.8v205.1l-177.8 102.8-177.8-102.8v-205.1L1201.7 997zm279.5 161.6 155.1 89.4v402.2c0 187.3-152 339.2-339 339.2v-.6c-79.1 0-156.3-27.6-217-78.4 2.5-1.2 8-4.3 11-6.1l360.4-207.5c18.4-10.4 30-30 29.4-51.4l.1-486.8zM1380 1421.9v178.8l-348.8 200.8c-162.5 93.1-369.6 38-463.4-123.7h.6c-39.8-68-54-148.8-40.5-226.5 2.5 1.8 7.4 4.3 10.4 6.1l360.4 208.2c18.4 10.4 41 10.4 59.4 0l421.9-243.7z", adj)
	icon, err := enc.Bytes()
	if err != nil {
		panic(err)
	}
	return icon
}()

// ClampWidth will limit the min and max width of the layout.Context to the given
// values low and high. If the min width is greater than the max width, the min
// width will be set to the max width. If the min width is greater than the
// current width, the current width will be set to the min width. If the max
// width is less than the current width, the current width will be set to the
// max width.
func ClampWidth(gtx layout.Context, low, high unit.Dp) layout.Constraints {
	if gtx.Constraints.Min.X < gtx.Dp(low) {
		gtx.Constraints.Min.X = gtx.Dp(low)
	}
	if gtx.Constraints.Max.X > gtx.Dp(high) {
		gtx.Constraints.Max.X = gtx.Dp(high)
	}
	if gtx.Constraints.Min.X > gtx.Constraints.Max.X {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
	}
	return gtx.Constraints
}

func FillRect(gtx layout.Context, r image.Rectangle, radius int, c color.Color) {
	if radius == 0 {
		paint.FillShape(gtx.Ops, NRGBA(c), clip.Rect(r).Op())
	} else {
		paint.FillShape(gtx.Ops, NRGBA(c), clip.UniformRRect(r, radius).Op(gtx.Ops))
	}
}

func Material(ops *op.Ops, c color.Color) op.CallOp {
	m := op.Record(ops)
	paint.ColorOp{Color: NRGBA(c)}.Add(ops)
	return m.Stop()
}

func NRGBA(c color.Color) color.NRGBA {
	return color.NRGBAModel.Convert(c).(color.NRGBA)
}
