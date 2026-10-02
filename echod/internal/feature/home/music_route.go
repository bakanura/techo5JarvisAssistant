package home

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/feature/media"
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

// MusicPlaybackView is observational state for the native full-screen music surface. Playback keeps
// working if Home Assistant cannot provide these extras; Sendspin remains the source of track/audio.
type MusicPlaybackView struct {
	Route    string
	Entity   string
	Output   string
	Rooms    []string
	Playing  bool
	Paused   bool
	Title    string
	Artist   string
	Album    string
	Position float64
	Duration float64
	Next     string
}

type musicPlaybackState struct {
	sync.Mutex
	view     MusicPlaybackView
	queueAt  time.Time
	queueFor string
}

var musicPlayback musicPlaybackState

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
		f.musicPlaybackTick()
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

func musicRoomNameFor(entity string) string {
	entity = strings.TrimSpace(entity)
	if entity == "" {
		return ""
	}
	cfg := config.Get().Home
	for _, r := range cfg.MusicRooms {
		if entity == strings.TrimSpace(r.Primary) || entity == strings.TrimSpace(r.Fallback) {
			return r.Name
		}
	}
	if active, _ := musicAssistantPlayer(); entity == active && cfg.MusicRoom != "" {
		return cfg.MusicRoom
	}
	return ""
}

func humanMusicName(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	v = strings.TrimPrefix(v, "media_player.")
	v = strings.ReplaceAll(v, "_", " ")
	return v
}

func floatAttr(st hass.State, name string) float64 {
	switch v := st.Attributes[name].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	default:
		return 0
	}
}

func (f *Feature) musicPlaybackTick() {
	if !hass.Get().Ready() {
		return
	}
	group, members := f.MusicGroupOutput()
	_, active, _ := f.MusicOutput()
	if active == "" && media.Get().Carried() {
		active, _ = musicAssistantPlayer()
	}
	if active == "" {
		musicPlayback.Lock()
		hadPicture := musicPlayback.picture != ""
		musicPlayback.view = MusicPlaybackView{}
		musicPlayback.queueAt, musicPlayback.queueFor = time.Time{}, ""
		musicPlayback.picture = ""
		musicPlayback.Unlock()
		if hadPicture && !media.Get().Carried() {
			RemoteArt(nil, nil)
		}
		return
	}

	view := MusicPlaybackView{Route: group, Entity: active, Output: humanMusicName(active)}
	if st, err := hass.Get().State(active); err == nil {
		if friendly, _ := st.Attributes["friendly_name"].(string); strings.TrimSpace(friendly) != "" {
			view.Output = strings.TrimSpace(friendly)
		}
		switch strings.ToLower(strings.TrimSpace(st.State)) {
		case "playing":
			view.Playing = true
		case "paused":
			view.Paused = true
		}
		view.Title, _ = st.Attributes["media_title"].(string)
		view.Artist, _ = st.Attributes["media_artist"].(string)
		view.Album, _ = st.Attributes["media_album_name"].(string)
		picture, _ := st.Attributes["entity_picture"].(string)
		if !media.Get().Carried() {
			musicPlayback.Lock()
			changed := picture != musicPlayback.picture
			if changed {
				musicPlayback.picture = picture
			}
			musicPlayback.Unlock()
			if changed {
				if picture == "" {
					RemoteArt(nil, nil)
				} else if b, err := hass.Get().FetchURL(picture); err == nil {
					RemoteArt(b, nil)
				} else {
					slog.Debug("music route: cover art unavailable", "err", err)
				}
			}
		}
		view.Position = floatAttr(st, "media_position")
		view.Duration = floatAttr(st, "media_duration")
		if view.Playing && view.Duration > 0 {
			if updated, _ := st.Attributes["media_position_updated_at"].(string); updated != "" {
				if at, err := time.Parse(time.RFC3339Nano, updated); err == nil {
					view.Position += time.Since(at).Seconds()
					if view.Position > view.Duration {
						view.Position = view.Duration
					}
				}
			}
		}
	}
	seenRoom := map[string]bool{}
	for _, member := range members {
		name := musicRoomNameFor(member)
		if name == "" {
			name = humanMusicName(member)
		}
		if name != "" && !seenRoom[name] {
			seenRoom[name] = true
			view.Rooms = append(view.Rooms, name)
		}
	}
	if group == "" {
		if room := musicRoomNameFor(active); room != "" {
			view.Rooms = []string{room}
		}
	}

	musicPlayback.Lock()
	needQueue := musicPlayback.queueFor != active || time.Since(musicPlayback.queueAt) >= 15*time.Second
	if !needQueue {
		view.Next = musicPlayback.view.Next
	}
	musicPlayback.Unlock()
	if needQueue {
		if next, err := musicQueueNext(active); err == nil {
			view.Next = next
			musicPlayback.Lock()
			musicPlayback.queueFor, musicPlayback.queueAt = active, time.Now()
			musicPlayback.Unlock()
		}
	}
	musicPlayback.Lock()
	musicPlayback.view = view
	musicPlayback.Unlock()
	f.Changed.Emit(struct{}{})
}

func musicQueueNext(entity string) (string, error) {
	raw, err := hass.Get().CallResponse("music_assistant", "get_queue", map[string]any{"entity_id": entity})
	if err != nil {
		return "", err
	}
	var response map[string]struct {
		NextItem *struct {
			Name string `json:"name"`
		} `json:"next_item"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return "", err
	}
	q, ok := response[entity]
	if !ok || q.NextItem == nil {
		return "", nil
	}
	return strings.TrimSpace(q.NextItem.Name), nil
}

// MusicPlayback reports optional routing/progress/queue context for the native music page. It never
// performs network I/O; the route loop refreshes this cache in the background.
func (f *Feature) MusicPlayback() MusicPlaybackView {
	musicPlayback.Lock()
	defer musicPlayback.Unlock()
	v := musicPlayback.view
	v.Rooms = append([]string(nil), v.Rooms...)
	return v
}

// MusicTransport controls the real Music Assistant route shown by the native music page. It falls
// back to the local media transport only when no routed MA player is active.
func (f *Feature) MusicTransport(t media.Transport) error {
	v := f.MusicPlayback()
	if v.Entity == "" || (!v.Playing && !v.Paused) {
		media.Get().Transport(t)
		return nil
	}
	service := ""
	switch t {
	case media.TransportPrevious:
		service = "media_previous_track"
	case media.TransportNext:
		service = "media_next_track"
	case media.TransportPlay:
		service = "media_play"
	case media.TransportPause:
		service = "media_pause"
	case media.TransportToggle:
		if v.Paused {
			service = "media_play"
		} else {
			service = "media_pause"
		}
	case media.TransportStop:
		service = "media_stop"
	}
	if service == "" {
		return nil
	}
	return hass.Get().Call("media_player", service, map[string]any{"entity_id": v.Entity})
}

func (f *Feature) stopRoutedMusic() bool {
	v := f.MusicPlayback()
	if v.Entity == "" || (!v.Playing && !v.Paused) {
		return false
	}
	if err := f.MusicTransport(media.TransportStop); err != nil {
		slog.Warn("music route: stopping active Music Assistant output failed", "err", err)
		return true
	}
	if v.Route != "" {
		if err := releaseActiveMusicGroup(); err != nil {
			slog.Warn("music route: releasing stopped temporary group failed", "err", err)
		}
	}
	return true
}

// MusicPlaybackArt is the best cover art available for the current MA route. Sendspin supplies it
// when this Show is an output; an external preferred player falls back to its HA entity_picture.
func (f *Feature) MusicPlaybackArt() (art, thumb *image.RGBA) { return remoteArt() }
