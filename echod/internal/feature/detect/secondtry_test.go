package detect

import (
	"testing"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/hardware/mic"
)

// say runs one utterance through judge: it climbs to peak, falls away, and is given time to settle,
// starting at t. It reports whether it fired.
func say(e *Engine, s *slot, peak float64, t time.Time) (fired bool, end time.Time) {
	src := &mic.Source{}
	heard := make(chan struct{}, 4)
	e.OnDetect = func(int) { heard <- struct{}{} }
	at := t
	for _, score := range []float64{0.2, peak, 0.2} {
		e.judge(0, s, score, at, src)
		at = at.Add(80 * time.Millisecond)
	}
	// Past the settle time, so a peak that never fired is counted as a near miss, and past the hold
	// and the refractory time, so one that fired is done.
	at = at.Add(NearMissSettle + Hold + Refractory)
	e.judge(0, s, 0.1, at, src)
	e.judge(0, s, 0.1, at.Add(Refractory), src)
	// OnDetect runs off the audio path, so it is waited for.
	select {
	case <-heard:
		fired = true
	case <-time.After(200 * time.Millisecond):
	}
	return fired, at.Add(Refractory)
}

func secondTryEngine(allowed bool) *Engine {
	e := New(1, nil)
	e.Threshold = func(int) float64 { return 0.8 }
	e.SecondTry = func(int) bool { return allowed }
	return e
}

// A word said again after a near miss is heard at a lower cutoff for a while, and only for a while.
func TestANearMissEasesTheSecondTry(t *testing.T) {
	e := secondTryEngine(true)
	s := &e.slots[0]
	now := time.Now()

	if fired, _ := say(e, s, 0.72, now); fired {
		t.Fatal("0.72 fired against a cutoff of 0.8 with no near miss before it")
	}
	fired, end := say(e, s, 0.74, now.Add(3*time.Second))
	if !fired {
		t.Fatal("the second try at 0.74, three seconds after a near miss at 0.72, was not heard")
	}

	// Used up by the detection: the next one is judged at the usual cutoff.
	if fired, _ := say(e, s, 0.74, end); fired {
		t.Fatal("a second try after the eased one fired, the ease was not used up")
	}
}

func TestTheEaseRunsOut(t *testing.T) {
	e := secondTryEngine(true)
	s := &e.slots[0]
	now := time.Now()

	say(e, s, 0.72, now)
	if fired, _ := say(e, s, 0.74, now.Add(SecondTryWindow+2*time.Second)); fired {
		t.Fatal("a try after the window ran out was still eased")
	}
}

// A near miss far below the cutoff is not somebody who nearly got through.
func TestAFarMissEasesNothing(t *testing.T) {
	e := secondTryEngine(true)
	s := &e.slots[0]
	now := time.Now()

	say(e, s, 0.55, now)
	if fired, _ := say(e, s, 0.74, now.Add(3*time.Second)); fired {
		t.Fatal("a near miss at 0.55 eased the next try against 0.8")
	}
}

// Over the device's own playback, or for a slot that may not have one, there is no second try.
func TestNoSecondTryWhereItIsNotAllowed(t *testing.T) {
	e := secondTryEngine(false)
	s := &e.slots[0]
	now := time.Now()

	say(e, s, 0.72, now)
	if fired, _ := say(e, s, 0.74, now.Add(3*time.Second)); fired {
		t.Fatal("a second try was eased where SecondTry said no")
	}
}

// The ease never goes under the floor, however low the cutoff was set.
func TestTheEaseStopsAtTheFloor(t *testing.T) {
	e := secondTryEngine(true)
	e.Threshold = func(int) float64 { return 0.62 }
	s := &e.slots[0]
	now := time.Now()

	say(e, s, 0.58, now)
	if fired, _ := say(e, s, 0.59, now.Add(3*time.Second)); fired {
		t.Fatalf("a second try at 0.59 fired, under the floor of %v", SecondTryFloor)
	}
}
