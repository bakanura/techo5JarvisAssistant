package home

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/i18n"
	"github.com/HuskerMinion/techo5/echod/internal/lib/hass"
)

// The device's own camera is on Home Assistant's list too, through its ESPHome entity. The cameras page
// showed it twice — once as the local camera and once under the name Home Assistant gave it — and both
// were the same sensor.
func TestTheDevicesOwnCameraIsListedOnce(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	i18n.Changed()
	t.Cleanup(i18n.Changed)

	cams := []config.Camera{
		{Entity: "camera.kitchen_show_camera", Name: "Kitchen Show Camera"}, // renamed: only the registry knows
		{Entity: "camera.hall_camera", Name: "Hall Camera"},                 // the id it would have had
		{Entity: "camera.front_door", Name: "Front door"},
	}
	got := withLocal(cams, map[string]bool{"camera.kitchen_show_camera": true}, "camera.hall_camera")
	want := []config.Camera{{Entity: LocalCamera, Name: "This device"}, {Entity: "camera.front_door", Name: "Front door"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("withLocal = %v, want %v", got, want)
	}

	// Home Assistant could not say: the guess still takes the usual one off.
	got = withLocal(cams[1:], nil, "camera.hall_camera")
	if len(got) != 2 || got[1].Entity != "camera.front_door" {
		t.Errorf("withLocal with no registry answer = %v", got)
	}
}

func TestTheLocalCameraIsNamedInTheScreensLanguage(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	if err := config.Set().Screen().Language("de"); err != nil {
		t.Fatal(err)
	}
	i18n.Changed()
	t.Cleanup(i18n.Changed)

	if got := withLocal(nil, nil, "")[0].Name; got != "Dieses Gerät" {
		t.Errorf("local camera on a German screen = %q", got)
	}
}

func TestTheDevicesOwnCamerasAreFoundByItsAddress(t *testing.T) {
	devices := []hass.Device{
		{ID: "dev-this", Connections: [][]string{{"mac", "aa:bb:cc:dd:ee:02"}}},
		{ID: "dev-stale", Connections: [][]string{{"mac", "aa:bb:cc:dd:ee:02"}}},
		{ID: "dev-other", Connections: [][]string{{"mac", "aa:bb:cc:dd:ee:01"}}},
	}
	entities := []hass.RegistryEntity{
		{ID: "media_player.kitchen_speaker", DeviceID: "dev-this", Platform: "esphome"},
		{ID: "camera.kitchen_camera", DeviceID: "dev-this", Platform: "esphome"},
		{ID: "camera.porch_camera", DeviceID: "dev-other", Platform: "esphome"},
		{ID: "camera.kitchen_generic", DeviceID: "dev-this", Platform: "generic"},
	}
	got := ownEntities(devices, entities, "AA:BB:CC:DD:EE:02", "camera")
	if !reflect.DeepEqual(got, []string{"camera.kitchen_camera"}) {
		t.Errorf("ownEntities = %v", got)
	}
	if got := mine(devices, entities, "aa:bb:cc:dd:ee:02"); got != "media_player.kitchen_speaker" {
		t.Errorf("mine = %q", got)
	}
}
