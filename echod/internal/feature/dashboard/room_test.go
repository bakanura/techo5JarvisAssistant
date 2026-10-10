//go:build !dot

package dashboard

import (
	"path/filepath"
	"testing"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/lib/hass"
)

var house = []config.DashboardChoice{
	{Label: "Overview · Home", Path: "lovelace/0"},
	{Label: "Overview · Küche", Path: "lovelace/kueche"},
	{Label: "Jarvis Display", Path: "jarvis-display"},
	{Label: "Wohnzimmer", Path: "dashboard-wohnzimmer"},
	{Label: "Bad · Licht", Path: "dashboard-bath/licht"},
	{Label: "Bad · Klima", Path: "dashboard-bath/klima"},
	{Label: "History (streamed only)", Path: "history", Streamed: true},
}

// The room's dashboard is a whole dashboard named after the area, by title or by address, before a view
// with the room's name; never the Show's own dashboard and never a built-in page.
func TestTheRoomsDashboardIsFoundByItsName(t *testing.T) {
	for _, c := range []struct {
		why, area, name, own, want string
	}{
		{"title", "living_room", "Wohnzimmer", "jarvis-display", "dashboard-wohnzimmer"},
		{"address from the id", "bath", "Badezimmer", "jarvis-display", "dashboard-bath"},
		{"a view with the name", "kuche", "Küche", "jarvis-display", "lovelace/kueche"},
		{"the Show's own is not it", "jarvis_display", "Jarvis Display", "jarvis-display", ""},
		{"nor a built-in page", "history", "History", "jarvis-display", ""},
		{"a room with no dashboard", "garage", "Garage", "jarvis-display", ""},
		{"no area", "", "", "jarvis-display", ""},
	} {
		if got := roomBoard(house, c.area, c.name, c.own); got != c.want {
			t.Errorf("%s: %q, want %q", c.why, got, c.want)
		}
	}
	addressed := []config.DashboardChoice{{Label: "Kitchen things", Path: "dashboard-kueche"}}
	if got := roomBoard(addressed, "kuche", "Küche", ""); got != "dashboard-kueche" {
		t.Errorf("address with ue: %q", got)
	}
}

// The area is the one of the device carrying this address that ESPHome's entities are on, so a stale
// entry Home Assistant left behind after a re-add does not move the device to another room.
func TestTheDevicesAreaIsTheLiveEntrys(t *testing.T) {
	mac := "aa:bb:cc:dd:ee:ff"
	devices := []hass.Device{
		{ID: "old", Area: "garage", Connections: [][]string{{"mac", mac}}},
		{ID: "new", Area: "kitchen", Connections: [][]string{{"mac", "AA:BB:CC:DD:EE:FF"}}},
		{ID: "other", Area: "bath", Connections: [][]string{{"mac", "11:22:33:44:55:66"}}},
	}
	entities := []hass.RegistryEntity{{ID: "media_player.x", DeviceID: "new", Platform: "esphome"}}
	if got := ownArea(devices, entities, mac); got != "kitchen" {
		t.Errorf("area %q, want kitchen", got)
	}
	if got := ownArea(devices, nil, mac); got != "garage" {
		t.Errorf("with no entities to go by: %q, want the first, garage", got)
	}
	if got := ownArea(devices, entities, "00:00:00:00:00:00"); got != "" {
		t.Errorf("a device not in Home Assistant is in %q", got)
	}
}

// The room's page is a path in the hello, which every dashcast knows, not a named page.
func TestTheRoomsPageIsSentAsAPath(t *testing.T) {
	cfg := config.Config{}
	cfg.Dashboard = config.Dashboard{Path: "jarvis-display"}
	h := helloFor(cfg, 960, 480, false, "/dashboard-kitchen", "")
	if h["path"] != "/dashboard-kitchen" {
		t.Errorf("path %v", h["path"])
	}
	if _, ok := h["page"]; ok {
		t.Errorf("a room path was sent as a page: %v", h)
	}
	if h := helloFor(cfg, 960, 480, false, PageMusic, ""); h["page"] != PageMusic || h["path"] != "/jarvis-display" {
		t.Errorf("the music page changed: %v", h)
	}
}

// RoomPath is the room set by hand, else the one found, and nothing without dashcast to stream it or
// when it is turned off.
func TestRoomPath(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	f := Get()
	f.mu.Lock()
	f.room = "dashboard-found"
	f.mu.Unlock()
	t.Cleanup(func() {
		f.mu.Lock()
		f.room = ""
		f.mu.Unlock()
	})
	if p := f.RoomPath(); p != "" {
		t.Errorf("no dashcast, yet %q", p)
	}
	if err := config.Set().Dashboard().Server("10.0.0.5:8123", ""); err != nil {
		t.Fatal(err)
	}
	if p := f.RoomPath(); p != "/dashboard-found" {
		t.Errorf("found: %q", p)
	}
	_ = config.Set().Dashboard().Room("lovelace/kitchen")
	if p := f.RoomPath(); p != "/lovelace/kitchen" {
		t.Errorf("set by hand: %q", p)
	}
	_ = config.Set().Dashboard().Room(config.RoomNone)
	if p := f.RoomPath(); p != "" {
		t.Errorf("turned off, yet %q", p)
	}
}
