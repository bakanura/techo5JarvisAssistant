package config

import (
	"os"
	"path/filepath"
	"testing"
)

// loadWake writes state as the saved config and loads it.
func loadWake(t *testing.T, state string) (*Store, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(p, []byte(state), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return st, p
}

// A Hey Jarvis slot nobody tuned, saved at the first default of 0.85, moves to the current default
// once; a threshold somebody set stays; 0.85 chosen after the move stays 0.85; and a new device starts
// on the current one.
func TestWakeCutoffSettledFromTheFirstDefault(t *testing.T) {
	st, p := loadWake(t, `{"wake":{"words":[{"id":"hey_jarvis","threshold":0.85},{"id":"hey_jarvis","threshold":0.8},{"id":"okay_nabu","threshold":0.85}]}}`)
	w := st.Get().Wake.Words
	if w[0].Threshold != DefaultThreshold || w[1].Threshold != 0.8 || w[2].Threshold != 0.85 {
		t.Fatalf("after the move: %+v", w)
	}
	if err := st.Update(func(c *Config) { c.Wake.Words[0].Threshold = 0.85 }); err != nil {
		t.Fatal(err)
	}
	again, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Get().Wake.Words[0].Threshold; got != 0.85 {
		t.Errorf("0.85 chosen after the move came back as %v", got)
	}

	fresh, err := Load(filepath.Join(t.TempDir(), "none.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := fresh.Get().Wake.Slot(0).Threshold; got != DefaultThreshold || !fresh.Get().Wake.CutoffSettled {
		t.Errorf("a new device starts at %v", got)
	}
}

// A device that already took the raise to 0.92 comes back down to the default, once, and a 0.85
// somebody set on it in between stays.
func TestWakeCutoffSettledFromTheRaise(t *testing.T) {
	st, p := loadWake(t, `{"wake":{"words":[{"id":"hey_jarvis","threshold":0.92},{"id":"hey_jarvis","threshold":0.85}],"cutoff_raised":true}}`)
	w := st.Get().Wake.Words
	if w[0].Threshold != DefaultThreshold || w[1].Threshold != 0.85 {
		t.Fatalf("after the move: %+v", w)
	}
	if err := st.Update(func(c *Config) { c.Wake.Words[0].Threshold = 0.92 }); err != nil {
		t.Fatal(err)
	}
	again, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Get().Wake.Words[0].Threshold; got != 0.92 {
		t.Errorf("0.92 chosen after the move came back as %v", got)
	}
}
