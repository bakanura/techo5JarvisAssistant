package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFreshDeviceGetsJarvisMusicStripDefault(t *testing.T) {
	st, err := Load(filepath.Join(t.TempDir(), "missing-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := st.Get().Screen.MusicStrip; got != DefaultMusicStripSeconds {
		t.Fatalf("fresh music strip = %d seconds, want %d", got, DefaultMusicStripSeconds)
	}
}

func TestExistingDeviceWithoutMusicStripKeepsFullPage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"speaker":{"volume":3},"screen":{"brightness":55}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := st.Get().Screen.MusicStrip; got != 0 {
		t.Fatalf("old config music strip = %d seconds, want legacy full-page 0", got)
	}
}

func TestExistingMusicStripChoiceWins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"screen":{"music_strip":45}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := st.Get().Screen.MusicStrip; got != 45 {
		t.Fatalf("saved music strip = %d seconds, want 45", got)
	}
}
