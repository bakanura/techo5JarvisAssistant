package mic

import (
	"math"
	"testing"
)

// refBlock is one capture period with a tone on the loopback at dbfs, and nothing on the microphones.
func refBlock(dbfs float64) []byte {
	frameBytes := Channels * Bits / 8
	raw := make([]byte, FrameSamples*frameBytes)
	amp := math.Pow(10, dbfs/20) * 32768 * math.Sqrt2
	for f := range FrameSamples {
		v := int32(amp*math.Sin(2*math.Pi*440*float64(f)/Rate)) << 8
		o := f*frameBytes + RefFirst*3
		raw[o], raw[o+1], raw[o+2] = byte(v), byte(v>>8), byte(v>>16)
	}
	return raw
}

// A loopback too faint to hide a word runs the canceller but does not count as masking; that slack
// let a TV in the room wake the device. A loud one masks, and stops a second after it goes quiet.
func TestOnlyALoudSpeakerMasks(t *testing.T) {
	c := newCanceller()
	if c == nil {
		t.Fatal("no canceller")
	}
	mic := make([]int16, FrameSamples)
	feed := func(dbfs float64, samples int) {
		for range samples / FrameSamples {
			c.apply(refBlock(dbfs), mic)
		}
	}

	feed(-62, Rate/2)
	if !c.active.Load() {
		t.Fatal("the canceller is not running on a -62 dBFS loopback")
	}
	if c.loud.Load() {
		t.Error("a -62 dBFS loopback counts as masking")
	}

	feed(-40, Rate/2)
	if !c.loud.Load() {
		t.Error("a -40 dBFS loopback does not count as masking")
	}

	feed(-120, Rate/2)
	if !c.loud.Load() {
		t.Error("masking ended half a second into a pause")
	}
	feed(-120, Rate)
	if c.loud.Load() {
		t.Error("masking outlasted the hold")
	}
}
