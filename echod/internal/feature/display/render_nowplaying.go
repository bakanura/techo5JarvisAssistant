//go:build !dot && !spot

package display

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"
	"time"
	"unicode"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"

	"github.com/HuskerMinion/techo5/echod/internal/feature/home"
	"github.com/HuskerMinion/techo5/echod/internal/i18n"
)

// nowPlaying is the idle screen while music plays or sits paused: the cover as a square on the left,
// and on the right where it plays, the song, who plays it and what comes next, the position, and one
// row of buttons. A station's logo stands on a card in the cover's place, and a song with no picture at
// all gets drawn notes (rings for talk radio). The ground takes a little of the cover's color, so the
// page belongs to the song without the words having to be read off a picture.
//
// It used to draw the cover across the whole screen under a wash with the words over it. A cover
// washed dark enough for the words to read did not look like the cover any more, and one left bright
// enough to look like itself put the words over somebody's face.
func (r *renderer) nowPlaying(s scene) {
	rd := s.radio
	cover, col := r.nowPlayingLayout()
	r.cover.prepare(rd.Thumb, cover.Dx())
	if r.cover.img != nil && !rd.Logo {
		top := lerp(walnut, r.cover.tint, 0.20)
		bottom := lerp(walnut, r.cover.tint, 0.06)
		r.vgradient(r.dst.Rect, top, bottom)
	}
	r.coverArt(rd, cover)
	if s.library {
		r.libraryPill(r.libraryButton())
	}

	station := rd.Now
	if station == "" {
		station = rd.Chosen
	}
	if station == "" {
		station = "Radio"
	}
	// Where it plays, or the station; the play button already says whether it does. "Playing in"
	// in front cost the room's name its end on the Show 5.
	label := station
	if rd.Now == "Music Assistant" {
		label = musicPlace(s.music)
	}
	if s.showLyrics && rd.Title != "" {
		// The words take the song's place below, so the head says which song they are.
		label = rd.Title
	}

	// The head of the column: where it plays, and the time on the right. The page is looked at for
	// minutes, and the big clock is not on it.
	head := col.Min.Y + r.s(26)
	clock := clockText(s.now)
	cw := r.width(r.small, clock)
	r.text(r.small, clock, col.Max.X-cw, head, dim)
	labelEnd := col.Max.X - cw - r.s(24)
	if s.lyrics != nil {
		b := r.lyricsButton(s.now)
		r.lyricsPill(b, s.showLyrics)
		labelEnd = b.Min.X - r.s(16)
	}
	r.text(r.small, r.fit(r.small, label, labelEnd-col.Min.X), col.Min.X, head, amber)

	fav, back, _, _, _ := r.nowPlayingButtons()
	bar := back.Min.Y - r.s(34)
	room := bar - r.s(44) // the words stop clear of the times over the bar

	if s.showLyrics {
		r.lyricWords(s, col, head, room)
	} else {
		headline := station
		if rd.Title != "" {
			headline = rd.Title
		}
		face := r.title
		if r.width(face, headline) > col.Dx() && len(r.wrap(face, headline, col.Dx())) > 2 {
			face = r.body
		}
		y := head + r.s(66)
		lines := r.wrap(face, headline, col.Dx())
		for i, line := range lines {
			if i == 2 {
				break
			}
			if i == 1 && len(lines) > 2 {
				line = r.clipTo(face, line+" "+strings.Join(lines[2:], " "), col.Dx())
			}
			r.text(face, line, col.Min.X, y, cream)
			y += r.s(52)
		}
		y -= r.s(52)
		line := func(face font.Face, text string, step int, c color.RGBA) {
			if text == "" || y+step > room {
				return
			}
			y += step
			r.text(face, r.clipTo(face, text, col.Dx()), col.Min.X, y, c)
		}
		line(r.body, rd.Artist, r.s(48), dim)
		line(r.small, rd.Album, r.s(42), dim)

		// A group says which rooms it reached; the queue says what comes next.
		if rd.Now == "Music Assistant" && len(s.music.Rooms) > 1 {
			line(r.small, strings.Join(s.music.Rooms, "  ·  "), r.s(40), amber)
		}
		if rd.Now == "Music Assistant" && s.music.Next != "" {
			line(r.small, i18n.T("Next")+"  ·  "+s.music.Next, r.s(40), dim)
		}
	}

	// Music Assistant's player state supplies a best-effort position/duration. Sendspin deliberately
	// has no position in its protocol, so an unavailable HA value leaves an honest empty bar rather
	// than inventing timing.
	if rd.Now == "Music Assistant" {
		r.musicProgress(s.music, s.now, image.Rect(col.Min.X, bar, col.Max.X, bar+r.s(6)))
	}

	// One row: the star, back, play or pause, forward, and stop. Sendspin routes these back to Music
	// Assistant, so the page controls the real queue/group rather than a local shadow player.
	_, back, play, next, stop := r.nowPlayingButtons()
	rad := float64(r.s(14))
	ground := shift(walnut, 10)
	if !dark() {
		ground = shift(walnut, -6)
	}
	for _, b := range []image.Rectangle{fav, back, next, stop} {
		r.roundButton(b, rad, ground)
	}
	r.roundButton(play, rad, amber)
	cx := func(b image.Rectangle) float64 { return float64(b.Min.X+b.Max.X) / 2 }
	cy := float64(play.Min.Y+play.Max.Y) / 2
	u := float64(r.s(13)) // half a mark's height
	r.markSkip(cx(back), cy, u, false, cream)
	r.markSkip(cx(next), cy, u, true, cream)
	if s.paused {
		r.aaPoly([][2]float64{{cx(play) - u*0.7, cy - u*1.15}, {cx(play) + u*1.1, cy}, {cx(play) - u*0.7, cy + u*1.15}}, onAccent())
	} else {
		bw := u * 0.62
		r.roundFillF(cx(play)-u*0.8, cy-u*1.05, cx(play)-u*0.8+bw, cy+u*1.05, bw/3, onAccent())
		r.roundFillF(cx(play)+u*0.8-bw, cy-u*1.05, cx(play)+u*0.8, cy+u*1.05, bw/3, onAccent())
	}
	starColor := dim
	if s.faved {
		starColor = amber
	}
	r.aaStar(cx(fav), cy, u*1.35, starColor)
	r.roundFillF(cx(stop)-u*0.8, cy-u*0.8, cx(stop)+u*0.8, cy+u*0.8, u*0.22, cream)
}

