// The style grid: one card per syntax style, and the click that dresses the
// whole system in one of them.
//
// # Why a style is a seed at all
//
// A syntax style is a palette somebody balanced — a dozen inks chosen against
// one another and against a ground, argued over for years in some cases. The
// window's other door asks for a picture and finds the colours in it; this one
// asks for nothing and offers colours that were already settled. Running a
// style's inks through the same extractor a photograph goes through is what
// makes the two doors one door: what comes back is a row of candidates ranked
// the same way, so everything downstream — the row, the ring, the keep
// affordance, the base list's override — is the machinery that was already
// there, unchanged.
//
// # What is on a card
//
// The style's dominant inks as one strip, the leading one twice as wide as any
// other because it is the one a click takes as the seed; the primary pair that
// seed derives, drawn the way the candidate row draws it, so a card promises
// what choosing it delivers; and the name, with a word at its trailing edge for
// the one thing about the style the name does not say.
//
// That word is where a palette drawn faint gets mentioned. A style whose own
// inks mostly fall under the contrast floor on its own ground carries it, in
// the muted ink the other words at that edge are set in and in the same slot,
// because it is the same kind of remark: a fact about the style that a person
// reading its name would otherwise find out by applying it. It is not a
// warning, and nothing follows from it — the card still offers the style, the
// click still applies it, and the fence still draws it exactly as its author
// drew it. Contrast in content is surfaced here, not enforced.
//
// # Which cards, and in which order
//
// Vivid first, by the chroma of the leading candidate — an absolute quantity in
// the space seeds are ranked in, and therefore the one axis that means the same
// thing in two different styles' answers. The prominence weight the candidates
// inside one style are ordered by is not that axis: it is a share times a
// chroma, and a share is a fraction of one palette, so a style with three inks
// would outrank a style with sixty on the strength of having fewer colours.
// Ties fall back to the order the styles are listed in, which is alphabetical,
// so the grid is the same grid on every machine and in every run.
//
// The cards are filtered by the scheme control exactly as the base list is: the
// sun shows the styles fitted to a light ground and the moon those fitted to a
// dark one, measured off each style's own background rather than read off its
// name. A style that names no ground is on both.
//
// One style gets no card. bw colours nothing at all — it draws code in the
// plain foreground and takes no position on anything else — so it yields no
// palette, no candidate and no seed. Its card would be an empty rectangle
// offering a click that could not do anything, so it is left off and the base
// list, where choosing it is still a coherent thing to want, still lists it.
package main

import (
	"fmt"
	"image"
	stdcolor "image/color"
	"runtime"
	"sort"
	"sync"

	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/vibrantgio/components/list"
	"github.com/vibrantgio/components/scrollbar"
	"github.com/vibrantgio/markdown/highlight"
	"github.com/vibrantgio/mvu"
	"github.com/vibrantgio/textdraw"
	"github.com/vibrantgio/theme/imageseed"
	"github.com/vibrantgio/theme/tokens"
)

// The grid's dimensions. A card is wider than it is tall and shorter than a
// candidate card: it carries a strip, a chip and a name rather than a swatch,
// a chip and two numbers, and there are forty of them rather than six.
const (
	// StyleMinW is set where it is because a card whose name is truncated is a
	// card that has stopped identifying its style, and the longest names in
	// the set run to seventeen characters. Six columns fit in this window at
	// 150 and truncated a dozen of them; five at this width truncate none.
	StyleMinW unit.Dp = 178
	// StyleH leaves the strip a third of the card rather than most of it.
	// Forty cards of full-chroma ink at their own widest is a test card and
	// not a gallery: with the colour taking half the tile there was nothing
	// on the screen for an eye to rest on, and the tile under it — a hairline
	// and a fill a couple of rungs off the page — disappeared beside the ink,
	// so a card read as a sample rather than as something to press. The inks
	// are not toned down to fix that. They are the styles' own colours and
	// the whole promise of a card is that clicking it applies them; a grid of
	// muted swatches would be a grid that lied. What is toned down is how
	// much of each card they are.
	StyleH     unit.Dp = 82
	StyleGap   unit.Dp = 12
	StylePad   unit.Dp = 10 // card edge to the strip inside it
	StyleChipW unit.Dp = 30 // the derived primary pair, beside the name
	StyleChipH unit.Dp = 22
	StyleFoot  unit.Dp = 8  // strip to the line under it
	StyleTagW  unit.Dp = 34 // the word at a card's trailing edge
	StyleHead  unit.Dp = 40 // the two lines heading the grid
	// DropH is the well's height on the first screen. It is more than twice a
	// card tall and the full width of the page, which is what keeps it the
	// primary invitation while leaving four rows of the grid above the fold.
	DropH unit.Dp = 184
)

