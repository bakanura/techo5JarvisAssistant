package firmware

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/update"
)

// A version is said when it can be, never in quiet hours or with nothing to say it with, and again every
// remindEvery while it waits; with automatic installs on, only once.
func TestUpdateIsSaidAndRepeated(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	oldSay, oldCan, oldQuiet := say, canSay, quiet
	t.Cleanup(func() { say, canSay, quiet = oldSay, oldCan, oldQuiet })
	var said []string
	say = func(words string) { said = append(said, words) }
	able, hush := false, true
	canSay = func() bool { return able }
	quiet = func() bool { return hush }

	u := &Firmware{}
	at := time.Date(2026, 10, 9, 18, 0, 0, 0, time.Local)
	u.tellOf("", at) // nothing offered
	u.tellOf("v1.0.1-dev.6", at)
	if len(said) != 0 {
		t.Fatalf("said %q in quiet hours with nothing to say it with", said)
	}
	hush = false
	u.tellOf("v1.0.1-dev.6", at)
	if len(said) != 0 {
		t.Fatalf("said %q with nothing to say it with", said)
	}
	able = true
	u.tellOf("v1.0.1-dev.6", at)
	u.tellOf("v1.0.1-dev.6", at.Add(time.Hour))
	if len(said) != 1 {
		t.Fatalf("said %d times in the first hour, want once: %q", len(said), said)
	}
	if want := "An update for this Show is ready, version 1.0.1 dev 6. You can install it in Settings, under Updates."; said[0] != want {
		t.Fatalf("said %q, want %q", said[0], want)
	}
	u.tellOf("v1.0.1-dev.6", at.Add(remindEvery))
	if len(said) != 2 {
		t.Fatalf("not said again after %s: %q", remindEvery, said)
	}
	u.tellOf("v1.0.1-dev.7", at.Add(remindEvery+time.Minute))
	if len(said) != 3 {
		t.Fatalf("a newer version was not said: %q", said)
	}

	if err := config.Set().Update().AutoInstall(true); err != nil {
		t.Fatal(err)
	}
	u.tellOf("v1.0.1-dev.7", at.Add(3*remindEvery))
	u.tellOf("v1.0.1-dev.8", at.Add(3*remindEvery))
	u.tellOf("v1.0.1-dev.8", at.Add(5*remindEvery))
	if len(said) != 4 || !strings.Contains(said[3], "tonight") {
		t.Fatalf("with automatic installs, want dev.8 said once and dev.7 not repeated: %q", said)
	}
}

func TestSpokenVersion(t *testing.T) {
	for v, want := range map[string]string{"v1.0.1": "1.0.1", "v1.0.1-dev.6": "1.0.1 dev 6", "v2.0.0-staging.31": "2.0.0 staging 31", "v1.0.1-stable.40": "1.0.1 stable 40"} {
		if got := spoken(v); got != want {
			t.Errorf("spoken(%q) = %q, want %q", v, got, want)
		}
	}
}

// A check with an unset clock is tried again in a minute, not in an hour.
func TestNextCheck(t *testing.T) {
	for _, c := range []struct {
		err  error
		want string
	}{
		{nil, checkEvery.String()},
		{fmt.Errorf("fetch: %w", update.ErrClock), clockRetry.String()},
		{errors.New("no route to host"), failRetry.String()},
	} {
		if got := nextCheck(c.err).String(); got != c.want {
			t.Errorf("nextCheck(%v) = %s, want %s", c.err, got, c.want)
		}
	}
}
