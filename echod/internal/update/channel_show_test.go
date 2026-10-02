//go:build !dot && !spot

package update

import "testing"

func TestJarvisShowReleaseChannels(t *testing.T) {
	old := releases
	t.Cleanup(func() { releases = old })
	releases = defaultReleases
	if got, want := Stable.URL(), "https://github.com/bakanura/techo5JarvisAssistant/releases/latest/download/manifest.json"; got != want {
		t.Fatalf("stable URL = %q, want %q", got, want)
	}
	if got, want := Dev.URL(), "https://github.com/bakanura/techo5JarvisAssistant/releases/download/dev/manifest.json"; got != want {
		t.Fatalf("dev URL = %q, want %q", got, want)
	}
}
