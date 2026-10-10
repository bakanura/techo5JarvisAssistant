//go:build !dot

package setup

import (
	"net/url"
	"strings"
	"testing"

	"github.com/HuskerMinion/techo5/echod/internal/config"
)

// A Dot has no dashboard page, so this is only for the builds with a screen.
func TestDashboardRoundTripThroughHTTP(t *testing.T) {
	f, c := in(t)
	to := post(t, f, c, url.Values{
		"what":    {"dashboard"},
		"tab":     {"connections"},
		"address": {"https://10.0.0.30:9555/something"},
		"key":     {"dashcast-test-key"},
	})
	if to.Query().Get("problem") != "" {
		t.Fatalf("dashboard save returned %s", to)
	}
	d := config.Get().Dashboard
	if d.Server != "10.0.0.30:9555" || d.Key != "dashcast-test-key" {
		t.Fatalf("dashboard did not round-trip: %+v", d)
	}
	body := get(f, "/setup?tab=connections", c).Body.String()
	if !strings.Contains(body, `value="10.0.0.30:9555"`) || !strings.Contains(body, "A key is saved") {
		t.Fatalf("connections tab did not show saved dashboard state: %s", first(body))
	}
	if strings.Contains(body, "dashcast-test-key") {
		t.Error("dashboard key was rendered back into the setup page")
	}
}
