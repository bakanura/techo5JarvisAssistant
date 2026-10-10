package home

import (
	"testing"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/feature/media"
)

// A tap on play or pause shows at once, before Music Assistant has heard of it, and the toggle is settled
// against what the screen showed, so two quick taps are a pause and then a play rather than whatever the
// server had not caught up with. Next and back start the bar from the beginning.
func TestMusicTapShowsAtOnce(t *testing.T) {
	musicTapsOnce.Do(func() {}) // no worker: the taps stay queued here, and nothing calls Home Assistant
	t.Cleanup(func() {
		musicPlayback.Lock()
		musicPlayback.view, musicPlayback.expectUntil = MusicPlaybackView{}, time.Time{}
		musicPlayback.Unlock()
	})
	musicPlayback.Lock()
	musicPlayback.view = MusicPlaybackView{Entity: "media_player.example", Playing: true, Position: 30, Duration: 200, PositionAt: time.Now()}
	musicPlayback.Unlock()

	f := &Feature{}
	want := []media.Transport{media.TransportPause, media.TransportPlay, media.TransportNext}
	f.MusicTap(media.TransportToggle)
	if v := f.MusicPlayback(); v.Playing || !v.Paused {
		t.Fatalf("after one tap the page shows playing=%v paused=%v, want paused", v.Playing, v.Paused)
	}
	f.MusicTap(media.TransportToggle)
	if v := f.MusicPlayback(); !v.Playing || v.Paused {
		t.Fatalf("after two taps the page shows playing=%v paused=%v, want playing", v.Playing, v.Paused)
	}
	f.MusicTap(media.TransportNext)
	if v := f.MusicPlayback(); v.Elapsed(time.Now()) > 1 {
		t.Errorf("after next the bar is at %.1fs, want the start", v.Elapsed(time.Now()))
	}
	for i, w := range want {
		select {
		case got := <-musicTaps:
			if got != w {
				t.Errorf("tap %d went to Music Assistant as %v, want %v", i, got, w)
			}
		default:
			t.Fatalf("tap %d never queued", i)
		}
	}
}

// What a tap asked for stays on the screen while Home Assistant still reports the state from before it,
// and gives way once it agrees or the wait is over.
func TestHoldExpected(t *testing.T) {
	now := time.Now()
	was := MusicPlaybackView{Paused: true, Position: 42, PositionAt: now.Add(-time.Second)}
	for _, c := range []struct {
		name       string
		ha         MusicPlaybackView
		until      time.Time
		wantPaused bool
		wantKept   bool
	}{
		{"behind", MusicPlaybackView{Playing: true, Position: 50, PositionAt: now}, now.Add(time.Second), true, true},
		{"caught up", MusicPlaybackView{Paused: true, Position: 43, PositionAt: now}, now.Add(time.Second), true, false},
		{"too long", MusicPlaybackView{Playing: true, Position: 50, PositionAt: now}, now.Add(-time.Millisecond), false, false},
		{"stopped", MusicPlaybackView{}, now.Add(time.Second), false, false},
	} {
		v, playing, until := c.ha, false, c.until
		holdExpected(&v, was, &playing, &until, now)
		if v.Paused != c.wantPaused {
			t.Errorf("%s: paused=%v, want %v", c.name, v.Paused, c.wantPaused)
		}
		if kept := !until.IsZero(); kept != c.wantKept {
			t.Errorf("%s: still waiting=%v, want %v", c.name, kept, c.wantKept)
		}
		if c.wantKept && v.Position != was.Position {
			t.Errorf("%s: position %.0f, want the screen's %.0f", c.name, v.Position, was.Position)
		}
	}
}
