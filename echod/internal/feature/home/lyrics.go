package home

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/lib/hass"
)

// Lyrics for the song Music Assistant is playing, when the song comes from the house's own music
// server. Most of a library ripped and tagged at home carries its words in the files, often with the
// time of every line, and an OpenSubsonic server (Navidrome) hands them out. The Show's network does
// not reach that server, Home Assistant's does: a rest_command named jarvis_lyrics in Home Assistant
// asks the server for a song's lyrics by its id (docs/lyrics.md). Nothing here leaves the house, and
// without that command there are simply no lyrics.

const (
	lyricsDomain  = "rest_command"
	lyricsService = "jarvis_lyrics"
	// lyricsRetry is how long a Home Assistant without the command, or a server that did not answer,
	// is left alone before it is asked again.
	lyricsRetry = 10 * time.Minute
)

// LyricLine is one line of a song's words; At is when it is sung, from the start of the song.
type LyricLine struct {
	At   time.Duration
	Text string
}

// Lyrics are one song's words. Title is the song they belong to, so a page that has moved on to the
// next song does not show the last one's words while these are fetched. Without Synced the lines
// have no times, only an order.
type Lyrics struct {
	Title  string
	Synced bool
	Lines  []LyricLine
}

// Current is the line being sung at elapsed, -1 before the first. Unsynced lyrics have no current
// line.
func (l *Lyrics) Current(elapsed time.Duration) int {
	if l == nil || !l.Synced {
		return -1
	}
	return sort.Search(len(l.Lines), func(i int) bool { return l.Lines[i].At > elapsed }) - 1
}

// NextAt is when the line after the one sung at elapsed starts, zero when there is none.
func (l *Lyrics) NextAt(elapsed time.Duration) time.Duration {
	if l == nil || !l.Synced {
		return 0
	}
	if i := l.Current(elapsed) + 1; i < len(l.Lines) {
		return l.Lines[i].At
	}
	return 0
}

// queueSong is what the queue says about the song playing: its name and, for a song from an
// OpenSubsonic server, its id there.
type queueSong struct {
	Name     string
	Provider string
	ID       string
}

func (q queueSong) subsonic() bool {
	return q.ID != "" && strings.HasPrefix(q.Provider, "opensubsonic")
}

var lyrics struct {
	mu      sync.Mutex
	song    string // the server's id of the song asked for last
	current *Lyrics
	resting time.Time // not asked again before this
}

// MusicLyrics is the words of the song playing, nil when there are none. It does no network I/O.
func (f *Feature) MusicLyrics() *Lyrics {
	lyrics.mu.Lock()
	defer lyrics.mu.Unlock()
	return lyrics.current
}

// refreshLyrics fetches the words for song when it is not the song they were fetched for already.
func refreshLyrics(song queueSong) {
	lyrics.mu.Lock()
	if !song.subsonic() {
		changed := lyrics.current != nil
		lyrics.song, lyrics.current = "", nil
		lyrics.mu.Unlock()
		if changed {
			Get().Changed.Emit(struct{}{})
		}
		return
	}
	if song.ID == lyrics.song || time.Now().Before(lyrics.resting) {
		lyrics.mu.Unlock()
		return
	}
	lyrics.song, lyrics.current = song.ID, nil
	lyrics.mu.Unlock()

	got, err := fetchLyrics(song)
	lyrics.mu.Lock()
	if lyrics.song != song.ID {
		lyrics.mu.Unlock()
		return
	}
	if err != nil {
		// Most often Home Assistant has no jarvis_lyrics at all, which is the normal state of a house
		// that has not set it up; once in a while is often enough to notice that it has.
		lyrics.song, lyrics.resting = "", time.Now().Add(lyricsRetry)
		lyrics.mu.Unlock()
		slog.Info("lyrics", "song", song.Name, "err", err)
		return
	}
	lyrics.current = got
	lyrics.mu.Unlock()
	if got != nil {
		slog.Info("lyrics", "song", song.Name, "lines", len(got.Lines), "synced", got.Synced)
	}
	Get().Changed.Emit(struct{}{})
}

func fetchLyrics(song queueSong) (*Lyrics, error) {
	raw, err := hass.Get().CallResponse(lyricsDomain, lyricsService, map[string]any{"id": song.ID})
	if err != nil {
		return nil, err
	}
	return decodeLyrics(raw, song.Name)
}

// decodeLyrics reads rest_command's answer: the server's HTTP status, and its body, which Home
// Assistant has already decoded when the server said it was JSON and left as text when not.
func decodeLyrics(raw json.RawMessage, title string) (*Lyrics, error) {
	var reply struct {
		Status  int             `json:"status"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &reply); err != nil {
		return nil, err
	}
	if reply.Status/100 != 2 {
		return nil, fmt.Errorf("the music server answered HTTP %d", reply.Status)
	}
	body := reply.Content
	var text string
	if json.Unmarshal(body, &text) == nil {
		body = json.RawMessage(text)
	}
	var doc struct {
		Response struct {
			Status string `json:"status"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
			LyricsList struct {
				Structured []struct {
					Synced bool `json:"synced"`
					Offset int  `json:"offset"`
					Line   []struct {
						Start *int   `json:"start"`
						Value string `json:"value"`
					} `json:"line"`
				} `json:"structuredLyrics"`
			} `json:"lyricsList"`
		} `json:"subsonic-response"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	if doc.Response.Status != "ok" {
		msg := "failed"
		if doc.Response.Error != nil {
			msg = doc.Response.Error.Message
		}
		return nil, errors.New("the music server said: " + msg)
	}
	// A file can carry its words more than once, timed and plain, or in more than one language. The
	// timed ones are worth more on a screen that follows the song.
	sets := doc.Response.LyricsList.Structured
	sort.SliceStable(sets, func(i, j int) bool { return sets[i].Synced && !sets[j].Synced })
	for _, set := range sets {
		l := &Lyrics{Title: title, Synced: set.Synced}
		for _, line := range set.Line {
			at := time.Duration(0)
			if set.Synced {
				if line.Start == nil {
					continue
				}
				at = time.Duration(*line.Start-set.Offset) * time.Millisecond
			}
			l.Lines = append(l.Lines, LyricLine{At: at, Text: strings.TrimSpace(line.Value)})
		}
		for len(l.Lines) > 0 && l.Lines[len(l.Lines)-1].Text == "" {
			l.Lines = l.Lines[:len(l.Lines)-1]
		}
		if len(l.Lines) == 0 {
			continue
		}
		if l.Synced {
			sort.SliceStable(l.Lines, func(i, j int) bool { return l.Lines[i].At < l.Lines[j].At })
		}
		return l, nil
	}
	return nil, nil
}
