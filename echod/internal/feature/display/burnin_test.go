//go:build !dot && !spot

package display

import (
	"testing"
	"time"
)

// The idle page moves a step at a time, never more than two steps from where it is drawn, and comes
// back round to it.
func TestIdleShiftGoesRoundSlowly(t *testing.T) {
	r := testRenderer()
	at := time.Date(2026, 10, 10, 3, 0, 0, 0, time.UTC)
	most, step := r.s(2*shiftStep), r.s(shiftStep)
	seen := map[[2]int]bool{}
	prev := r.idleShift(at)
	for i := 1; i <= 2*len(shiftRing); i++ {
		p := r.idleShift(at.Add(time.Duration(i) * shiftEvery))
		if abs(p.X) > most || abs(p.Y) > most {
			t.Fatalf("step %d moved %v, more than %d", i, p, most)
		}
		if abs(p.X-prev.X) > step || abs(p.Y-prev.Y) > step {
			t.Errorf("step %d jumped from %v to %v", i, prev, p)
		}
		if r.idleShift(at.Add(time.Duration(i)*shiftEvery+shiftEvery-time.Second)) != p {
			t.Errorf("step %d didn't hold for %v", i, shiftEvery)
		}
		seen[[2]int{p.X, p.Y}] = true
		prev = p
	}
	if len(seen) != len(shiftRing) {
		t.Errorf("visited %d places, want %d", len(seen), len(shiftRing))
	}
}
