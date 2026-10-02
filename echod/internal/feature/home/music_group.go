package home

import (
	"errors"
	"fmt"
	"strings"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/lib/hass"
)

const (
	maxMusicRooms        = 32
	maxMusicGroups       = 16
	maxMusicGroupRooms   = 32
	maxMusicGroupAliases = 8
)

func musicName(v string) string { return strings.ToLower(strings.TrimSpace(v)) }

func validMAEntity(v string) bool {
	v = strings.TrimSpace(v)
	return v != "" && strings.HasPrefix(v, "media_player.") && len(v) <= 255
}

func validateMusicRouting(localRoom string, rooms []config.MusicRoomRoute, groups []config.MusicGroup) (string, []config.MusicRoomRoute, []config.MusicGroup, error) {
	localRoom = musicName(localRoom)
	if len(localRoom) > 64 {
		return "", nil, nil, errors.New("music routing: local room name is too long")
	}
	if len(rooms) > maxMusicRooms || len(groups) > maxMusicGroups {
		return "", nil, nil, errors.New("music routing: too many rooms or groups")
	}
	roomNames := map[string]bool{}
	for i := range rooms {
		rooms[i].Name = musicName(rooms[i].Name)
		rooms[i].Primary = strings.TrimSpace(rooms[i].Primary)
		rooms[i].Fallback = strings.TrimSpace(rooms[i].Fallback)
		if rooms[i].Name == "" || len(rooms[i].Name) > 64 || roomNames[rooms[i].Name] {
			return "", nil, nil, fmt.Errorf("music routing: invalid or duplicate room %q", rooms[i].Name)
		}
		roomNames[rooms[i].Name] = true
		if rooms[i].Primary != "" && !validMAEntity(rooms[i].Primary) {
			return "", nil, nil, fmt.Errorf("music routing: room %q has invalid primary", rooms[i].Name)
		}
		if rooms[i].Fallback != "" && !validMAEntity(rooms[i].Fallback) {
			return "", nil, nil, fmt.Errorf("music routing: room %q has invalid fallback", rooms[i].Name)
		}
		if rooms[i].Name != localRoom && rooms[i].Fallback == "" {
			return "", nil, nil, fmt.Errorf("music routing: remote room %q needs a Jarvis fallback", rooms[i].Name)
		}
	}
	seenGroup := map[string]bool{}
	seenAlias := map[string]bool{}
	for i := range groups {
		groups[i].Name = musicName(groups[i].Name)
		if groups[i].Name == "" || len(groups[i].Name) > 64 || seenGroup[groups[i].Name] || seenAlias[groups[i].Name] {
			return "", nil, nil, fmt.Errorf("music routing: invalid or duplicate group %q", groups[i].Name)
		}
		seenGroup[groups[i].Name] = true
		if len(groups[i].Rooms) == 0 || len(groups[i].Rooms) > maxMusicGroupRooms || len(groups[i].Aliases) > maxMusicGroupAliases {
			return "", nil, nil, fmt.Errorf("music routing: invalid member/alias count for %q", groups[i].Name)
		}
		memberSeen := map[string]bool{}
		for j := range groups[i].Rooms {
			groups[i].Rooms[j] = musicName(groups[i].Rooms[j])
			if !roomNames[groups[i].Rooms[j]] || memberSeen[groups[i].Rooms[j]] {
				return "", nil, nil, fmt.Errorf("music routing: group %q references invalid/duplicate room %q", groups[i].Name, groups[i].Rooms[j])
			}
			memberSeen[groups[i].Rooms[j]] = true
		}
		for j := range groups[i].Aliases {
			groups[i].Aliases[j] = musicName(groups[i].Aliases[j])
			a := groups[i].Aliases[j]
			if a == "" || len(a) > 64 || seenGroup[a] || seenAlias[a] {
				return "", nil, nil, fmt.Errorf("music routing: invalid or duplicate alias %q", a)
			}
			seenAlias[a] = true
		}
	}
	return localRoom, rooms, groups, nil
}

func findMusicGroup(name string) (config.MusicGroup, bool) {
	name = musicName(name)
	for _, g := range config.Get().Home.MusicGroups {
		if musicName(g.Name) == name {
			return g, true
		}
		for _, a := range g.Aliases {
			if musicName(a) == name {
				return g, true
			}
		}
	}
	return config.MusicGroup{}, false
}

