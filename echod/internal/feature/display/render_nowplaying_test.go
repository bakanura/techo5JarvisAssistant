//go:build !dot && !spot

package display

import (
	"fmt"
	"image"
	"testing"

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
