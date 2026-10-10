//go:build !dot && !spot

package display

import (
	"image"
	"image/color"
	"testing"
)

// inked is whether anything was drawn in columns [x0, x1) of img.
func inked(img *image.RGBA, x0, x1 int) bool {
	for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
		for x := x0; x < x1; x++ {
			if img.RGBAAt(x, y).A != 0 {
				return true
			}
		}
	}
	return false
}

const longProblem = "Streaming needs a dashcast server: set one with the dashboard_server action, " +
	"or pick the drawn dashboard in the settings instead."

func TestCentredTextNeverLeavesThePanel(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 960, 480))
	r := newRenderer(img)
	if r.width(r.title, longProblem) <= r.w {
		t.Fatal("the test line has to be wider than the panel")
	}
	r.text(r.title, longProblem, (r.w-r.width(r.title, longProblem))/2, r.h/2, color.White)
	edge := r.s(textEdge)
	if inked(img, 0, edge-1) || inked(img, r.w-edge+1, r.w) {
		t.Error("a centred line too wide for the panel was drawn into its edges")
	}
	if !inked(img, edge, r.w/2) {
		t.Error("nothing of the line was drawn")
	}
}

func TestMessageWrapsInsideTheMargins(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 960, 480))
	r := newRenderer(img)
	r.message(r.small, longProblem, r.margin, r.h/2, color.White)
	if inked(img, 0, r.margin-1) || inked(img, r.w-r.margin+1, r.w) {
		t.Error("the message was drawn outside its margins")
	}
	if lines := r.wrapLines(r.small, longProblem, r.w-2*r.margin); len(lines) < 2 {
		t.Fatalf("expected the message to need several lines, got %d", len(lines))
	}
}