// nowPlayingLayout is the square the cover goes in and the column the words and buttons go in. The
// column is never narrower than the Show 5's 440: on the Show 8, which is less wide for its height,
// a cover the panel's full height would leave the buttons no room, so there the cover gives way.
// home's thumbSide makes the picture at this size, so the page does not scale it.
func (r *renderer) nowPlayingLayout() (cover, col image.Rectangle) {
	m := r.margin
	side := min(r.h-2*m, r.w-3*m-r.s(440))
	top := (r.h - side) / 2
	cover = image.Rect(m, top, m+side, top+side)
	col = image.Rect(cover.Max.X+m, top, r.w-m, top+side)
	return cover, col
}

// nowPlayingButtons is the row at the foot of the column, level with the bottom of the cover: the star
// and stop at its ends, the three transport buttons in the middle. The rectangles are the tap regions
// too.
func (r *renderer) nowPlayingButtons() (fav, back, play, next, stop image.Rectangle) {
	_, col := r.nowPlayingLayout()
	h := r.s(58)
	y := col.Max.Y - h
	fav = image.Rect(col.Min.X, y, col.Min.X+h, y+h)
	stop = image.Rect(col.Max.X-h, y, col.Max.X, y+h)
	back, play, next = r.transportButtons()
	return fav, back, play, next, stop
}

// transportButtons are back, play or pause, and forward, in the middle of the column. They are laid
// out from the panel's own size, because this page is drawn on the Show 8 as well: a width that fits
// the Show 5 is a third of a wider screen with the marks stranded in the corner of it.
func (r *renderer) transportButtons() (back, play, next image.Rectangle) {
	_, col := r.nowPlayingLayout()
	w, h, gap := r.s(92), r.s(58), r.s(12)
	x := col.Min.X + (col.Dx()-(3*w+2*gap))/2
	y := col.Max.Y - h
	back = image.Rect(x, y, x+w, y+h)
	play = image.Rect(x+w+gap, y, x+2*w+gap, y+h)
	next = image.Rect(x+2*(w+gap), y, x+3*w+2*gap, y+h)
	return back, play, next
}

