package home

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/devicename"
	"github.com/HuskerMinion/techo5/echod/internal/layout"
	"github.com/HuskerMinion/techo5/echod/internal/lib/hass"
)

// A name given in Home Assistant.
//
// A device renamed on its setup page announces the new name when it comes back, and Home Assistant
// shows it, unless somebody named the device in Home Assistant: that name wins there. This follows it
// the other way, so a name given in Home Assistant becomes the device's own.
//
// Home Assistant is only read. Clearing a name there takes an administrator's token, and the device
// does not hold one. So the name Home Assistant gave is remembered, and the same one is not taken twice:
// a device renamed on its setup page afterwards keeps that name, and Home Assistant goes on showing its
// own until somebody clears it there.
const (
	// nameRetry is how long a dropped connection waits before it is opened again.
	nameRetry = time.Minute
	// nameEvery looks again in case a change was missed, or Home Assistant would not say when one is made.
	nameEvery = 30 * time.Minute
)

func (f *Feature) nameLoop(ctx context.Context) {
	for {
		err := followName(ctx)
		if ctx.Err() != nil {
			return
		}
		slog.Debug("home: following the name Home Assistant gives this device", "err", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(nameRetry):
		}
	}
}

// followName looks at the device's entry in Home Assistant's device registry now, whenever the registry
// changes, and every so often, until the connection ends.
func followName(ctx context.Context) error {
	if !hass.Get().Ready() {
		return errors.New("no Home Assistant access")
	}
	mac := ownMAC()
	if mac == "" {
		return errors.New("no address of its own to be found by")
	}
	live, err := hass.Get().OpenLive(ctx)
	if err != nil {
		return err
	}
	defer live.Close()

	changed := make(chan struct{}, 1)
	changed <- struct{}{}
	if err := live.SubscribeEvents(ctx, "device_registry_updated", func(map[string]any) {
		select {
		case changed <- struct{}{}:
		default:
		}
	}); err != nil {
		slog.Debug("home: device registry changes are not told; looking every so often instead", "err", err)
	}
	every := time.NewTicker(nameEvery)
	defer every.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-live.Done():
			return live.Err()
		case <-changed:
		case <-every.C:
		}
		if err := checkName(ctx, live, mac); err != nil {
			return err
		}
	}
}

func checkName(ctx context.Context, live *hass.Live, mac string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	raw, err := live.Call(ctx, map[string]any{"type": "config/device_registry/list"})
	if err != nil {
		return err
	}
	var devices []hass.Device
	if err := json.Unmarshal(raw, &devices); err != nil {
		return err
	}
	for _, d := range devices {
		if d.Has(mac) {
			takeName(d.NameByUser)
			return nil
		}
	}
	return nil
}

// takeName renames the device to the name Home Assistant gives it, when that is a name it has not seen
// before.
func takeName(inHA string) {
	inHA = strings.TrimSpace(inHA)
	b, _ := os.ReadFile(layout.NameInHAPath)
	seen := strings.TrimSpace(string(b))
	if inHA == seen {
		return
	}
	to := nameToTake(inHA, seen, config.Get().Device.Name)
	// Remembered before anything else, and nothing taken when it cannot be: a name taken and then
	// forgotten would be taken again after every rename on the setup page.
	if err := os.WriteFile(layout.NameInHAPath, []byte(inHA+"\n"), 0o644); err != nil {
		slog.Warn("home: could not remember the name Home Assistant gave this device", "err", err)
		return
	}
	if to == "" {
		return
	}
	if err := devicename.Set(to, "Home Assistant"); err != nil {
		slog.Warn("home: could not take the name Home Assistant gave this device", "name", to, "err", err)
	}
}

// nameToTake is the name the device takes when Home Assistant gives it inHA, and gave it seen the last
// time it looked: empty to keep its own. Clearing the name in Home Assistant gives nothing back; the
// device's own name is the one Home Assistant falls back to.
func nameToTake(inHA, seen, current string) string {
	switch {
	case inHA == "", inHA == seen, inHA == current:
		return ""
	}
	if problem := devicename.Problem(inHA); problem != "" {
		slog.Warn("home: not taking the name Home Assistant gave this device", "name", inHA, "why", problem)
		return ""
	}
	return inHA
}
