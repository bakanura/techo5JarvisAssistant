package voice

import (
	"testing"

	"github.com/HuskerMinion/techo5/echod/internal/lib/endpoint"
)

// level is a tenth of a second of a steady tone at amplitude a.
func level(a int16) []int16 {
	f := make([]int16, endpoint.Window)
	for i := range f {
		if i%2 == 0 {
			f[i] = a
		} else {
			f[i] = -a
		}
	}
	return f
}

func TestFollowGateHoldsTheRoom(t *testing.T) {
	var g followGate
	for i := 0; i < 50; i++ {
		if got := g.pass(level(300)); got != nil {
			t.Fatalf("window %d: sent %d samples of a quiet room", i, len(got))
		}
	}
}

func TestFollowGateHoldsSteadyMusic(t *testing.T) {
	var g followGate
	for i := 0; i < 50; i++ {
		if got := g.pass(level(3000)); got != nil {
			t.Fatalf("window %d: sent %d samples of steady music", i, len(got))
		}
	}
}

func TestFollowGateSendsTheStartWithTheOnset(t *testing.T) {
	var g followGate
	for i := 0; i < 20; i++ {
		g.pass(level(300))
	}
	if got := g.pass(level(3000)); got != nil {
		t.Fatal("opened on one loud window")
	}
	got := g.pass(level(3000))
	if len(got) != preRoll {
		t.Fatalf("sent %d samples on the onset, want the last %d", len(got), preRoll)
	}
	if got[len(got)-1] != -3000 || got[0] != 300 {
		t.Fatalf("sent %d..%d, want the quiet before and the speech up to now", got[0], got[len(got)-1])
	}
}

func TestBareOkay(t *testing.T) {
	for text, want := range map[string]bool{
		"Okay.":                        true,
		"okay":                         true,
		" OK! ":                        true,
		"Okay…":                        true,
		"Okay, das Licht ist an.":      false,
		"Pausiert":                     false,
		"":                             false,
		"Okay. Was möchtest du hören?": false,
	} {
		if got := bareOkay(text); got != want {
			t.Errorf("bareOkay(%q) = %v, want %v", text, got, want)
		}
	}
}
