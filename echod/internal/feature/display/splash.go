//go:build !dot && !spot

package display

import (
	"image"
	"image/draw"
	"math"
	"time"
)

// The Jarvis Show mark, drawn while the device comes up. It deliberately uses the active screen
// palette rather than embedding an upstream TECHO5 bitmap, so a fresh White Jade device has one
// visual identity from the first daemon-rendered frame onward. Signal arcs pulse outward until Home
// Assistant is listening.

const (
	// The pulse: arcs leave the mark at arcFrom pixels from the center and fade out by arcTo,
	// arcCount of them in flight, one full sweep every arcPeriod.
	arcFrom   = 100.0
	arcTo     = 380.0
	arcCount  = 3
	arcPeriod = 2400 * time.Millisecond
	arcWidth  = 5.0
	// arcSpread is how far above and below the horizontal the arcs reach, in radians.
	arcSpread = 38 * math.Pi / 180

	// waitingAfter is when the splash starts saying what it is waiting for: long enough that a
	// device which is already adopted is up and gone before it shows, short enough that nobody
	// sits watching a screen that looks stuck.
	waitingAfter = 12 * time.Second

	// splashMin is the least the splash is shown, so a fast connection still shows the mark.
	splashMin = 4 * time.Second

	// noAddressWait is how long after the start a device with no network address waits before the Wi-Fi
	// page opens by itself, ending the splash: long enough for a lease on a slow network, short enough
	// that a fresh unit, or one in a house it has no network for, is not left on the splash.
	noAddressWait = 45 * time.Second

	// noHomeAssistantWait is how long a device with no Home Assistant access waits on the splash for
	// Home Assistant to add it before showing the clock: a device already in a Home Assistant is
	// usually listening well inside it.
	noHomeAssistantWait = 60 * time.Second
)

// splash is the theme-aware wordmark's geometry and where its arcs are centered on the canvas.
type splash struct {
	cx, cy       float64
	titleY, subY int
}

func newSplash(w, h int) *splash {
	cy := float64(h)/2 - 22
	return &splash{
		cx:     float64(w) / 2,
		cy:     cy,
		titleY: int(cy) + 10,
		subY:   int(cy) + 54,
	}
}

// draw paints the splash for the moment t into the elapsed animation.
func (r *renderer) drawSplash(s *splash, elapsed time.Duration) {
	draw.Draw(r.dst, r.dst.Rect, image.NewUniform(walnut), image.Point{}, draw.Src)
	if s == nil {
		mark := "JARVIS"
		r.text(r.title, mark, (r.w-r.width(r.title, mark))/2, r.h/2, cream)
		return
	}
	r.arcs(s, elapsed)
	mark := "JARVIS"
	r.text(r.title, mark, (r.w-r.width(r.title, mark))/2, s.titleY, cream)
	sub := "SHOW"
	r.text(r.small, sub, (r.w-r.width(r.small, sub))/2, s.subY, amber)
	if elapsed >= waitingAfter {
		r.splashWaiting(s)
	}
}

// splashWaiting says what the splash is waiting for, once it has been up long enough that waiting is
// the explanation.
//
// The mark stays until Home Assistant subscribes, which does not happen until somebody accepts the
// device there. Between flashing a unit and adopting it that can be minutes, or as long as it takes
// to walk to a computer, and the screen said nothing at all - so it reads as a device that has hung
// on its first boot. Somebody sat in front of one for ten minutes before finding out that accepting
// the ESPHome prompt was what freed it (techo5-checkers issue #2). It costs two lines to say so.
//
// Not said from the first frame: a device that is adopted already reaches Home Assistant in a couple
// of seconds, and telling that owner to go and do something they did not need to do would be worse
// than saying nothing.
func (r *renderer) splashWaiting(s *splash) {
	const (
		what  = "Waiting for Home Assistant"
		where = "Settings > Devices & services > ESPHome"
	)
	// Below the wordmark. If a panel ever leaves less room than they need, the lines sit on the
	// bottom edge instead of climbing onto the mark.
	const gap, lead = 26, 32
	y := s.subY + gap + 34
	if bottom := r.h - 6; y+lead > bottom {
		y = bottom - lead
	}
	r.text(r.tiny, what, (r.w-r.width(r.tiny, what))/2, y, amber)
	r.text(r.tiny, where, (r.w-r.width(r.tiny, where))/2, y+lead, dim)
}

// arcs draws arcCount rings expanding from the mark to both sides, each fading as it travels.
// Only the band the arcs can reach is scanned, and only pixels near a ring are touched.
func (r *renderer) arcs(s *splash, elapsed time.Duration) {
	phase := math.Mod(elapsed.Seconds()/arcPeriod.Seconds(), 1)
	var radii [arcCount]float64
	var fades [arcCount]float64
	for i := range arcCount {
		p := math.Mod(phase+float64(i)/arcCount, 1)
		radii[i] = arcFrom + p*(arcTo-arcFrom)
		fades[i] = (1 - p) * (1 - p) // brighter near the mark, gone at the edge
	}
	x0 := max(int(s.cx-arcTo-arcWidth), 0)
	x1 := min(int(s.cx+arcTo+arcWidth), r.w-1)
	y0 := max(int(s.cy-arcTo*math.Sin(arcSpread)-arcWidth), 0)
	y1 := min(int(s.cy+arcTo*math.Sin(arcSpread)+arcWidth), r.h-1)
	pix := r.dst.Pix
	for y := y0; y <= y1; y++ {
		dy := float64(y) - s.cy
		for x := x0; x <= x1; x++ {
			dx := float64(x) - s.cx
			if math.Abs(dy) > math.Abs(dx)*math.Tan(arcSpread) {
				continue // outside the two sideways cones
			}
			d := math.Hypot(dx, dy)
			for i := range arcCount {
				w := math.Abs(d - radii[i])
				if w > arcWidth {
					continue
				}
				a := (1 - w/arcWidth) * fades[i]
				// soften the cone edges
				edge := 1 - math.Abs(dy)/(math.Abs(dx)*math.Tan(arcSpread)+1e-9)
				a *= math.Min(edge*4, 1)
				o := r.dst.PixOffset(x, y)
				pix[o] = blend(pix[o], amber.R, a)
				pix[o+1] = blend(pix[o+1], amber.G, a)
				pix[o+2] = blend(pix[o+2], amber.B, a)
			}
		}
	}
}

func blend(under, over uint8, a float64) uint8 {
	return uint8(float64(under)*(1-a) + float64(over)*a + 0.5)
}