// StyleLeadShare is how many shares of the swatch strip the leading ink takes
// against one for each of the others. It is what says which of the colours on
// a card is the one a click applies: equal bands would make a card a palette
// swatch, and a palette swatch does not tell anybody what pressing it does.
const StyleLeadShare = 2

// What heads the grid. The invitation is a line of its own under the label
// rather than a clause at the far right of it: what a click does is the one
// thing about this grid nobody can work out by looking, and the smallest,
// palest text in the corner of a screen is not where a promise goes.
const (
	StyleLabel  = "Or start from a style"
	StyleInvite = "One click takes the seed and both syntax bases off a card."
	// StyleFaint is the word a card carries when the palette it offers draws
	// most of its code under the contrast floor on its author's own ground.
	// It is a description and not a verdict: the style still applies, still
	// draws exactly as its author drew it, and is still worth choosing if it
	// is the one somebody wants.
	StyleFaint = "faint"
)

// StyleCountFor says how many cards are on screen, which half of the set they
// are, and what put them in that order. The half matters for the reason it
// matters on the base list — a grid showing some of the styles there are, with
// nothing saying so, reads as one that failed to load the rest — and the
// ordering matters because a grid that opens on its loudest cards looks
// arbitrary until something says it is not.
func StyleCountFor(dark bool, n int) string {
	if dark {
		return fmt.Sprintf("%d dark styles, vivid first", n)
	}
	return fmt.Sprintf("%d light styles, vivid first", n)
}

// Chip is a derived primary pair as a card wears it: the colour and the ink
// that proves the colour is legible under something.
type Chip struct {
	Fill, Ink stdcolor.NRGBA
}

// StyleCard is one style offered as a seed: the candidates its palette yields,
// the pair a click applies, and the two chips it is drawn with.
//
// All of it is resolved once, before the first frame. None of it can change
// while the window is open — a style's inks are a fact about a file — and
// deriving forty palettes per frame to learn that would be a waste of a frame.
type StyleCard struct {
	Name string
	// Added marks a style read from the styles folder rather than one that
	// ships embedded.
	Added bool
	// Light and Dark are the appearances this style was fitted to, measured
	// off its own ground. A style fitted to no ground carries both.
	Light, Dark bool
	// Faint marks a style that draws most of its code under the contrast
	// floor on its own ground — measured, not judged, and measured against
	// the ground its own author chose rather than against this window's.
	//
	// It is here and not asked per frame for the reason none of the rest is:
	// a style's inks are a fact about a file, and it cannot change while the
	// window is open.
	Faint bool
	// Candidates are the seeds its palette yields, most prominent first.
	// They are what a click hands the candidate row, so the row a style
	// produces is the row a picture produces.
	Candidates []imageseed.Candidate
	// Pair is what a click applies: this style on the side its author fitted
	// it to, and the nearest measured answer on the other.
	Pair highlight.BasePair
	// Chips are the primary pair the leading candidate derives, one per
	// appearance, so the card can promise under either scheme what choosing
	// it delivers.
	Chips [2]Chip
}

// Seed is the colour a click on this card applies.
func (s StyleCard) Seed() stdcolor.NRGBA { return s.Candidates[0].Color }

// Chroma is how much colour the style's leading ink has: the axis the grid is
// ordered on.
func (s StyleCard) Chroma() float64 { return s.Candidates[0].Chroma }

// Suits reports whether this card belongs on the given appearance's grid.
func (s StyleCard) Suits(dark bool) bool {
	if dark {
		return s.Dark
	}
	return s.Light
}

// Chip is the pair the card is drawn with under one appearance.
func (s StyleCard) Chip(dark bool) Chip {
	if dark {
		return s.Chips[1]
	}
	return s.Chips[0]
}

