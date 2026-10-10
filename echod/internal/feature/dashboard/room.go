//go:build !dot

package dashboard

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/layout"
	"github.com/HuskerMinion/techo5/echod/internal/lib/hass"
)

// The room's dashboard: the one for the room the device stands in, a swipe on from the Show's own.
// It is found, never made: a dashboard named after the device's area in Home Assistant, by its title
// or its address, or else a view with the room's name in some other dashboard. A house without one
// has no room page, and the swipe takes the dashboard away as it always did.

// roomWait is how long Home Assistant is given to say which area the device is in.
const roomWait = 10 * time.Second

// RoomPath is the room's dashboard as a page for Stream ("/dashboard-kitchen"), empty when there is
// none. It is streamed whatever the dashboard's mode, so it needs a dashcast server.
func (f *Feature) RoomPath() string {
	d := config.Get().Dashboard
	if d.Server == "" || d.Mode == config.DashboardOff {
		return ""
	}
	p := d.Room
	switch p {
	case config.RoomNone:
		return ""
	case "":
		f.mu.Lock()
		p = f.room
		f.mu.Unlock()
	}
	if p == "" {
		return ""
	}
	return "/" + strings.Trim(p, "/")
}

// findRoom works out the room's dashboard from Home Assistant and keeps it for RoomPath. A house
// that cannot be asked keeps what was found last time.
func (f *Feature) findRoom(ctx context.Context, known []config.DashboardChoice) {
	mac, err := layout.FactoryMAC()
	if err != nil || mac == "" {
		return
	}
	cctx, cancel := context.WithTimeout(ctx, roomWait)
	devices, entities, err := hass.Get().Registries(cctx)
	cancel()
	if err != nil {
		slog.Info("dashboard: asking Home Assistant which room this is failed", "err", err)
		return
	}
	area := ownArea(devices, entities, mac)
	name := ""
	if area != "" {
		// Area ids are slugs, so the id sits in the template as it is.
		if n, err := hass.Get().Render("{{ area_name('" + area + "') }}"); err == nil {
			name = strings.TrimSpace(n)
		}
	}
	p := roomBoard(known, area, name, config.Get().Dashboard.Path)
	f.mu.Lock()
	changed := p != f.room
	f.room = p
	f.mu.Unlock()
	if changed {
		slog.Info("dashboard: the room's dashboard", "area", area, "path", p)
	}
}

// ownArea is the area of the device with this address. A device Home Assistant added twice leaves an
// entry behind with the address on it, so the one with ESPHome's entities on it wins.
func ownArea(devices []hass.Device, entities []hass.RegistryEntity, mac string) string {
	live := make(map[string]bool)
	for _, e := range entities {
		if e.Platform == "esphome" {
			live[e.DeviceID] = true
		}
	}
	found := ""
	for _, d := range devices {
		if !d.Has(mac) || d.Area == "" {
			continue
		}
		if live[d.ID] {
			return d.Area
		}
		if found == "" {
			found = d.Area
		}
	}
	return found
}

// roomBoard is the dashboard for the area out of the known ones: first a whole dashboard whose title
// is the room's name or whose address is made from it ("kitchen", "dashboard-kitchen"), then a view
// with the room's name. The Show's own dashboard (own) and Home Assistant's built-in pages are never
// it.
func roomBoard(known []config.DashboardChoice, area, name, own string) string {
	if area == "" && name == "" {
		return ""
	}
	keys := roomKeys(area, name)
	mine := func(c config.DashboardChoice, base string) bool {
		return c.Streamed || c.Path == own || base == own
	}
	for _, c := range known {
		base, _, _ := strings.Cut(c.Path, "/")
		title, _, _ := strings.Cut(c.Label, " · ")
		if mine(c, base) {
			continue
		}
		if name != "" && strings.EqualFold(strings.TrimSpace(title), name) {
			return base
		}
		if keys[slugKey(base, false)] || keys[slugKey(strings.TrimPrefix(base, "dashboard-"), false)] {
			return base
		}
	}
	if name == "" {
		return ""
	}
	for _, c := range known {
		base, _, _ := strings.Cut(c.Path, "/")
		_, view, ok := strings.Cut(c.Label, " · ")
		if ok && !mine(c, base) && strings.EqualFold(strings.TrimSpace(view), name) {
			return c.Path
		}
	}
	return ""
}

// roomKeys is every way an address could be made from the room: from its id, and from its name with
// umlauts dropped to their letter (Home Assistant's own way, "Küche" is "kuche") or spelled out
// ("kueche", the way a person types it).
func roomKeys(area, name string) map[string]bool {
	keys := make(map[string]bool)
	for _, k := range []string{slugKey(area, false), slugKey(name, false), slugKey(name, true)} {
		if k != "" {
			keys[k] = true
		}
	}
	return keys
}

// slugKey is s with only its letters and digits left, lower case, so "living-room", "living_room"
// and "Living Room" are one key. spell writes ä as ae rather than a.
func slugKey(s string, spell bool) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == 'ß':
			b.WriteString("ss")
		case r == 'ä', r == 'ö', r == 'ü':
			b.WriteString(map[rune]string{'ä': "a", 'ö': "o", 'ü': "u"}[r])
			if spell {
				b.WriteByte('e')
			}
		}
	}
	return b.String()
}