func musicEntityUsable(entity string) bool {
	if !validMAEntity(entity) {
		return false
	}
	st, err := hass.Get().State(entity)
	return err == nil && musicPlayerOnline(st) && musicAssistantState(st)
}

func resolveMusicGroup(name string) (config.MusicGroup, []string, error) {
	group, ok := findMusicGroup(name)
	if !ok {
		return config.MusicGroup{}, nil, fmt.Errorf("unknown music group %q", strings.TrimSpace(name))
	}
	cfg := config.Get().Home
	roomByName := make(map[string]config.MusicRoomRoute, len(cfg.MusicRooms))
	for _, r := range cfg.MusicRooms {
		roomByName[musicName(r.Name)] = r
	}
	localPlayer := ""
	outputs := make([]string, 0, len(group.Rooms))
	seen := map[string]bool{}
	for _, roomName := range group.Rooms {
		r, ok := roomByName[musicName(roomName)]
		if !ok {
			continue
		}
		chosen := ""
		if musicEntityUsable(r.Primary) {
			chosen = r.Primary
		} else {
			fallback := strings.TrimSpace(r.Fallback)
			if fallback == "" && musicName(r.Name) == musicName(cfg.MusicRoom) {
				if localPlayer == "" {
					localPlayer, _ = musicAssistantPlayer()
				}
				fallback = localPlayer
			}
			if musicEntityUsable(fallback) {
				chosen = fallback
			}
		}
		if chosen != "" && !seen[chosen] {
			seen[chosen] = true
			outputs = append(outputs, chosen)
		}
	}
	if len(outputs) == 0 {
		return group, nil, fmt.Errorf("music group %q has no available outputs", group.Name)
	}
	return group, outputs, nil
}

func releaseActiveMusicGroup() error {
	musicRoute.Lock()
	group := musicRoute.activeGroup
	members := append([]string(nil), musicRoute.activeMembers...)
	musicRoute.Unlock()
	if group == "" || len(members) == 0 {
		return nil
	}
	for _, member := range members {
		if err := hass.Get().Call("media_player", "unjoin", map[string]any{"entity_id": member}); err != nil {
			return fmt.Errorf("release previous Jarvis music group %q: %w", group, err)
		}
	}
	musicRoute.Lock()
	if musicRoute.activeGroup == group {
		musicRoute.activeGroup = ""
		musicRoute.activeMembers = nil
		musicRoute.activeOutput = ""
	}
	musicRoute.Unlock()
	return nil
}

// PlayMusicGroup resolves one live Music Assistant output per configured room, forms a temporary
// HA/MA sync group, and starts the requested media on its leader. Membership is re-evaluated on
// every explicit call; Jarvis does not continuously reshuffle an already-playing group.
func (f *Feature) PlayMusicGroup(groupName, mediaID string) (string, []string, error) {
	mediaID = strings.TrimSpace(mediaID)
	if mediaID == "" {
		return "", nil, errors.New("music request is empty")
	}
	group, outputs, err := resolveMusicGroup(groupName)
	if err != nil {
		return "", nil, err
	}
	if err := releaseActiveMusicGroup(); err != nil {
		return "", nil, err
	}
	leader := outputs[0]
	if len(outputs) > 1 {
		if err := hass.Get().Call("media_player", "join", map[string]any{
			"entity_id":     leader,
			"group_members": outputs[1:],
		}); err != nil {
			return "", nil, fmt.Errorf("form Music Assistant group %q: %w", group.Name, err)
		}
	}
	if err := hass.Get().Call("music_assistant", "play_media", map[string]any{
		"entity_id": leader,
		"media_id":  mediaID,
	}); err != nil {
		return "", nil, err
	}
	musicRoute.Lock()
	musicRoute.primaryWasPlaying = false
	musicRoute.fallbackActive = false
	musicRoute.activeOutput = leader
	musicRoute.activeGroup = group.Name
	musicRoute.activeMembers = append([]string(nil), outputs...)
	musicRoute.Unlock()
	f.Changed.Emit(struct{}{})
	return group.Name, outputs, nil
}
