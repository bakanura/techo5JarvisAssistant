package endpoint

import "math"

// Onset decides when somebody starts talking into a turn that opened on nothing: a follow-up, where
// the microphone comes back on after a reply without anyone having said a word.
//
// Sending that silence to Home Assistant goes wrong two ways. Its detector takes the tail of the reply
// or a cough for speech and closes the turn on the quiet after it, a second or two in, before anyone
// has thought of an answer. And speech to text handed a few seconds of an empty room makes words up
// (Whisper likes "Musik"), which the agent then acts on. Music in the room is worse still, because the
// words in it are real.
//
// So a follow-up holds its audio until a window stands well clear of the room. The room is the level
// the follow-up has been hearing: it drops at once to anything quieter and creeps up to anything
// louder, so a steady television or song becomes the room while somebody speaking up close does not.
type Onset struct {
	sum float64 // squares in the window being filled
	n   int

	floor float64 // the room, as heard so far
	loud  int     // windows in a row clear of it
	open  bool
}

const (
	onsetOver    = 3.0  // how many times the room's level counts as somebody talking (about 10 dB)
	onsetMin     = 400  // and never quieter than this, which is an empty room after the leveler
	onsetWindows = 2    // in a row: a door or a clap is one
	onsetRise    = 0.05 // how much of the way to a louder window the room moves each window
)

// Feed takes samples and reports whether somebody has started talking. Once it has said so it goes on
// saying so.
func (o *Onset) Feed(samples []int16) bool {
	for _, v := range samples {
		if o.open {
			return true
		}
		o.sum += float64(v) * float64(v)
		o.n++
		if o.n == Window {
			o.window(math.Sqrt(o.sum / Window))
			o.sum, o.n = 0, 0
		}
	}
	return o.open
}

func (o *Onset) window(rms float64) {
	if o.floor == 0 {
		o.floor = max(rms, 1)
		return
	}
	if rms >= onsetOver*o.floor && rms >= onsetMin {
		o.loud++
		o.open = o.loud >= onsetWindows
		return
	}
	o.loud = 0
	if rms < o.floor {
		o.floor = max(rms, 1)
	} else {
		o.floor += (rms - o.floor) * onsetRise
	}
}
