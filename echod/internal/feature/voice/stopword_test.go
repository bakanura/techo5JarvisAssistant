package voice

import "testing"

func TestStopMeans(t *testing.T) {
	for _, c := range []struct {
		name                                         string
		listening, followUp, ringing, playing, sound bool
		want                                         stopMeaning
	}{
		{"idle and silent", false, false, false, false, false, stopNothing},
		{"after the wake word, nothing playing", true, false, false, false, false, stopNothing},
		{"after the wake word over music", true, false, false, true, false, stopMusic},
		{"follow-up, nothing playing", true, true, false, false, false, stopConversation},
		{"follow-up over music", true, true, false, true, false, stopConversation},
		{"follow-up while a timer rings", true, true, true, false, false, stopSound},
		{"music, no turn", false, false, false, true, false, stopSound},
		{"an announcement", false, false, false, false, true, stopSound},
		{"a timer ringing over a turn", true, false, true, true, false, stopSound},
	} {
		if got := stopMeans(c.listening, c.followUp, c.ringing, c.playing, c.sound); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}
