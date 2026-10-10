//go:build !dot && !spot

package display

import (
	"path/filepath"
	"testing"

	"github.com/HuskerMinion/techo5/echod/internal/config"
)

// The streamed dashboard standing in for the clock stays behind the clock until its first picture,
// rather than putting a black page up with "Connecting" on it. One Home Assistant asked for shows up
// straight away, connecting or not.
func TestIdleDashboardWaitsForItsFirstPicture(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	if err := config.Set().Dashboard().Mode(config.DashboardStreamed); err != nil {
		t.Fatal(err)
	}
	if err := config.Set().Dashboard().Idle(true); err != nil {
		t.Fatal(err)
	}

	d := &Display{}
	idle := scene{phase: "idle"}
	d.dashScene(&idle, false)
	if idle.showDash || d.dashShowing {
		t.Fatal("the idle dashboard went up before it had anything to show")
	}

	d.dashboardAsked(true)
	asked := scene{phase: "idle"}
	d.dashScene(&asked, false)
	if !asked.showDash {
		t.Fatal("a dashboard Home Assistant asked for waited for its first picture")
	}
}
