package home

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/lib/hass"
)

// Room music routing belongs to Music Assistant. Jarvis Show only decides which MA player represents
// this room right now: the configured preferred player while it is available, otherwise this Show's
// own MA player. Queue transfer and grouping remain Music Assistant operations.
const musicRouteEvery = 5 * time.Second

type musicRouteState struct {
	sync.Mutex
	primaryWasPlaying bool
	fallbackActive    bool
	activeOutput      string
	activeGroup       string
	activeMembers     []string
}

var musicRoute musicRouteState

func musicPlayerOnline(st hass.State) bool {
	switch strings.ToLower(strings.TrimSpace(st.State)) {
	case "", "unavailable", "unknown":
		return false
	default:
		return true
	}
}

func musicAssistantState(st hass.State) bool {
	app, _ := st.Attributes["app_id"].(string)
	kind, _ := st.Attributes["mass_player_type"].(string)
	return app == "music_assistant" && (kind == "" || kind == "player")
}

// musicOutput resolves a new explicit Play request. It never migrates an existing fallback session
// back to the preferred speaker merely because that speaker came online again.
func musicOutput() (target, local string, usingFallback bool, err error) {
	local, err = musicAssistantPlayer()
	if err != nil {
		return "", "", false, err
	}
	if local == "" {
		return "", "", false, errors.New("no Music Assistant player for this Jarvis Show")
	}
	primary := strings.TrimSpace(config.Get().Home.MusicPrimary)
	if primary == "" || primary == local {
		return local, local, false, nil
	}
	st, stateErr := hass.Get().State(primary)
	if stateErr != nil || !musicPlayerOnline(st) || !musicAssistantState(st) {
		return local, local, true, nil
	}
	return primary, local, false, nil
}

// PlayMusic starts a new Music Assistant request on this room's currently preferred live output. If a
// previous request failed over to the Show and the preferred speaker has returned, the old local
// fallback is stopped first so the new request cannot play from both places.
func (f *Feature) PlayMusic(mediaID string) (string, error) {
	mediaID = strings.TrimSpace(mediaID)
	if mediaID == "" {
		return "", errors.New("music request is empty")
	}
	if err := releaseActiveMusicGroup(); err != nil {
		return "", err
	}
	target, local, fallback, err := musicOutput()
	if err != nil {
		return "", err
	}

	musicRoute.Lock()
	wasFallback := musicRoute.fallbackActive
	musicRoute.Unlock()
	if wasFallback && target != local {
		if err := hass.Get().Call("media_player", "media_stop", map[string]any{"entity_id": local}); err != nil {
			return "", fmt.Errorf("stop local fallback before returning to preferred speaker: %w", err)
		}
	}
	if err := hass.Get().Call("music_assistant", "play_media", map[string]any{
		"entity_id": target,
		"media_id":  mediaID,
	}); err != nil {
		return "", err
	}

	musicRoute.Lock()
	musicRoute.fallbackActive = fallback
	musicRoute.primaryWasPlaying = !fallback && target != local
	musicRoute.activeOutput = target
	musicRoute.activeGroup = ""
	musicRoute.activeMembers = nil
	musicRoute.Unlock()
	return target, nil
}

// musicRouteLoop watches only for the one automatic migration Jarvis owns: a preferred room speaker
// that was actively playing becomes unavailable. It never performs the reverse migration.
func (f *Feature) musicRouteLoop(ctx context.Context) {
	ticker := time.NewTicker(musicRouteEvery)
	defer ticker.Stop()
	for {
		f.musicRouteTick()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (f *Feature) musicRouteTick() {
	if !hass.Get().Ready() {
		return
	}
	primary := strings.TrimSpace(config.Get().Home.MusicPrimary)
	if primary == "" {
		return
	}
	local, err := musicAssistantPlayer()
	if err != nil || local == "" || local == primary {
		return
	}
	st, err := hass.Get().State(primary)
	if err != nil {
		return
	}

	musicRoute.Lock()
	if musicRoute.fallbackActive {
		musicRoute.Unlock()
		return // sticky until the next explicit Play request
	}
	if musicPlayerOnline(st) {
		if !musicAssistantState(st) {
			musicRoute.primaryWasPlaying = false
			musicRoute.Unlock()
			return
		}
		musicRoute.primaryWasPlaying = strings.EqualFold(st.State, "playing")
		if musicRoute.primaryWasPlaying {
			musicRoute.activeOutput = primary
		}
		musicRoute.Unlock()
		return
	}
	shouldFailOver := musicRoute.primaryWasPlaying
	musicRoute.Unlock()
	if !shouldFailOver {
		return
	}

	if err := hass.Get().Call("music_assistant", "transfer_queue", map[string]any{
		"entity_id":     local,
		"source_player": primary,
		"auto_play":     true,
	}); err != nil {
		slog.Warn("music route: preferred speaker disappeared; queue transfer to Jarvis Show failed", "err", err)
		return
	}
	musicRoute.Lock()
	musicRoute.primaryWasPlaying = false
	musicRoute.fallbackActive = true
	musicRoute.activeOutput = local
	musicRoute.Unlock()
	slog.Info("music route: preferred speaker disappeared; continuing on Jarvis Show", "player", local)
	f.Changed.Emit(struct{}{})
}

// MusicOutput reports the route for display/diagnostics. It does not trigger a route change.
func (f *Feature) MusicOutput() (preferred, active string, fallback bool) {
	preferred = strings.TrimSpace(config.Get().Home.MusicPrimary)
	musicRoute.Lock()
	active, fallback = musicRoute.activeOutput, musicRoute.fallbackActive
	musicRoute.Unlock()
	return
}

// MusicGroupOutput reports the currently explicit named-group route for the full-screen music UI.
// It is observational only and never changes routing.
func (f *Feature) MusicGroupOutput() (group string, members []string) {
	musicRoute.Lock()
	group = musicRoute.activeGroup
	members = append([]string(nil), musicRoute.activeMembers...)
	musicRoute.Unlock()
	return
}