// styleCards is every style that has a colour to offer, vivid first.
//
// The cards are built across the machine's cores rather than one after
// another. Seventy-odd palette derivations are what the grid costs, at over a
// millisecond and a half each, and a window that spent a sixth of a second
// deriving before it drew its first frame would open visibly late — on a
// screen whose whole content is these cards. Nothing here shares anything: a
// style's card is a function of that style's file, each worker writes its own
// slot, and the order comes from the slice and the sort rather than from which
// worker finished when, so the grid is byte for byte the same grid however
// many cores it was built on.
func styleCards() []StyleCard {
	names := highlight.Bases()
	built := make([]StyleCard, len(names))
	var wg sync.WaitGroup
	next := make(chan int)
	for range min(runtime.GOMAXPROCS(0), len(names)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				built[i] = styleCardFor(names[i])
			}
		}()
	}
	for i := range names {
		next <- i
	}
	close(next)
	wg.Wait()

	out := make([]StyleCard, 0, len(built))
	for _, c := range built {
		if len(c.Candidates) > 0 { // a style that colours nothing has no seed to offer
			out = append(out, c)
		}
	}
	// Stable over the alphabetical order the names arrived in, so two styles
	// whose leading inks are equally vivid — and pure blue is four styles'
	// leading ink — come out in the same order every time.
	sort.SliceStable(out, func(i, j int) bool { return out[i].Chroma() > out[j].Chroma() })
	return out
}

// styleCardFor resolves one style into everything a card and a click on it
// need. It comes back with no candidates for a style that colours nothing.
func styleCardFor(name string) StyleCard {
	cands := imageseed.ExtractPalette(highlight.BasePalette(name))
	if len(cands) == 0 {
		return StyleCard{Name: name}
	}
	light, dark := tokens.FromSeed(cands[0].Color)
	authored, measured := highlight.BaseContrast(name)
	return StyleCard{
		Name:       name,
		Added:      highlight.Loaded(name),
		Light:      highlight.BaseSuits(name, false),
		Dark:       highlight.BaseSuits(name, true),
		Faint:      measured && authored.BelowFloor(),
		Candidates: cands,
		Pair:       highlight.CompletePair(name),
		Chips: [2]Chip{
			{Fill: light.Primary, Ink: light.OnPrimary},
			{Fill: dark.Primary, Ink: dark.OnPrimary},
		},
	}
}

// styleGrid is what the grid keeps across emissions: where it is scrolled, one
// click handler per style, and which half of the set was on screen last time.
//
// The handlers are one per style and not one per visible card, which is what
// makes the filter cost nothing: a style keeps its handler when half the grid
// goes, so a press in flight cannot be handed to another style by the scheme
// changing under it.
type styleGrid struct {
	st     *list.State
	clicks []gesture.Click
	shown  bool
	dark   bool
}

func newStyleGrid() *styleGrid { return &styleGrid{st: list.NewState()} }

// handlers returns n click handlers, allocating on the first call.
func (g *styleGrid) handlers(n int) []gesture.Click {
	if len(g.clicks) < n {
		g.clicks = make([]gesture.Click, n)
	}
	return g.clicks
}

// follow puts the grid back at the top when the scheme control has replaced
// one half of the set with the other. A scroll offset measured against the
// cards that are gone points at nothing in the cards that arrived, and the new
// grid's own vivid end is what somebody flipping the switch is looking for.
// Between flips the grid is left exactly where it was put.
func (g *styleGrid) follow(dark bool) {
	if g.shown && g.dark == dark {
		return
	}
	g.shown, g.dark = true, dark
	g.st.ScrollToStart()
}

