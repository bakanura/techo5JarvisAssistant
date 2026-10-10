//go:build !dot && !spot

package display

import (
	"image"
	"time"
)

// The Show's panel is an LCD: it doesn't burn in the way an OLED does, but a clock left in one place
// for months can leave a faint ghost of itself. So the idle clock, and the weather in its corner, go
// round a small ring a few pixels across, one step every shiftEvery: too little and too slow to be
// seen moving, enough that no edge stays on the same pixels. The clock redraws every minute anyway,
// so this costs no frames.
const shiftEvery = 3 * time.Minute

// shiftRing is the ring, in steps of shiftStep; it starts and ends next to where the page was drawn.
var shiftRing = [...]image.Point{{0, 0}, {1, -1}, {2, 0}, {1, 1}, {0, 2}, {-1, 1}, {-2, 0}, {-1, -1}}

const shiftStep = 3

// idleShift is how far the idle page is moved at now.
func (r *renderer) idleShift(now time.Time) image.Point {
	n := now.Unix() / int64(shiftEvery/time.Second)
	p := shiftRing[n%int64(len(shiftRing))]
	return image.Pt(r.s(p.X*shiftStep), r.s(p.Y*shiftStep))
}
