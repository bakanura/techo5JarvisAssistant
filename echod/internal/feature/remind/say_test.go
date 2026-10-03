package remind

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/HuskerMinion/techo5/echod/internal/component"
	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/lib/hass"
)

// Home Assistant 2026.9 refuses preannounce sent as the text "false", so the label went unspoken; it
// goes as a template now, which renders to a real false (#58).
func TestSayingALabelSendsPreannounceAsATemplate(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	// Say uses HA only when access is configured. Keep that route and its entity lookup local.
	oldPath := hass.Path
	hass.Path = filepath.Join(t.TempDir(), "hass.json")
	t.Cleanup(func() { hass.Path = oldPath })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/states" || r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Errorf("unexpected Home Assistant entity lookup: %s", r.URL.Path)
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)
	if err := hass.Get().Set(server.URL, "fixture-token"); err != nil {
		t.Fatal(err)
	}
	var got []component.Call
	stop := component.CallService.Listen(func(c component.Call) { got = append(got, c) })
	defer stop()

	Say("Take the trash out")

	if len(got) != 1 {
		t.Fatalf("%d calls, want 1", len(got))
	}
	c := got[0]
	if c.Service != "assist_satellite.announce" || c.Data["message"] != "Take the trash out" {
		t.Errorf("call %+v", c)
	}
	if _, asText := c.Data["preannounce"]; asText {
		t.Error("preannounce is still sent as text, which Home Assistant 2026.9 refuses")
	}
	if c.Templates["preannounce"] != "{{ false }}" {
		t.Errorf("preannounce template %q, want {{ false }}", c.Templates["preannounce"])
	}
}
