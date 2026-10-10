//go:build !dot && !spot

package display

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/feature/phone"
	"github.com/HuskerMinion/techo5/echod/internal/i18n"
)

// At every menu size, on both panels, every card, every list of choices on it and every tab of the
// drawer draws with all it can be tapped on inside the panel, and no two choices on top of each other.
// With SHOW_PREVIEW set, the frames are written there, under menusize-<percent>-.
func TestEveryMenuSizeFits(t *testing.T) {
	t.Cleanup(func() { menuZoom.Store(100) })
	at := time.Date(2026, 9, 18, 15, 0, 0, 0, time.Local)
	dir := os.Getenv("SHOW_PREVIEW")

	scenes := map[string]scene{}
	for c := category(0); c < categories; c++ {
		s := sheetScene(c)
		s.showSheet = true
		scenes[categoryNames[c]] = s
		end := s
		end.sheet.cardScroll = 1 << 20 // held to the end
		mid := s
		mid.sheet.cardScroll = 260
		scenes[categoryNames[c]+"-mid"] = mid
		scenes[categoryNames[c]+"-end"] = end
		for _, row := range categoryCard(s.view()).rows {
			if row.kind != ctlChoice || row.id == "" {
				continue
			}
			p := s
			p.sheet.picker = row.id
			if _, ok := pickerFor(row.id, p.view()); ok {
				scenes[categoryNames[c]+"-"+row.id] = p
			}
		}
	}
	for i, name := range drawerTabs {
		scenes["drawer-"+name] = scene{now: at, phase: "idle", showDrawer: true, drawerTab: i, demo: true,
			announceReady: true, announcePeers: 3,
			callees: []phone.Callee{{Name: "a", Device: true}, {Name: "b", Device: true}, {Name: "d", Number: "15551234567"}}}
	}

	config.Use(filepath.Join(t.TempDir(), "state.json"))
	t.Cleanup(i18n.Changed)
	for _, lang := range []string{"", "de"} {
		if err := config.Set().Screen().Language(lang); err != nil {
			t.Fatal(err)
		}
		i18n.Changed()
		for _, m := range menuSizes {
			useMenuSize(menuSizeIndexOf(m.percent))
			// No row is cut at this size either: it wraps, as it does at the drawn size.
			for c := category(0); c < categories; c++ {
				r := testRenderer()
				card := r.nextCard(r.w, r.h) // where the rail leaves it, as settingsPage has it
				r.zoomed(func() {
					fc := r.faces()
					for _, row := range categoryCard(sheetScene(c).view()).rows {
						label, sub, _ := r.rowWords(card, row, fc.label)
						for _, l := range append(label, sub...) {
							if strings.HasSuffix(l, "…") {
								t.Errorf("%s %q at %d%%: %q is cut: %q %q", categoryNames[c], lang, menuZoom.Load(), row.label, label, sub)
							}
						}
					}
				})
			}
			for _, panel := range []struct {
				name       string
				wide, high int
			}{{"", showWide, showHigh}, {"-show8", show8Wide, show8High}} {
				for name, s := range scenes {
					what := fmt.Sprintf("%s %q at %d%%%s", name, lang, menuZoom.Load(), panel.name)
					img := image.NewRGBA(image.Rect(0, 0, panel.wide, panel.high))
					r := newRenderer(img)
					r.draw(s)
					var opts []image.Rectangle
					for _, z := range r.zones {
						if z.r.Empty() || !z.r.In(img.Rect) {
							t.Errorf("%s: zone %+v is off the panel or empty", what, z)
						}
						if z.kind == zoneOption {
							opts = append(opts, z.r)
						}
					}
					if s.sheet.picker != "" && len(opts) == 0 {
						t.Errorf("%s: the list of choices has nothing to tap", what)
					}
					for i := range opts {
						for j := i + 1; j < len(opts); j++ {
							if opts[i].Inset(4).Overlaps(opts[j].Inset(4)) {
								t.Errorf("%s: choices %d and %d overlap", what, i, j)
							}
						}
					}
					if dir != "" {
						writePNG(t, filepath.Join(dir, fmt.Sprintf("menusize-%d-%s%s%s.png", menuZoom.Load(), name, panel.name, map[string]string{"de": "-de"}[lang])), img)
					}
				}
			}
		}
	}
}

func menuSizeIndexOf(percent int) int {
	for i, m := range menuSizes {
		if m.percent == percent {
			return i
		}
	}
	return 0
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