// lyricsButton is the words button in the head of the column, left of the clock: a pill with lines of
// text drawn on it, because a word there cost the room's name most of its letters on the Show 5. It is
// there only for a song the music server has words for. The clock's width moves it, so a tap is checked
// against the same clock the frame drew.
func (r *renderer) lyricsButton(now time.Time) image.Rectangle {
	_, col := r.nowPlayingLayout()
	right := col.Max.X - r.width(r.small, clockText(now)) - r.s(18)
	top := col.Min.Y - r.s(6)
	return image.Rect(right-r.s(56), top, right, top+r.s(42))
}

// libraryButton is the button for Music Assistant's own pages (the library, the queue, the groups): a
// pill of books in the cover's lower right corner. In the head beside the words button it left the
// room's name about half of what "Wohnzimmer" needs on the Show 5, and the row of buttons has no gap
// for it; on the cover it costs nothing, and a tap anywhere else on the cover is still play/pause.
func (r *renderer) libraryButton() image.Rectangle {
	cover, _ := r.nowPlayingLayout()
	w, h, in := r.s(56), r.s(42), r.s(12)
	return image.Rect(cover.Max.X-in-w, cover.Max.Y-in-h, cover.Max.X-in, cover.Max.Y-in)
}

// libraryPill is the button: three books on a shelf, the last one leaning.
func (r *renderer) libraryPill(b image.Rectangle) {
	ground := shift(walnut, 10)
	if !dark() {
		ground = shift(walnut, -6)
	}
	r.roundButton(b, float64(b.Dy())/2, ground)
	x := float64(b.Min.X + r.s(17))
	cy, bw := float64(b.Min.Y+b.Max.Y)/2, float64(r.s(5))
	foot := cy + float64(r.s(9))
	for i, h := range []float64{17, 14} {
		left := x + float64(i)*(bw+float64(r.s(2)))
		r.roundFillF(left, foot-float64(r.s(int(h))), left+bw, foot, bw/4, cream)
	}
	lean := x + 2*(bw+float64(r.s(2)))
	top := foot - float64(r.s(16))
	r.aaPoly([][2]float64{{lean, foot}, {lean + bw, foot}, {lean + bw + float64(r.s(6)), top + float64(r.s(1))}, {lean + float64(r.s(6)), top}}, cream)
}

// lyricsPill is the button: lit while the words are up, on the ground's own color while they are not.
func (r *renderer) lyricsPill(b image.Rectangle, on bool) {
	ground, ink := shift(walnut, 10), cream
	if !dark() {
		ground = shift(walnut, -6)
	}
	if on {
		ground, ink = amber, onAccent()
	}
	r.roundButton(b, float64(b.Dy())/2, ground)
	// Three lines of a verse, ragged on the right.
	x := float64(b.Min.X + r.s(17))
	cy, gap, th := float64(b.Min.Y+b.Max.Y)/2, float64(r.s(8)), float64(r.s(4))
	for i, w := range []float64{22, 15, 19} {
		y := cy + float64(i-1)*gap
		r.roundFillF(x, y-th/2, x+float64(r.s(int(w))), y+th/2, th/2, ink)
	}
}

