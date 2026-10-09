package firmware

import (
	"log/slog"
	"strings"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/feature/remind"
	"github.com/HuskerMinion/techo5/echod/internal/i18n"
	"github.com/HuskerMinion/techo5/echod/internal/lib/safe"
)

// Saying that an update is ready. Home Assistant shows its update card, and the screen says so under the
// clock, but nobody opens either to look: the device says it once, out loud, for each new version. Not in
// quiet hours, and not when there is nothing to say it with; the next check tries again, so a version
// found at night is said in the morning.
var (
	say    = func(words string) { safe.Go("update notice", func() { remind.Say(words) }) }
	canSay = remind.CanSay
	quiet  = config.Quiet
)

func (u *Firmware) tell() { u.tellOf(u.Offered()) }

// tellOf says that v is ready, if it has not been said and can be now.
func (u *Firmware) tellOf(v string) {
	if v == "" || v == config.Get().Update.Told || quiet() || !canSay() {
		return
	}
	if err := config.Set().Update().Told(v); err != nil {
		slog.Error("saving the update notice failed", "err", err)
		return
	}
	slog.Info("saying an update is ready", "version", v)
	say(notice(v, u.AutoInstall()))
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
