package main

import (
	"context"
	"testing"
)

// Music Assistant's panel is found by its app, under the slugs its repositories give it, and never
// one for administrators or anything else that frames a page.
func TestMusicAssistantsPanelIsFound(t *testing.T) {
	for _, c := range []struct {
		panels map[string]panel
		want   string
	}{
		{map[string]panel{"lovelace": {component: "lovelace"}, "d5369777_music_assistant": {component: "app", addon: "d5369777_music_assistant"}}, "d5369777_music_assistant"},
		{map[string]panel{"music": {component: "app", addon: "music_assistant"}}, "music"},
		{map[string]panel{"mab": {component: "app", addon: "d5369777_music_assistant_beta"}}, "mab"},
		{map[string]panel{"ma": {component: "app", addon: "d5369777_music_assistant", admin: true}}, ""},
		{map[string]panel{"ma": {component: "iframe", url: "http://10.0.0.5:8095"}}, ""},
		{map[string]panel{"esphome": {component: "app", addon: "5c53de3b_esphome"}}, ""},
		{map[string]panel{"x": {component: "app", addon: "not_music_assistant_really"}}, ""},
		{map[string]panel{"b": {component: "app", addon: "music_assistant"}, "a": {component: "app", addon: "x_music_assistant_beta"}}, "a"},
	} {
		if got := musicPanel(c.panels); got != c.want {
			t.Errorf("%v: found %q, want %q", c.panels, got, c.want)
		}
	}
}

// A Music Assistant session is kept on its panel: no dashboard, no other page, nothing that only
// starts with the panel's name.
func TestOnlyIsOnePanel(t *testing.T) {
	allows := only("d5369777_music_assistant")
	for path, want := range map[string]bool{
		"/d5369777_music_assistant":         true,
		"/d5369777_music_assistant/":        true,
		"/d5369777_music_assistant_beta":    false,
		"/lovelace/0":                       false,
		"/config/dashboard":                 false,
		"/d5369777_music_assistant/../conf": false,
		"":                                  false,
	} {
		if got, err := allows(context.Background(), path); err != nil || got != want {
			t.Errorf("%q: allowed %v (%v), want %v", path, got, err, want)
		}
	}
}
