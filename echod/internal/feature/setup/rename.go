package setup

import (
	"strings"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/devicename"
)

// Renaming a device.
//
// Home Assistant keys its entities on the device's MAC address, so a renamed device is the same
// device to it: the entity ids it was given stay, and automations naming them keep working. What
// changes is what Home Assistant shows, and that the entity ids stop looking like the name — enough
// to be told about, which is what the box on the page is for.
//
// On a device that has never met a Home Assistant there is nothing to think about at all, and naming
// one before it is handed to somebody is exactly what this is for.

// rename writes the new name and restarts, since the name is announced when the daemon starts.
// It reports what was wrong, or empty when the device is on its way back up.
func rename(to string, acknowledged bool) string {
	switch {
	case devicename.Problem(to) != "":
		return devicename.Problem(to)
	case strings.TrimSpace(to) == config.Get().Device.Name:
		return "that is already its name"
	case !acknowledged:
		return "tick the box to say you know the entity ids in Home Assistant do not change with it"
	}
	if err := devicename.Set(to, "the setup page"); err != nil {
		return "could not write the name: " + err.Error()
	}
	return ""
}