// lyricWords is the song's words between the head and the bar, in place of the song's details. Timed
// words keep the line being sung near the top, lit, with the one before it above when it is short; untimed ones
// have no line being sung and move down through the song as it goes.
func (r *renderer) lyricWords(s scene, col image.Rectangle, head, room int) {
	l := s.lyrics
	type row struct {
		text string
		line int
	}
	var rows []row
	first := make([]int, len(l.Lines))
	for i, ln := range l.Lines {
		first[i] = len(rows)
		text := ln.Text
		if text == "" && l.Synced {
			text = "♪" // a gap in timed words is the band playing
		} else if text == "" {
			rows = append(rows, row{"", i}) // and in untimed ones the gap between verses
			continue
		}
		for _, w := range r.wrap(r.body, text, col.Dx()) {
			rows = append(rows, row{r.clipTo(r.body, w, col.Dx()), i})
		}
	}
	step := r.s(50)
	top := head + r.s(62)
	fits := max((room-top)/step+1, 1)
	elapsed := time.Duration(s.music.Elapsed(s.now) * float64(time.Second))
	cur := l.Current(elapsed)
	start := 0
	switch {
	case l.Synced && cur >= 0:
		// The line before stays above for the eye coming back, unless it would show only its end.
		start = first[cur]
		if cur > 0 && first[cur]-first[cur-1] == 1 {
			start--
		}
	case !l.Synced && s.music.Duration > 0 && len(rows) > fits:
		start = int(float64(len(rows)-fits+1) * elapsed.Seconds() / s.music.Duration)
	}
	start = min(start, max(len(rows)-fits, 0))
	past := lerp(walnut, dim, 0.55)
	for i, y := start, top; i < len(rows) && y <= room; i, y = i+1, y+step {
		c := dim
		switch {
		case !l.Synced || rows[i].line == cur:
			c = cream
		case rows[i].line < cur:
			c = past
		}
		r.text(r.body, rows[i].text, col.Min.X, y, c)
	}
}

// coverArt draws the picture in its square: the cover with rounded corners and a shadow under it, a
// logo on a light card, or drawn notes or rings on a card of the ground's own color.
func (r *renderer) coverArt(rd home.Radio, b image.Rectangle) {
	rad := float64(r.s(18))
	r.roundShadow(b, rad, float64(r.s(18)), r.s(8), shadowAlpha())
	switch {
	case r.cover.img != nil && rd.Logo:
		r.roundFill(b, rad, color.RGBA{0xf4, 0xf2, 0xee, 0xff}, color.RGBA{0xe8, 0xe5, 0xe0, 0xff})
		pad := b.Dx() / 8
		inner := b.Inset(pad)
		xdraw.ApproxBiLinear.Scale(r.dst, inner, r.cover.img, r.cover.img.Bounds(), draw.Over, nil)
	case r.cover.img != nil:
		r.roundImage(b, rad, r.cover.img)
	default:
		card := shift(walnut, 12)
		if !dark() {
			card = shift(walnut, -10)
		}
		r.roundFill(b, rad, shift(card, 6), shift(card, -4))
		cx, cy, u := (b.Min.X+b.Max.X)/2, (b.Min.Y+b.Max.Y)/2, float64(b.Dx())/400
		r.faded(70, func() {
			if rd.Music || rd.Now == "Music Assistant" {
				// Two notes: heads, stems and a beam.
				r.disc(cx-int(75*u), cy+int(80*u), int(34*u), amber)
				r.disc(cx+int(75*u), cy+int(60*u), int(34*u), amber)
				r.stroke(cx-int(47*u), cy+int(70*u), cx-int(47*u), cy-int(110*u), int(14*u), amber)
				r.stroke(cx+int(103*u), cy+int(50*u), cx+int(103*u), cy-int(130*u), int(14*u), amber)
				r.stroke(cx-int(53*u), cy-int(110*u), cx+int(109*u), cy-int(130*u), int(30*u), amber)
				return
			}
			// Talk: rings going out from a transmitter dot, largest first so each ring shows.
			for _, rr := range []int{150, 100, 50} {
				r.disc(cx, cy, int(float64(rr+9)*u), amber)
				r.disc(cx, cy, int(float64(rr-9)*u), card)
			}
			r.disc(cx, cy, int(18*u), amber)
		})
	}
}

// roundImage copies img, which is b's size, into b with its corners rounded off.
func (r *renderer) roundImage(b image.Rectangle, rad float64, img *image.RGBA) {
	x0, y0, x1, y1 := rectF(b)
	corner := int(rad) + 1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		if y < r.dst.Rect.Min.Y || y >= r.dst.Rect.Max.Y {
			continue
		}
		sy := y - b.Min.Y
		edge := y < b.Min.Y+corner || y >= b.Max.Y-corner
		for x := b.Min.X; x < b.Max.X; x++ {
			if !edge && x == b.Min.X+corner {
				// The middle of the row has no corner in it: one copy.
				to := b.Max.X - corner
				copy(r.dst.Pix[r.dst.PixOffset(x, y):r.dst.PixOffset(to, y)], img.Pix[img.PixOffset(x-b.Min.X, sy):img.PixOffset(to-b.Min.X, sy)])
				x = to - 1
				continue
			}
			i := img.PixOffset(x-b.Min.X, sy)
			c := color.RGBA{img.Pix[i], img.Pix[i+1], img.Pix[i+2], 255}
			r.blendAt(x, y, c, clamp01(0.5-rrDist(float64(x)+0.5, float64(y)+0.5, x0, y0, x1, y1, rad)))
		}
	}
}

