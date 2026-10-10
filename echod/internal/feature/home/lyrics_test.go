package home

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLyricsPreferTheTimedWords(t *testing.T) {
	// What metadata/get_track_lyrics answers: the plain words and the timed ones. A line sung twice
	// has two times, tags without one are left out, and the empty line at 1s is a pause.
	lrc := "[ar:Someone]\n[ti:Song]\n[00:04.94]First line\n[00:01.00]\n[00:09.00][00:12.50] Second line \n[00:14.00]\n"
	raw, _ := json.Marshal([]any{"First line\nSecond line", lrc})
	l, err := decodeLyrics(raw, "Song")
	if err != nil || l == nil {
		t.Fatalf("decode: %v, %v", l, err)
	}
	if !l.Synced || l.Title != "Song" {
		t.Fatalf("want the timed words for Song, got %+v", l)
	}
	want := []LyricLine{{time.Second, ""}, {4940 * time.Millisecond, "First line"}, {9 * time.Second, "Second line"},
		{12500 * time.Millisecond, "Second line"}}
	if len(l.Lines) != len(want) {
		t.Fatalf("lines %+v, want %+v", l.Lines, want)
	}
	for i := range want {
		if l.Lines[i] != want[i] {
			t.Fatalf("line %d = %+v, want %+v", i, l.Lines[i], want[i])
		}
	}
	for _, c := range []struct {
		at   time.Duration
		cur  int
		next time.Duration
	}{
		{0, -1, time.Second},
		{time.Second, 0, 4940 * time.Millisecond},
		{5 * time.Second, 1, 9 * time.Second},
		{time.Minute, 3, 0},
	} {
		if got := l.Current(c.at); got != c.cur {
			t.Errorf("Current(%v) = %d, want %d", c.at, got, c.cur)
		}
		if got := l.NextAt(c.at); got != c.next {
			t.Errorf("NextAt(%v) = %v, want %v", c.at, got, c.next)
		}
	}
}

func TestLyricsOffsetAndWordTimes(t *testing.T) {
	lrc := "[offset:+500]\r\n[01:02:50]<01:02.50>Every <01:03.10>word\r\n[00:00.20]Early"
	l := parseLRC(lrc, "Song")
	if l == nil || len(l.Lines) != 2 {
		t.Fatalf("got %+v", l)
	}
	if l.Lines[0] != (LyricLine{0, "Early"}) {
		t.Errorf("an offset past the start stops at 0: %+v", l.Lines[0])
	}
	if l.Lines[1] != (LyricLine{62 * time.Second, "Every word"}) {
		t.Errorf("got %+v", l.Lines[1])
	}
}

func TestLyricsPlainWords(t *testing.T) {
	raw, _ := json.Marshal([]any{"\nOnly line\r\nAnd this\n\n", nil})
	l, err := decodeLyrics(raw, "Song")
	if err != nil || l == nil || l.Synced || len(l.Lines) != 2 || l.Lines[0].Text != "Only line" || l.Lines[1].Text != "And this" {
		t.Fatalf("got %+v, %v", l, err)
	}
	if l.Current(time.Minute) != -1 {
		t.Fatal("plain words have no current line")
	}
	// Timed words with nothing timed in them fall back to the plain ones.
	raw, _ = json.Marshal([]any{"Plain", "[ar:Someone]"})
	if l, _ := decodeLyrics(raw, "Song"); l == nil || l.Synced || l.Lines[0].Text != "Plain" {
		t.Fatalf("got %+v", l)
	}
}

func TestLyricsNone(t *testing.T) {
	for _, raw := range []string{`[null, null]`, `[]`, `["", "\n"]`} {
		if l, err := decodeLyrics(json.RawMessage(raw), "Song"); l != nil || err != nil {
			t.Errorf("%s: %+v, %v", raw, l, err)
		}
	}
	if _, err := decodeLyrics(json.RawMessage(`{"error": "x"}`), "Song"); err == nil {
		t.Error("an answer that is not a pair is an error")
	}
}

func TestLRCTime(t *testing.T) {
	for tag, want := range map[string]time.Duration{"00:00": 0, "01:02.5": 62500 * time.Millisecond,
		"10:00:25": 600250 * time.Millisecond} {
		if got, ok := lrcTime(tag); !ok || got != want {
			t.Errorf("%q = %v %v, want %v", tag, got, ok, want)
		}
	}
	for _, tag := range []string{"ar:Someone", "offset:500", "00:61", "-1:00", "00:1e1"} {
		if _, ok := lrcTime(tag); ok {
			t.Errorf("%q is not a time", tag)
		}
	}
}

func TestElapsedMovesOnOnlyWhilePlaying(t *testing.T) {
	at := time.Now()
	v := MusicPlaybackView{Playing: true, Position: 10, Duration: 30, PositionAt: at}
	if got := v.Elapsed(at.Add(5 * time.Second)); got != 15 {
		t.Fatalf("playing: %v", got)
	}
	if got := v.Elapsed(at.Add(time.Minute)); got != 30 {
		t.Fatalf("past the end: %v", got)
	}
	v.Playing, v.Paused = false, true
	if got := v.Elapsed(at.Add(5 * time.Second)); got != 10 {
		t.Fatalf("paused: %v", got)
	}
}
