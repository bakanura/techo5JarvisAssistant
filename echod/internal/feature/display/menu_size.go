//go:build !dot && !spot

package display

import (
	"log/slog"
	"sync/atomic"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/HuskerMinion/techo5/echod/internal/config"
)

// The menu size draws the settings screen's card, the drawer and their lists of choices larger, for
// anyone who finds them small at arm's length. The rail of categories keeps its size: on a Show 5 it
// already fills the panel's height, and its names are short.

// menuSizes are the choices, in the order the screen and Home Assistant offer them. Percent 0 is the
// drawn size.
var menuSizes = []struct {
	label   string
	percent int
}{
	{"Normal", 0},
	{"Large", 115},
	{"Larger", 130},
}

// menuZoom is the chosen size in percent, kept apart from the config so a frame reads it without its lock.
var menuZoom atomic.Int32

func init() { menuZoom.Store(100) }

func menuSizeOptions() []string {
	out := make([]string, len(menuSizes))
	for i, m := range menuSizes {
		out[i] = m.label
	}
	return out
}

// menuSizeIndex is the saved choice's place in menuSizes; a value no choice has reads as Normal.
func menuSizeIndex() int {
	v := config.Get().Screen.MenuSize
	for i, m := range menuSizes {
		if m.percent == v {
			return i
		}
	}
	return 0
}

// useMenuSize puts the choice at i in force without saving it.
func useMenuSize(i int) {
	p := menuSizes[i].percent
	if p == 0 {
		p = 100
	}
	menuZoom.Store(int32(p))
}

// setMenuSize saves the choice at i, puts it in force and shows it in Home Assistant.
func setMenuSize(s *esphome.Select, i int) {
	if i < 0 || i >= len(menuSizes) {
		return
	}
	if err := config.Set().Screen().MenuSize(menuSizes[i].percent); err != nil {
		slog.Error("saving the menu size failed", "err", err)
		return
	}
	useMenuSize(i)
	s.Set(menuSizes[i].label)
}

// menuSizeSelect is the Home Assistant setting; wake redraws the screen once it changes.
func menuSizeSelect(wake func()) *esphome.Select {
	s := &esphome.Select{
		Base: esphome.Base{
			ObjectID: "screen_menu_size",
			Name:     "Menu size",
			Icon:     "mdi:magnify-plus-outline",
			Category: esphome.CategoryConfig,
		},
		Options: menuSizeOptions(),
	}
	s.OnCommand = func(v string) {
		for i, m := range menuSizes {
			if m.label == v {
				setMenuSize(s, i)
				wake()
				return
			}
		}
	}
	return s
}

// menuSizeRow is the setting on the screen, under Display.
func menuSizeRow() settingRow {
	return settingRow{id: "menusize", label: "Menu size", sub: "Settings and the drawer, larger", kind: ctlChoice,
		value: menuSizes[menuSizeIndex()].label}
}

func menuSizePicker() (pickerView, bool) {
	return pickerView{title: "Menu size", opts: menuSizeOptions(), cur: menuSizeIndex()}, true
}

func (d *Display) chooseMenuSize(i int) {
	setMenuSize(d.menuSize, i)
	d.wake()
}

// zoomed runs draw with every fixed size, and the settings screen's text, made larger by the menu size.
// Positions taken from the panel's edges stay where they are; what grows is what is measured in the
// Show 5's pixels, which is every size the settings parts draw with.
func (r *renderer) zoomed(draw func()) {
	z := int(menuZoom.Load())
	if z <= 100 {
		draw()
		return
	}
	num, den, fc, more := r.sNum, r.sDen, r.fc, r.moreLines
	defer func() { r.sNum, r.sDen, r.fc, r.moreLines = num, den, fc, more }()
	r.moreLines = 2
	if den == 0 {
		r.sNum, r.sDen = 1, 1
	}
	r.sNum, r.sDen = r.sNum*z, r.sDen*100
	if r.zoomFaces == nil {
		r.zoomFaces = map[int]*sheetFaces{}
	}
	if r.zoomFaces[z] == nil {
		f := sheetFacesAt(r.s)
		r.zoomFaces[z] = &f
	}
	r.fc = r.zoomFaces[z]
	draw()
}