// StyleGrid draws the cards under the drop well: a label saying what they are
// and how many, and under it as many columns as the window is wide enough for,
// scrolled rather than cut off.
func StyleGrid(p Palette, c tokens.ColorTokens, ty Type, m Model, dark bool, g *styleGrid) layout.Widget {
	visible := m.VisibleStyles(dark)
	clicks := g.handlers(len(m.Styles))
	g.follow(dark)
	count := StyleCountFor(dark, len(visible))
	st := g.st
	return func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		headH := gtx.Dp(StyleHead)
		line := image.Rect(0, 0, size.X, headH/2)
		under := image.Rect(0, headH/2, size.X, headH)
		textdraw.FillText(gtx, ty.Shaper, ty.Label, line, 0, 0.5, p.Text, StyleLabel)
		textdraw.FillText(gtx, ty.Shaper, ty.Small, line, 1, 0.5, p.Muted, count)
		// Full-strength ink, small: it is a promise and not a footnote.
		textdraw.FillText(gtx, ty.Shaper, ty.Small, under, 0, 0.5, p.Text, StyleInvite)

		top := headH + gtx.Dp(RowTop)
		if top >= size.Y || len(visible) == 0 {
			return layout.Dimensions{Size: size}
		}
		// The bar takes a gutter rather than floating over the cards: a card's
		// trailing edge carries the word saying where the style came from, and
		// a thumb resting over the last column would hide it on exactly the
		// cards a reader is most likely to be unsure about. It stands rather
		// than fading, as the base list's does and for the same reason: the
		// grid is cut off mid-row at the bottom of the window, and a cut with
		// nothing beside it reads as the set finishing there.
		bar := scrollbar.FromTokens(c)
		bar.FadeDelay = 0
		gap := gtx.Dp(StyleGap)
		cols := styleColumns(size.X-gtx.Dp(bar.Width()), gap, gtx.Dp(StyleMinW))
		rows := chunk(visible, cols)
		at(gtx, image.Pt(0, top), func(gtx layout.Context) {
			gtx.Constraints = layout.Exact(image.Pt(size.X, size.Y-top))
			list.LayoutScrollbar(gtx, st, bar, list.Occupy, rows,
				func(gtx layout.Context, row []int) layout.Dimensions {
					return StyleRow(gtx, p, ty, m, dark, row, cols, clicks)
				})
		})
		return layout.Dimensions{Size: size}
	}
}

// StyleRow draws one row of the grid: up to cols cards sharing the row's width
// so the grid reaches the same right margin as the label over it, whatever the
// last row holds.
func StyleRow(gtx layout.Context, p Palette, ty Type, m Model, dark bool, row []int, cols int, clicks []gesture.Click) layout.Dimensions {
	gap := gtx.Dp(StyleGap)
	width := gtx.Constraints.Max.X
	cell := image.Pt(max(1, (width-(cols-1)*gap)/cols), gtx.Dp(StyleH))
	for col, i := range row {
		x := col * (cell.X + gap)
		at(gtx, image.Pt(x, 0), func(gtx layout.Context) {
			StyleCell(gtx, p, ty, m.Styles[i], i, dark, &clicks[i], cell)
		})
	}
	return layout.Dimensions{Size: image.Pt(width, cell.Y+gap)}
}

// StyleCell draws one card at the origin and makes it clickable.
func StyleCell(gtx layout.Context, p Palette, ty Type, s StyleCard, index int, dark bool, click *gesture.Click, size image.Point) {
	card := image.Rectangle{Max: size}
	fill, edge := p.Surface, p.CardEdge
	if click.Hovered() {
		fill, edge = p.Selection, p.Accent
	}
	fillRRect(gtx, card, gtx.Dp(Radius), fill)
	strokeRRect(gtx, card, gtx.Dp(Radius), gtx.Dp(Hairline), edge)

	inner := card.Inset(gtx.Dp(StylePad))
	foot := image.Rect(inner.Min.X, inner.Max.Y-gtx.Dp(StyleChipH), inner.Max.X, inner.Max.Y)
	strip := image.Rect(inner.Min.X, inner.Min.Y, inner.Max.X, foot.Min.Y-gtx.Dp(StyleFoot))
	if strip.Dy() > 0 {
		SwatchBands(gtx, strip, gtx.Dp(InnerR), s.Candidates, p.Edge)
	}

	chip := image.Rect(foot.Min.X, foot.Min.Y, min(foot.Min.X+gtx.Dp(StyleChipW), foot.Max.X), foot.Max.Y)
	pair := s.Chip(dark)
	fillRRect(gtx, chip, gtx.Dp(InnerR), pair.Fill)
	textdraw.FillText(gtx, ty.Shaper, ty.Small, chip, 0.5, 0.5, pair.Ink, "Aa")

	name := image.Rect(chip.Max.X+gtx.Dp(StyleFoot), foot.Min.Y, foot.Max.X, foot.Max.Y)
	if tag := styleTag(s); tag != "" {
		box := image.Rect(max(name.Min.X, foot.Max.X-gtx.Dp(StyleTagW)), foot.Min.Y, foot.Max.X, foot.Max.Y)
		textdraw.FillText(gtx, ty.Shaper, ty.Small, box, 1, 0.5, p.Muted, tag)
		name.Max.X = box.Min.X - gtx.Dp(StyleFoot)/2
	}
	textdraw.FillText(gtx, ty.Shaper, ty.Small, name, 0, 0.5, p.Text, s.Name)

	// The clickable area is the card, registered after the paint so the hover
	// state read above is the one the previous frame recorded.
	area := clip.UniformRRect(card, gtx.Dp(Radius)).Push(gtx.Ops)
	click.Add(gtx.Ops)
	area.Pop()
	for {
		e, ok := click.Update(gtx.Source)
		if !ok {
			break
		}
		if e.Kind == gesture.KindClick {
			mvu.MessageOp{Message: AdoptStyle{Index: index}}.Add(gtx.Ops)
		}
	}
}

