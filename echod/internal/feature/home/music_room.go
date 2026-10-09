package home

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/lib/hass"
)

// The room's music speaker when nobody has named one (music_primary_player): the Music Assistant player
// Home Assistant puts in the same area as this device. Music started by voice or an automation lands on
// the room's speaker, not on the Show, and without knowing which speaker that is the screen had nothing
// to show while the room played music. Only one such player in the area counts; with two or more there
// is no telling which one is meant, and the setting is the way to say.

// roomPlayerEvery is how long the answer is trusted. Short, so a device just put in an area, or a
// speaker just added to one, is picked up within minutes.
const roomPlayerEvery = 10 * time.Minute

var roomPlayerCache struct {
	sync.Mutex
	id string
	at time.Time
}

// roomPlayer is replaced in tests, which have no registry to ask.
var roomPlayer = func() string {
	roomPlayerCache.Lock()
	if !roomPlayerCache.at.IsZero() && time.Since(roomPlayerCache.at) < roomPlayerEvery {
		id := roomPlayerCache.id
		roomPlayerCache.Unlock()
		return id
	}
	roomPlayerCache.Unlock()

	id := findRoomPlayer()
	roomPlayerCache.Lock()
	if id != roomPlayerCache.id {
		slog.Info("music route: the room's Music Assistant player", "entity", id)
	}
	roomPlayerCache.id, roomPlayerCache.at = id, time.Now()
	roomPlayerCache.Unlock()
	return id
}

// musicPrimary is the room's preferred speaker: the one somebody named, else the one in this device's
// area, else none.
func musicPrimary() string {
	if p := strings.TrimSpace(config.Get().Home.MusicPrimary); p != "" {
		return p
	}
	if !hass.Get().Ready() {
		return ""
	}
	return roomPlayer()
}

func findRoomPlayer() string {
	mac := ownMAC()
	if mac == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), registryWait)
	defer cancel()
	devices, entities, err := hass.Get().Registries(ctx)
	if err != nil {
		slog.Debug("music route: asking Home Assistant for its registries", "err", err)
		return ""
	}
	return roomPlayerIn(devices, entities, mac)
}

// roomPlayerIn is the one Music Assistant player in the area of the device known by mac, empty when the
// device has no area or the area has none of them, or more than one.
func roomPlayerIn(devices []hass.Device, entities []hass.RegistryEntity, mac string) string {
	ours := map[string]bool{}
	areaOf := map[string]string{}
	area := ""
	for _, d := range devices {
		areaOf[d.ID] = d.Area
		if d.Has(mac) {
			ours[d.ID] = true
			if d.Area != "" {
				area = d.Area
			}
		}
	}
	if area == "" {
		return ""
	}
	found := ""
	for _, e := range entities {
		if e.Platform != "music_assistant" || !strings.HasPrefix(e.ID, "media_player.") || ours[e.DeviceID] {
			continue
		}
		in := e.Area
		if in == "" {
			in = areaOf[e.DeviceID]
		}
		if in != area {
			continue
		}
		if found != "" {
			return ""
		}
		found = e.ID
	}
	return found
}
