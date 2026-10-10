package home

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLyricsPreferTheTimedWords(t *testing.T) {
	// What rest_command hands back for an OpenSubsonic getLyricsBySongId: the server's status and its
	// already decoded JSON body, here with a plain set ahead of a timed one.
	raw := json.RawMessage(`{"status": 200, "headers": {}, "content": {"subsonic-response": {"status": "ok",
		"lyricsList": {"structuredLyrics": [
			{"lang": "xxx", "synced": false, "line": [{"value": "First line"}, {"value": "Second line"}]},
			{"lang": "xxx", "synced": true, "offset": 0, "line": [
				{"start": 4940, "value": "First line"}, {"start": 1000, "value": ""},
				{"start": 9000, "value": " Second line "}, {"start": 12000, "value": ""}]}
		]}}}}`)
	l, err := decodeLyrics(raw, "Song")
	if err != nil || l == nil {
		t.Fatalf("decode: %v, %v", l, err)
	}
	if !l.Synced || l.Title != "Song" {
		t.Fatalf("want the timed set for Song, got %+v", l)
	}
	want := []LyricLine{{time.Second, ""}, {4940 * time.Millisecond, "First line"}, {9 * time.Second, "Second line"}}
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
		{time.Minute, 2, 0},
	} {
		if got := l.Current(c.at); got != c.cur {
			t.Errorf("Current(%v) = %d, want %d", c.at, got, c.cur)
		}
		if got := l.NextAt(c.at); got != c.next {
			t.Errorf("NextAt(%v) = %v, want %v", c.at, got, c.next)
		}
	}
}

func TestLyricsReadATextBodyAndPlainWords(t *testing.T) {
	// Home Assistant leaves a body it did not recognise as JSON as text.
	body := `{"subsonic-response": {"status": "ok", "lyricsList": {"structuredLyrics": [
		{"synced": false, "line": [{"value": "Only line"}]}]}}}`
	b, _ := json.Marshal(map[string]any{"status": 200, "content": body})
	l, err := decodeLyrics(b, "Song")
	if err != nil || l == nil || l.Synced || len(l.Lines) != 1 || l.Lines[0].Text != "Only line" {
		t.Fatalf("got %+v, %v", l, err)
	}
	if l.Current(time.Minute) != -1 {
		t.Fatal("plain words have no current line")
	}
}

func TestLyricsNoneOrRefused(t *testing.T) {
	none := json.RawMessage(`{"status": 200, "content": {"subsonic-response": {"status": "ok", "lyricsList": {}}}}`)
	if l, err := decodeLyrics(none, "Song"); l != nil || err != nil {
		t.Fatalf("a song without words: %+v, %v", l, err)
	}
	for _, raw := range []string{
		`{"status": 401, "content": "Unauthorized"}`,
		`{"status": 200, "content": {"subsonic-response": {"status": "failed", "error": {"code": 40, "message": "Wrong username or password"}}}}`,
	} {
		if _, err := decodeLyrics(json.RawMessage(raw), "Song"); err == nil {
			t.Errorf("%s: want an error", raw)
		}
	}
}

func TestLyricsOnlyForSongsFromTheMusicServer(t *testing.T) {
	if (queueSong{Provider: "opensubsonic--5nVM", ID: "abc"}).subsonic() != true {
		t.Fatal("an OpenSubsonic song has lyrics to ask for")
	}
	for _, q := range []queueSong{{Provider: "radiobrowser", ID: "abc"}, {Provider: "opensubsonic--x"}} {
		if q.subsonic() {
			t.Fatalf("%+v has nothing to ask the music server for", q)
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