// SwatchBands paints candidates side by side inside r, the leading one widest,
// under one rounded outline. It is what stands in for a picture when the
// colours on screen came out of a style instead of one: the same colours the
// row below carries, in the same order, in the place the photograph would be.
//
// The frame drawn in edge is not decoration. Plenty of styles carry a
// near-white ink, and plenty of photographs do; a near-white band at the end of
// a strip standing on a near-white card has no boundary of its own, so the
// strip appears to stop short and reads as something that failed to finish
// drawing rather than as a colour somebody chose. The frame is what says the
// band is there.
//
// It is a fill with the colours inset into it rather than a stroke over them.
// A one-point stroke is centred on the boundary, so it lands as two rows of
// half-strength antialiasing rather than as a line — which is enough of a line
// between an ink and a card that differ, and not enough between two
// near-whites, exactly the case the frame exists for. A fill and an inset put a
// whole pixel of the border colour there at its own strength.
func SwatchBands(gtx layout.Context, r image.Rectangle, radius int, cands []imageseed.Candidate, edge stdcolor.NRGBA) {
	if len(cands) == 0 || r.Dx() <= 0 || r.Dy() <= 0 {
		return
	}
	fillRRect(gtx, r, radius, edge)
	line := gtx.Dp(Hairline)
	in := r.Inset(line)
	if in.Dx() <= 0 || in.Dy() <= 0 {
		return
	}
	defer clip.UniformRRect(in, max(0, radius-line)).Push(gtx.Ops).Pop()
	total, cum := StyleLeadShare+len(cands)-1, 0
	for i, c := range cands {
		x0 := in.Min.X + in.Dx()*cum/total
		if i == 0 {
			cum += StyleLeadShare
		} else {
			cum++
		}
		x1 := in.Min.X + in.Dx()*cum/total
		if i == len(cands)-1 {
			x1 = in.Max.X // rounding never leaves a sliver of the frame showing
		}
		paint.FillShape(gtx.Ops, c.Color, clip.Rect(image.Rect(x0, in.Min.Y, x1, in.Max.Y)).Op())
	}
}

// styleTag is the word at a card's trailing edge, or none: one thing about the
// style that the name in front of it does not say.
//
// It is the slot the base list already keeps for where a style came from, and
// how the style draws goes in that slot rather than beside it. The footer is
// one line and it already carries a chip, a name and a word; the longest names
// in the set reach that word as it is, so a second one at the same edge would
// be bought by truncating the name — which is the one thing on a card that says
// which style it is.
//
// The measured word wins where a style is both. It is the more consequential
// of the two — how a palette draws is what somebody is choosing, where it came
// from is a note about the file — and it is also the only place the fact is
// said at all, the base list saying where a style came from on its own rows.
func styleTag(s StyleCard) string {
	if s.Faint {
		return StyleFaint
	}
	return originTag(s.Added, s.Light, s.Dark)
}

// styleColumns is how many cards fit across width dp, gap dp apart, at no less
// than narrowest dp each — at least one, however narrow the window is.
func styleColumns(width, gap, narrowest int) int {
	if width <= 0 || narrowest <= 0 {
		return 1
	}
	return max(1, (width+gap)/(narrowest+gap))
}

// chunk cuts a list of card indices into rows of at most n.
func chunk(all []int, n int) [][]int {
	if n <= 0 {
		return nil
	}
	out := make([][]int, 0, (len(all)+n-1)/n)
	for i := 0; i < len(all); i += n {
		out = append(out, all[i:min(i+n, len(all))])
	}
	return out
}
