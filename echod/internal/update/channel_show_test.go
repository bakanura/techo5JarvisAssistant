//go:build !dot && !spot

package update

import "testing"

func TestJarvisShowReleaseChannels(t *testing.T) {
	old := releases
	t.Cleanup(func() { releases = old })
	releases = defaultReleases
	if got, want := Stable.URL(), "https://github.com/vardstein/techo5JarvisAssistant/releases/latest/download/manifest.json"; got != want {
		t.Fatalf("stable URL = %q, want %q", got, want)
	}
	if got, want := Staging.URL(), "https://github.com/vardstein/techo5JarvisAssistant/releases/download/channel-staging/manifest.json"; got != want {
		t.Fatalf("staging URL = %q, want %q", got, want)
	}
	if got, want := Dev.URL(), "https://github.com/vardstein/techo5JarvisAssistant/releases/download/channel-dev/manifest.json"; got != want {
		t.Fatalf("dev URL = %q, want %q", got, want)
	}
	for _, c := range Channels() {
		if back, ok := map[string]Channel{"stable": Stable, "staging": Staging, "dev": Dev}[c.Label()]; !ok || back != c {
			t.Fatalf("channel %d has label %q, which does not name it", c, c.Label())
		}
	}
}
