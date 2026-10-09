package home

import (
	"testing"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/lib/hass"
)

const testMAC = "aa:bb:cc:00:11:22"

func roomDevices() []hass.Device {
	return []hass.Device{
		{ID: "show", Area: "living_room", Connections: [][]string{{"mac", testMAC}}},
		{ID: "sonos", Area: "living_room"},
		{ID: "kitchen", Area: "kitchen"},
		{ID: "ma", Area: ""},
	}
}

func TestRoomPlayerIsTheOneMusicAssistantPlayerInTheShowsArea(t *testing.T) {
	entities := []hass.RegistryEntity{
		{ID: "media_player.show_speaker", DeviceID: "show", Platform: "esphome"},
		{ID: "media_player.show_ma", DeviceID: "show", Platform: "music_assistant"},
		{ID: "media_player.sonos", DeviceID: "sonos", Platform: "sonos"},
		{ID: testPrimary, DeviceID: "sonos", Platform: "music_assistant"},
		{ID: "media_player.kitchen", DeviceID: "kitchen", Platform: "music_assistant"},
	}
	if got := roomPlayerIn(roomDevices(), entities, "AA:BB:CC:00:11:22"); got != testPrimary {
		t.Fatalf("room player = %q, want %q", got, testPrimary)
	}
}

func TestRoomPlayerFollowsTheEntitysOwnArea(t *testing.T) {
	entities := []hass.RegistryEntity{
		{ID: testPrimary, DeviceID: "ma", Platform: "music_assistant", Area: "living_room"},
		{ID: "media_player.kitchen", DeviceID: "sonos", Platform: "music_assistant", Area: "kitchen"},
	}
	if got := roomPlayerIn(roomDevices(), entities, testMAC); got != testPrimary {
		t.Fatalf("room player = %q, want %q", got, testPrimary)
	}
}

func TestRoomPlayerIsNoneWhenItIsNotClearWhich(t *testing.T) {
	two := []hass.RegistryEntity{
		{ID: testPrimary, DeviceID: "sonos", Platform: "music_assistant"},
		{ID: "media_player.other", DeviceID: "ma", Platform: "music_assistant", Area: "living_room"},
	}
	if got := roomPlayerIn(roomDevices(), two, testMAC); got != "" {
		t.Fatalf("two players in the room gave %q, want none", got)
	}
	one := two[:1]
	noArea := roomDevices()
	noArea[0].Area = ""
	if got := roomPlayerIn(noArea, one, testMAC); got != "" {
		t.Fatalf("a Show in no area gave %q, want none", got)
	}
	if got := roomPlayerIn(roomDevices(), one, "aa:bb:cc:99:99:99"); got != "" {
		t.Fatalf("an unknown Show gave %q, want none", got)
	}
}

func TestNamedPrimaryWinsOverTheRoomsPlayer(t *testing.T) {
	setupMusicFailoverTest(t)
	roomPlayer = func() string { return "media_player.kitchen" }
	if err := config.Set().Home().MusicPrimary(testPrimary); err != nil {
		t.Fatal(err)
	}
	if got := musicPrimary(); got != testPrimary {
		t.Fatalf("primary = %q, want the named %q", got, testPrimary)
	}
}

// A Show that is no Music Assistant player itself, in a room whose speaker is one: music goes to the
// room's speaker, and the screen shows what it plays.
func TestShowWithoutItsOwnPlayerShowsTheRoomsMusic(t *testing.T) {
	f, fake := setupMusicFailoverTest(t)
	fake.setState(testLocalMA, "idle", "Somebody Else's Jarvis")
	fake.setState(testPrimary, "playing", "Living Room Sonos")
	roomPlayer = func() string { return testPrimary }

	f.musicRouteTick()
	_, active, fallback := f.MusicOutput()
	if active != testPrimary || fallback {
		t.Fatalf("route = active %q fallback %v, want the room's %q", active, fallback, testPrimary)
	}
	f.musicPlaybackTick()
	if view := f.MusicPlayback(); view.Entity != testPrimary {
		t.Fatalf("Now Playing entity = %q, want %q", view.Entity, testPrimary)
	}

	fake.clearCalls()
	if target, err := f.PlayMusic("library://track/room"); err != nil || target != testPrimary {
		t.Fatalf("PlayMusic = %q, %v; want %q", target, err, testPrimary)
	}

	// The room's speaker goes away: there is nothing here to carry the music on, so nothing is moved.
	fake.clearCalls()
	fake.setState(testPrimary, "unavailable", "Living Room Sonos")
	f.musicRouteTick()
	if calls := fake.serviceCalls(); len(calls) != 0 {
		t.Fatalf("lost room speaker with no player here made calls: %#v", calls)
	}
	if _, err := f.PlayMusic("library://track/nowhere"); err == nil {
		t.Fatal("PlayMusic with no player anywhere did not fail")
	}
}
