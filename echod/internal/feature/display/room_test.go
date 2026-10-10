//go:build !dot && !spot

package display

import (
	"path/filepath"
	"testing"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/feature/dashboard"
)

// The swipe in from the left goes round: the clock, the Show's dashboard, the room's, the clock. With
// no room's dashboard it is the clock and the Show's dashboard, as it always was.
func TestBackGoesRoundThroughTheRoomsDashboard(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	if err := config.Set().Dashboard().Server("127.0.0.1:1", ""); err != nil {
		t.Fatal(err)
	}
	if err := config.Set().Dashboard().Room("dashboard-kitchen"); err != nil {
		t.Fatal(err)
	}
	d := &Display{}
	if !d.openDashboard() || d.dashPage != "" {
		t.Fatalf("the clock's swipe did not put the Show's dashboard up: page %q", d.dashPage)
	}
	d.back()
	if !d.dash || d.dashPage != "/dashboard-kitchen" {
		t.Fatalf("from the Show's dashboard: dash %v page %q, want the room's", d.dash, d.dashPage)
	}
	s := scene{phase: "idle"}
	d.dashScene(&s, false)
	if !s.showDash || s.dashMode != config.DashboardStreamed || s.dashPage != "/dashboard-kitchen" {
		t.Fatalf("room's page: shown %v mode %v page %q", s.showDash, s.dashMode, s.dashPage)
	}
	d.back()
	if d.dash {
		t.Fatalf("from the room's dashboard: still up on %q, want the clock", d.dashPage)
	}

	// From the library, back is still the Show's dashboard, and the room's after it.
	d.dash, d.dashPage = true, dashboard.PageMusic
	d.back()
	if !d.dash || d.dashPage != "" {
		t.Fatalf("from the library: page %q", d.dashPage)
	}

	if err := config.Set().Dashboard().Room(config.RoomNone); err != nil {
		t.Fatal(err)
	}
	d.back()
	if d.dash {
		t.Fatal("with no room's dashboard the swipe did not take the dashboard away")
	}
}
