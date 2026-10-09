package firmware

import (
	"log/slog"
	"strings"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/feature/remind"
	"github.com/HuskerMinion/techo5/echod/internal/i18n"
	"github.com/HuskerMinion/techo5/echod/internal/lib/safe"
)

// Saying that an update is ready. Home Assistant shows its update card, and the screen says so under the
// clock, but nobody opens either to look: the device says it out loud, and again every remindEvery until
// the update is installed, so it cannot be missed. Not in quiet hours, and not when there is nothing to
// say it with; the next check tries again, so a version found at night is said in the morning. With
// automatic installs on there is nothing for anybody to do, so that is said once.
var (
	say    = func(words string) { safe.Go("update notice", func() { remind.Say(words) }) }
	canSay = remind.CanSay
	quiet  = config.Quiet
)

// remindEvery is how long after saying an update is ready it is said again, while it still waits.
const remindEvery = 4 * time.Hour

func (u *Firmware) tell() { u.tellOf(u.Offered(), time.Now()) }

// tellOf says that v is ready, if it is due and can be said now.
func (u *Firmware) tellOf(v string, now time.Time) {
	if v == "" || quiet() || !canSay() {
		return
	}
	auto := u.AutoInstall()
	c := config.Get().Update
	if v == c.Told && (auto || now.Sub(c.ToldAt) < remindEvery) {
		return
	}
	if err := config.Set().Update().Told(v, now); err != nil {
		slog.Error("saving the update notice failed", "err", err)
		return
	}
	slog.Info("saying an update is ready", "version", v, "again", v == c.Told)
	say(notice(v, auto))
}

// notice is what is said: the version, and what happens next.
func notice(version string, auto bool) string {
	if auto {
		return i18n.F("An update for this Show is ready, version {version}. It installs by itself tonight.", "version", spoken(version))
	}
	return i18n.F("An update for this Show is ready, version {version}. You can install it in Settings, under Updates.", "version", spoken(version))
}

// spoken is a version as it reads aloud: "1.0.1 dev 6", not "v1.0.1-dev.6".
func spoken(v string) string {
	v = strings.TrimPrefix(v, "v")
	core, pre, ok := strings.Cut(v, "-")
	if !ok {
		return core
	}
	return core + " " + strings.ReplaceAll(pre, ".", " ")
}
