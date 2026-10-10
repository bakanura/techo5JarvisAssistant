//go:build !dot && !spot

package display

import (
	"fmt"
	"image"
	"testing"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/feature/home"
)

// The now-playing buttons are drawn from the panel's own size, and their rectangles are the tap regions
// as well, so an overlap is a button that cannot be pressed and an overflow is one that cannot be seen.
// The Show 8 is why this is a table: the page is one file for both panels, and the cover beside the
// column is a different size on each.
func TestTheNowPlayingButtonsFitTheirPanel(t *testing.T) {
	for _, panel := range []image.Point{{X: 960, Y: 480}, {X: 1280, Y: 800}} {
		t.Run(fmt.Sprintf("%dx%d", panel.X, panel.Y), func(t *testing.T) {
			screen := image.Rect(0, 0, panel.X, panel.Y)
			r := newRenderer(image.NewRGBA(screen))
			cover, col := r.nowPlayingLayout()
			fav, back, play, next, stop := r.nowPlayingButtons()
			row := []image.Rectangle{fav, back, play, next, stop}

			if !cover.In(screen) || !col.In(screen) || cover.Overlaps(col) {
				t.Errorf("cover %v and column %v do not sit side by side on the panel", cover, col)
			}
			if cover.Dx() != cover.Dy() {
				t.Errorf("the cover's place %v is not square", cover)
			}
			if col.Dx() < r.s(440) {
				t.Errorf("the column is %d wide, narrower than the %d its row needs", col.Dx(), r.s(440))
			}
			for i, b := range row {
				if !b.In(col) {
					t.Errorf("button %d %v is outside the column %v", i, b, col)
				}
				if b.Min.Y != play.Min.Y || b.Dy() != play.Dy() {
					t.Errorf("button %d %v is not in one row with play %v", i, b, play)
				}
				if i > 0 && row[i-1].Max.X > b.Min.X {
					t.Errorf("buttons %d %v and %d %v overlap or are out of order", i-1, row[i-1], i, b)
				}
			}
			if left, right := back.Min.X-col.Min.X, col.Max.X-next.Max.X; left-right > 1 || right-left > 1 {
				t.Errorf("the transport is %d px from the column's left and %d from its right", left, right)
			}
			if play.Max.Y != cover.Max.Y {
				t.Errorf("the row ends at %d, not level with the cover's foot at %d", play.Max.Y, cover.Max.Y)
			}
			// The Lyrics pill sits in the head of the column, left of the clock, clear of the row.
			now := time.Date(2026, 10, 10, 22, 58, 0, 0, time.UTC)
			words := r.lyricsButton(now)
			clock := col.Max.X - r.width(r.small, clockText(now))
			if !words.In(screen) || words.Min.X < col.Min.X+col.Dx()/3 || words.Max.X > clock-r.s(8) {
				t.Errorf("the Lyrics pill %v is not between the place and the clock at %d in %v", words, clock, col)
			}
			if words.Inset(-r.s(8)).Overlaps(fav) || words.Max.Y > col.Min.Y+r.s(60) {
				t.Errorf("the Lyrics pill %v is not in the head of the column %v", words, col)
			}
			if footer := panel.Y - r.s(26); play.Max.Y > footer {
				t.Errorf("the buttons reach %d, into the footer at %d", play.Max.Y, footer)
			}
		})
	}
}

func TestMusicPlaceReportsTheResolvedDestination(t *testing.T) {
	for _, tc := range []struct {
		name string
		view home.MusicPlaybackView
		want string
	}{
		{
			name: "Jarvis fallback endpoint",
			view: home.MusicPlaybackView{Entity: "media_player.living_room_jarvis", Output: "Living Room Jarvis"},
			want: "Living Room Jarvis",
		},
		{
			name: "single resolved room",
			view: home.MusicPlaybackView{Entity: "media_player.living_room_jarvis", Output: "Living Room Jarvis", Rooms: []string{"living"}},
			want: "Living",
		},
		{
			name: "named group",
			view: home.MusicPlaybackView{Route: "wohnung", Rooms: []string{"living", "bedroom"}},
			want: "Wohnung",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := musicPlace(tc.view); got != tc.want {
				t.Fatalf("musicPlace() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSongParts(t *testing.T) {
	for _, c := range []struct{ title, song, version string }{
		{"My Love Is Like...Wo (All Star Mix – Main Pass)", "My Love Is Like...Wo", "All Star Mix – Main Pass"},
		{"Bohemian Rhapsody - Remastered 2011", "Bohemian Rhapsody", "Remastered 2011"},
		{"Song (feat. Somebody) [Radio Edit]", "Song", "feat. Somebody  ·  Radio Edit"},
		{"Symphony No. 5 (Part 2)", "Symphony No. 5 (Part 2)", ""},
		{"(Remix)", "(Remix)", ""},
		{"Reason That I Sing", "Reason That I Sing", ""},
	} {
		song, version := songParts(c.title)
		if song != c.song || version != c.version {
			t.Errorf("songParts(%q) = %q, %q; want %q, %q", c.title, song, version, c.song, c.version)
		}
	}
}
