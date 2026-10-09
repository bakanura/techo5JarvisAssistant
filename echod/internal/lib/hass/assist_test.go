package hass

import "testing"

func TestPipelineLanguage(t *testing.T) {
	pipes := []Pipeline{
		{ID: "a", Name: "Home Assistant", Language: "en"},
		{ID: "b", Name: "Jarvis", Language: "de"},
		{ID: "c", Name: "Plain", Conversation: "fr"},
	}
	for _, c := range []struct{ preferred, picked, want string }{
		{"b", "preferred", "de"},
		{"b", "", "de"},
		{"b", "Home Assistant", "en"},
		{"a", "Plain", "fr"},
		{"a", "gone", "en"},
		{"x", "preferred", ""},
	} {
		if got := pipelineLanguage(pipes, c.preferred, c.picked); got != c.want {
			t.Errorf("preferred %q picked %q: %q, want %q", c.preferred, c.picked, got, c.want)
		}
	}
}

// The select is found by the device's address and its unique id, not by its entity id, which a rename
// changes.
func TestPipelineSelect(t *testing.T) {
	devices := []Device{
		{ID: "stale", Connections: [][]string{{"mac", "aa:bb:cc:dd:ee:ff"}}},
		{ID: "dev", Connections: [][]string{{"mac", "AA:BB:CC:DD:EE:FF"}}},
		{ID: "other", Connections: [][]string{{"mac", "11:22:33:44:55:66"}}},
	}
	entities := []RegistryEntity{
		{ID: "select.other_assistant", DeviceID: "other", Platform: "esphome", UniqueID: "112233445566-pipeline"},
		{ID: "select.kitchen_finished_speaking_detection", DeviceID: "dev", Platform: "esphome", UniqueID: "AABBCCDDEEFF-vad_sensitivity"},
		{ID: "select.kitchen_assistant", DeviceID: "dev", Platform: "esphome", UniqueID: "AABBCCDDEEFF-pipeline"},
	}
	if got := pipelineSelect(devices, entities, "aa:bb:cc:dd:ee:ff"); got != "select.kitchen_assistant" {
		t.Errorf("got %q", got)
	}
	if got := pipelineSelect(devices, entities, "00:00:00:00:00:01"); got != "" {
		t.Errorf("a device Home Assistant does not know found %q", got)
	}
}
