// Package devicename changes what the device is called, from the setup page or from Home Assistant.
//
// The name is announced when the daemon starts, so a new one is written where boot reads it and the
// daemon restarts. The daemon, not the device: the name is only the daemon's, and a reboot would bypass
// the supervisor and roll back an update still on trial.
package devicename

import (
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/layout"
	"github.com/HuskerMinion/techo5/echod/internal/lib/safe"
	"github.com/HuskerMinion/techo5/echod/internal/update"
)

// Limit is what the installer allows too: one line, short enough for the screen.
const Limit = 31

// restartAfter is long enough for whoever asked to hear that it worked, rather than see a connection
// die mid-request.
var (
	restartAfter = 3 * time.Second
	restart      = update.Restart
)

// Problem says what is wrong with to as a name, empty when nothing is.
func Problem(to string) string {
	to = strings.TrimSpace(to)
	switch {
	case to == "":
		return "the device needs a name"
	case len(to) > Limit:
		return "a name is at most 31 characters"
	case strings.ContainsAny(to, "\n\r\t"):
		return "a name is one line"
	}
	return ""
}

// Set writes to as the device's name and restarts to announce it, why saying who asked. The name must
// have passed Problem.
func Set(to, why string) error {
	to = strings.TrimSpace(to)
	if err := os.WriteFile(layout.NamePath, []byte(to+"\n"), 0o644); err != nil {
		return err
	}
	slog.Warn("device renamed; restarting to announce it", "to", to, "by", why)
	safe.Go("restart after rename", func() {
		time.Sleep(restartAfter)
		restart("renamed")
	})
	return nil
}
