package announce

import (
	"path/filepath"
	"testing"

	"github.com/HuskerMinion/techo5/echod/internal/config"
)

func TestDoNotDisturbMutesHouseAnnouncements(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	if announcementQuiet() {
		t.Fatal("a fresh device unexpectedly considers announcements quiet")
	}
	if err := config.Set().Home().DoNotDisturb(true); err != nil {
		t.Fatal(err)
	}
	if !announcementQuiet() {
		t.Fatal("do not disturb did not mute house announcements")
	}
}