// coverCache is the cover as the page draws it, and the color it lends the ground. home makes the
// picture at the page's size already; this only scales one that came some other way, and only once.
type coverCache struct {
	src  *image.RGBA
	side int
	img  *image.RGBA
	tint color.RGBA
}

func (c *coverCache) prepare(src *image.RGBA, side int) {
	if src == c.src && side == c.side {
		return
	}
	c.src, c.side, c.img = src, side, nil
	if src == nil || side <= 0 {
		return
	}
	if src.Bounds().Dx() == side && src.Bounds().Dy() == side && src.Bounds().Min == (image.Point{}) {
		c.img = src
	} else {
		c.img = image.NewRGBA(image.Rect(0, 0, side, side))
		xdraw.CatmullRom.Scale(c.img, c.img.Bounds(), src, src.Bounds(), draw.Src, nil)
	}
	c.tint = averageColor(c.img)
}

// averageColor is a picture's mean color, from every eighth pixel each way, which is plenty for a tint.
func averageColor(img *image.RGBA) color.RGBA {
	var rs, gs, bs, n int
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y += 8 {
		for x := b.Min.X; x < b.Max.X; x += 8 {
			i := img.PixOffset(x, y)
			if img.Pix[i+3] == 0 {
				continue
			}
			rs, gs, bs, n = rs+int(img.Pix[i]), gs+int(img.Pix[i+1]), bs+int(img.Pix[i+2]), n+1
		}
	}
	if n == 0 {
		return walnut
	}
	return color.RGBA{uint8(rs / n), uint8(gs / n), uint8(bs / n), 255}
}

// markSkip is back or forward: a bar and a triangle pointing away from it, centered on cx, cy, u high
// from the middle.
func (r *renderer) markSkip(cx, cy, u float64, forward bool, c color.RGBA) {
	bw, tw := u*0.38, u*1.25
	w := bw + u*0.15 + tw
	x := cx - w/2
	if forward {
		r.aaPoly([][2]float64{{x, cy - u}, {x + tw, cy}, {x, cy + u}}, c)
		r.roundFillF(x+tw+u*0.15, cy-u, x+w, cy+u, bw/3, c)
		return
	}
	r.roundFillF(x, cy-u, x+bw, cy+u, bw/3, c)
	r.aaPoly([][2]float64{{x + w, cy - u}, {x + w - tw, cy}, {x + w, cy + u}}, c)
}

// aaStar fills a five-pointed star centered at cx, cy with outer radius rad, smooth at the edges.
func (r *renderer) aaStar(cx, cy, rad float64, c color.RGBA) {
	pts := make([][2]float64, 10)
	for i := range pts {
		a := -math.Pi/2 + float64(i)*math.Pi/5
		rr := rad
		if i%2 == 1 {
			rr *= 0.45
		}
		pts[i] = [2]float64{cx + rr*math.Cos(a), cy + rr*math.Sin(a)}
	}
	r.aaPoly(pts, c)
}

// aaPoly fills a polygon, each edge pixel by how many of sixteen samples in it fall inside.
func (r *renderer) aaPoly(pts [][2]float64, c color.RGBA) {
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range pts {
		x0, y0, x1, y1 = min(x0, p[0]), min(y0, p[1]), max(x1, p[0]), max(y1, p[1])
	}
	for y := int(math.Floor(y0)); y <= int(math.Ceil(y1)); y++ {
		for x := int(math.Floor(x0)); x <= int(math.Ceil(x1)); x++ {
			in := 0
			for sy := 0; sy < 4; sy++ {
				for sx := 0; sx < 4; sx++ {
					if inPolygon(float64(x)+(float64(sx)+0.5)/4, float64(y)+(float64(sy)+0.5)/4, pts) {
						in++
					}
				}
			}
			r.blendAt(x, y, c, float64(in)/16)
		}
	}
}

