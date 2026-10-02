package media

import (
	"testing"

	"github.com/HuskerMinion/techo5/echod/internal/lib/asp"
)

func TestClampToneUsesEntitySafetyRange(t *testing.T) {
	got := clampTone(asp.Tone{Bass: 99, Treble: -99})
	if got.Bass != asp.ToneRange || got.Treble != -asp.ToneRange {
		t.Fatalf("clampTone = %+v, want +/- %g dB", got, asp.ToneRange)
	}
}
