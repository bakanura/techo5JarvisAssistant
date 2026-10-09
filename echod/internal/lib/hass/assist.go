package hass

import (
	"context"
	"encoding/json"
	"strings"
)

// Pipeline is an Assist pipeline as Home Assistant lists it: its name and the language it is set to in
// the voice assistant settings, which is what its speech is recognized and its answers spoken in.
type Pipeline struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Language     string `json:"language"`
	Conversation string `json:"conversation_language"`
}

// AssistLanguage is the language of the Assist pipeline this device's turns run on, "de" or "pt-BR" as
// Home Assistant has it, empty when it cannot be told. The pipeline is the one picked on the device's own
// Assistant select, found by the address the hardware recorded (mac), and the preferred pipeline when
// that select says "preferred" or cannot be found.
func (c *Client) AssistLanguage(ctx context.Context, mac string) (string, error) {
	s, err := c.wsOpen(ctx)
	if err != nil {
		return "", err
	}
	defer s.conn.Close()

	get := func(kind string, into any) error {
		raw, err := s.call(map[string]any{"type": kind})
		if err != nil {
			return err
		}
		return json.Unmarshal(raw, into)
	}
	var list struct {
		Pipelines []Pipeline `json:"pipelines"`
		Preferred string     `json:"preferred_pipeline"`
	}
	if err := get("assist_pipeline/pipeline/list", &list); err != nil {
		return "", err
	}
	// The select's state is the pipeline's name, so it is looked up before the connection closes and
	// read over the REST API after.
	selectID := ""
	if mac != "" {
		var devices []Device
		var entities []RegistryEntity
		if get("config/device_registry/list", &devices) == nil && get("config/entity_registry/list", &entities) == nil {
			selectID = pipelineSelect(devices, entities, mac)
		}
	}
	picked := ""
	if selectID != "" {
		if st, err := c.State(selectID); err == nil {
			picked = st.State
		}
	}
	return pipelineLanguage(list.Pipelines, list.Preferred, picked), nil
}

// pipelineSelect is the Assistant select ESPHome gives the device with this address, empty when there is
// none. Home Assistant keys it by the device's address and "-pipeline", whatever it has been renamed to.
func pipelineSelect(devices []Device, entities []RegistryEntity, mac string) string {
	ours := make(map[string]bool)
	for _, d := range devices {
		if d.Has(mac) {
			ours[d.ID] = true
		}
	}
	for _, e := range entities {
		if ours[e.DeviceID] && e.Platform == "esphome" && strings.HasPrefix(e.ID, "select.") &&
			strings.HasSuffix(e.UniqueID, "-pipeline") {
			return e.ID
		}
	}
	return ""
}

// pipelineLanguage is the language of the pipeline named picked, or of the preferred one when picked is
// "preferred", empty or no pipeline's name.
func pipelineLanguage(pipelines []Pipeline, preferred, picked string) string {
	var pref *Pipeline
	for i, p := range pipelines {
		if picked != "" && picked != "preferred" && p.Name == picked {
			return p.language()
		}
		if p.ID == preferred {
			pref = &pipelines[i]
		}
	}
	if pref == nil {
		return ""
	}
	return pref.language()
}

func (p Pipeline) language() string {
	if p.Language != "" {
		return p.Language
	}
	return p.Conversation
}