// roundFillF is a rounded rectangle in one color at fractional coordinates, for marks too small for
// whole pixels to look even.
func (r *renderer) roundFillF(x0, y0, x1, y1, rad float64, c color.RGBA) {
	for y := int(math.Floor(y0)); y <= int(math.Ceil(y1)); y++ {
		for x := int(math.Floor(x0)); x <= int(math.Ceil(x1)); x++ {
			r.blendAt(x, y, c, clamp01(0.5-rrDist(float64(x)+0.5, float64(y)+0.5, x0, y0, x1, y1, rad)))
		}
	}
}

// musicPlace is where Music Assistant plays, for the head of the now-playing page: the group, the one
// room it resolved to, or the speaker it fell back to.
func musicPlace(v home.MusicPlaybackView) string {
	switch {
	case v.Route != "":
		return prettyMusicName(v.Route)
	case len(v.Rooms) == 1:
		return prettyMusicName(v.Rooms[0])
	case v.Output != "":
		return v.Output
	default:
		return "Music Assistant"
	}
}

func prettyMusicName(v string) string {
	v = strings.TrimSpace(strings.ReplaceAll(v, "_", " "))
	if v == "" {
		return v
	}
	r := []rune(v)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// musicProgress is the position as a rounded bar in b, with the time gone and the length over its ends.
// The position moves on from Music Assistant's last report while the song plays, so the time gone
// counts up between reports rather than jumping.
func (r *renderer) musicProgress(v home.MusicPlaybackView, now time.Time, b image.Rectangle) {
	pos := v.Elapsed(now)
	rad := float64(b.Dy()) / 2
	track := shift(walnut, 18)
	if !dark() {
		track = shift(walnut, -18)
	}
	r.roundFillF(float64(b.Min.X), float64(b.Min.Y), float64(b.Max.X), float64(b.Max.Y), rad, track)
	if v.Duration > 0 {
		p := min(max(pos/v.Duration, 0), 1)
		fill := float64(b.Min.X) + float64(b.Dx())*p
		if fill > float64(b.Min.X)+2*rad {
			r.roundFillF(float64(b.Min.X), float64(b.Min.Y), fill, float64(b.Max.Y), rad, amber)
		}
		left := mediaClock(pos)
		right := mediaClock(v.Duration)
		r.text(r.tiny, left, b.Min.X, b.Min.Y-r.s(12), dim)
		r.text(r.tiny, right, b.Max.X-r.width(r.tiny, right), b.Min.Y-r.s(12), dim)
	}
}

func mediaClock(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	n := int(seconds + .5)
	return fmt.Sprintf("%d:%02d", n/60, n%60)
}

// The marks are 36 rows high from x and top at the Show 5's width, drawn as rows the way this panel has
// always drawn its play mark. Sizes go through s() so a wider panel gets marks to match its buttons.

func (r *renderer) markBack(x, top int) {
	r.markTriangle(x+r.s(14), top, r.s(30), false)
	r.markBar(x, top)
}

func (r *renderer) markNext(x, top int) {
	r.markTriangle(x, top, r.s(30), true)
	r.markBar(x+r.s(44), top)
}

func (r *renderer) markPlay(x, top int) { r.markTriangle(x, top, r.s(36), true) }

func (r *renderer) markPause(x, top int) {
	r.markBar(x, top)
	r.markBar(x+r.s(19), top)
}

func (r *renderer) markBar(x, top int) {
	mark := image.Rect(x, top, x+r.s(11), top+r.s(36))
	draw.Draw(r.dst, mark, image.NewUniform(amber), image.Point{}, draw.Src)
}

// markTriangle draws a triangle pointing right or left, widest in the middle.
func (r *renderer) markTriangle(x, top, size int, right bool) {
	for i := 0; i < size; i++ {
		half := min(i, size-1-i)
		w := half*3/2 + 1
		x0 := x
		if !right {
			x0 = x + (size/2)*3/2 + 1 - w
		}
		draw.Draw(r.dst, image.Rect(x0, top+i, x0+w, top+i+1), image.NewUniform(amber), image.Point{}, draw.Src)
	}
}
