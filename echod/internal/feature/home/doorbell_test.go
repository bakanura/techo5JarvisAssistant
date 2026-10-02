package home

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/HuskerMinion/techo5/echod/internal/config"
)

func TestDoorbellCameraEntityGate(t *testing.T) {
	for _, good := range []string{"camera.front_door", "camera.driveway", LocalCamera} {
		if !validCameraEntity(good) {
			t.Errorf("%q was refused", good)
		}
	}
	for _, bad := range []string{"", "light.front_door", "http://camera", "camera"} {
		if validCameraEntity(bad) {
			t.Errorf("%q was accepted", bad)
		}
	}
}

func TestDoorbellCameraNameIsSafeForTheScreen(t *testing.T) {
	got := safeCameraName("  Front door\x1b[2J\n" + strings.Repeat("x", 100))
	if strings.ContainsAny(got, "\x1b\n\r") {
		t.Fatalf("control text survived: %q", got)
	}
	if len([]rune(got)) > cameraNameMost {
		t.Fatalf("camera name kept %d runes, want <= %d", len([]rune(got)), cameraNameMost)
	}
}

func TestDoNotDisturbMakesDoorbellVisualOnly(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	if !doorbellAudible() {
		t.Fatal("a normal doorbell started muted")
	}
	if err := config.Set().Home().DoNotDisturb(true); err != nil {
		t.Fatal(err)
	}
	if doorbellAudible() {
		t.Fatal("doorbell remained audible under DND")
	}
}
