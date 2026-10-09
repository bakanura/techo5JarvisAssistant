//go:build !dot

package display

import (
	"image"
	"image/color"
	"math"
	"time"
)

// The glow turn screen: a bar of light across the bottom of the picture, and above it a soft glow
// that rises and brightens with the voice, highest in the middle where the low notes are put. While
// it thinks, two bright spots run in from the ends, meet, and part again. Blue running to cyan by
// day, embers at night.
//
// Each column is worked out once (its color, how bright, how high the glow reaches) and the rows
// above and below the bar only look a falloff up in a table, so a frame costs little more than
// filling the band.

var (
	glowDeep      = color.RGBA{24, 70, 255, 255}
	glowBright    = color.RGBA{60, 225, 255, 255}
	glowNightDeep = color.RGBA{110, 12, 8, 255}
	glowNightHot  = color.RGBA{255, 96, 50, 255}
)

// glowFalloff is how much glow is left d of the way up its height, for d in 0..3 in 1024 steps.
var glowFalloff = func() []float32 {
	t := make([]float32, 1025)
	for i := range t {
		d := 3 * float64(i) / 1024
		t[i] = float32(math.Exp(-2.2 * d * d))
	}
	return t
}()

func falloff(d float32) float32 {
	if d >= 3 {
		return 0
	}
	return glowFalloff[int(d*1024/3)]
}

// glowColumns is how bright the light is at u across the band, 0 to 1, and how high its glow
// reaches as a share of the band.
func glowColumn(v *eqView, u, t float64) (bright, high float64) {
	if v.thinking {
		c := 0.5 * math.Abs(math.Cos(t*1.9)) // the two spots, from the ends to the middle and back
		g := func(x float64) float64 { return math.Exp(-x * x / 0.006) }
		s := g(u-0.5+c) + g(u-0.5-c)
		return 0.35 + 0.65*min(1, s), 0.06 + 0.2*min(1, s)
	}
	// The bands from the middle out: the voice's low notes, its loudest, in the middle.
	n := len(v.level)
	p := math.Abs(u-0.5) * 2 * float64(n-1) * 0.85
	i := min(int(p), n-2)
	e := v.level[i] + (v.level[i+1]-v.level[i])*(p-float64(i))
	breathe := 0.04 * math.Sin(t*2.5) // alive even when it hears nothing yet
	return 0.3 + 0.7*e + breathe, 0.05 + 0.95*e + breathe
}

// drawGlow draws the glow into band on dst, over ground, at now: the bar along the band's bottom and
// the glow rising from it up to the band's top, with a faint reflection below it.
func drawGlow(dst *image.RGBA, band image.Rectangle, v *eqView, ground color.RGBA, now time.Time) {
	deep, hot := glowDeep, glowBright
	if v.night {
		deep, hot = glowNightDeep, glowNightHot
	}
	t := float64(now.UnixMilli()%(1<<40)) / 1000
	x0, x1 := max(band.Min.X, dst.Rect.Min.X), min(band.Max.X, dst.Rect.Max.X)
	w := float64(band.Dx())
	thick := max(3, band.Dy()/28)
	barTop := band.Max.Y - thick
	reach := float64(barTop - band.Min.Y)
	below := min(band.Dy()/6, dst.Rect.Max.Y-band.Max.Y)
	g := [3]float32{float32(ground.R), float32(ground.G), float32(ground.B)}

	put := func(x, y int, c [3]float32, a float32) {
		if y < dst.Rect.Min.Y || y >= dst.Rect.Max.Y || a <= 0 {
			return
		}
		o := dst.PixOffset(x, y)
		p := dst.Pix[o : o+4 : o+4]
		for k := range 3 {
			p[k] = uint8(min(g[k]+c[k]*a, 255))
		}
		p[3] = 255
	}

	for x := x0; x < x1; x++ {
		u := (float64(x-band.Min.X) + 0.5) / w
		bright, high := glowColumn(v, u, t)
		bright, high = min(max(bright, 0), 1), min(max(high, 0), 1)
		// The bar's ends round off rather than stopping square.
		end := min(1, u/0.06, (1-u)/0.06)
		end = end * end * (3 - 2*end)
		col := mix(deep, hot, bright)
		c := [3]float32{float32(col.R), float32(col.G), float32(col.B)}
		core := mix(col, color.RGBA{255, 255, 255, 255}, 0.55*bright)
		cc := [3]float32{float32(core.R), float32(core.G), float32(core.B)}

		// The bar: brightest along its middle line.
		for y := barTop; y < band.Max.Y; y++ {
			d := math.Abs(float64(y-barTop)+0.5-float64(thick)/2) / (float64(thick) / 2)
			put(x, y, cc, float32((1-0.5*d)*(0.55+0.45*bright)*end))
		}
		// The glow above it.
		h := float32(max(high*reach, 2))
		a := float32(0.85 * bright * end)
		for y := barTop - 1; y >= band.Min.Y; y-- {
			f := falloff(float32(barTop-y) / h)
			if f < 0.004 {
				break
			}
			put(x, y, c, a*f)
		}
		// The reflection under it.
		for y := band.Max.Y; y < band.Max.Y+below; y++ {
			f := falloff(float32(y-band.Max.Y) / (h / 5))
			if f < 0.004 {
				break
			}
			put(x, y, c, 0.25*a*f)
		}
	}
}
