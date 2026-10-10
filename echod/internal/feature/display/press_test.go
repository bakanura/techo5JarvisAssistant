//go:build !dot && !spot

package display

import (
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/feature/home"
)

// A pressed music button is drawn lighter than the same button unpressed, on the music page and on the
// strip, and nowhere else on the panel changes: the light is the button's own.
func TestAPressedMusicButtonLights(t *testing.T) {
	at := time.Date(2026, 9, 18, 15, 0, 0, 0, time.Local)
	r := testRenderer()
	_, back, play, next, _ := r.nowPlayingButtons()
	sback, splay, snext, _ := r.stripButtons()
	page := scene{now: at, phase: "idle", nowPlaying: true, playing: true,
		radio: home.Radio{Playing: true, Now: "Music Assistant", Title: "Some Jazz", Artist: "The Quartet", Music: true}}
	strip := scene{now: at, phase: "idle", strip: true, playing: true,
		radio: home.Radio{Playing: true, Now: "KXYZ 101.1", Title: "Take It Easy", Artist: "Eagles"}}
	for _, c := range []struct {
		name string
		s    scene
		b    image.Rectangle
	}{{"page-back", page, back}, {"page-play", page, play}, {"page-next", page, next},
		{"strip-back", strip, sback}, {"strip-play", strip, splay}, {"strip-next", strip, snext}} {
		plain := frame(c.s)
		c.s.pressed = c.b
		lit := frame(c.s)
		if dir := os.Getenv("SHOW_PREVIEW"); dir != "" {
			writePNG(t, filepath.Join(dir, "pressed-"+c.name+".png"), lit)
		}
		var brighter, outside int
		for y := 0; y < plain.Bounds().Dy(); y++ {
			for x := 0; x < plain.Bounds().Dx(); x++ {
				a, b := plain.RGBAAt(x, y), lit.RGBAAt(x, y)
				if a == b {
					continue
				}
				if !image.Pt(x, y).In(c.b) {
					outside++
				} else if luma(b.R, b.G, b.B) > luma(a.R, a.G, a.B) {
					brighter++
				}
			}
		}
		if brighter < c.b.Dx()*c.b.Dy()/3 {
			t.Errorf("%s: only %d pixels of the pressed button %v got lighter", c.name, brighter, c.b)
		}
		if outside > 0 {
			t.Errorf("%s: %d pixels outside the pressed button changed", c.name, outside)
		}
	}
}

func frame(s scene) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, showWide, showHigh))
	newRenderer(img).draw(s)
	return img
}
