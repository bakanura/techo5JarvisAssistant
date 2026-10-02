//go:build !dot && !spot

package update

// defaultReleases is the official Jarvis Show release namespace. `releases` is a variable rather
// than a const so release builds can point a development/mirror build somewhere else with Go's
// -ldflags -X without adding a runtime setting that could aim a root updater at arbitrary software.
const defaultReleases = "https://github.com/bakanura/techo5JarvisAssistant/releases"

var releases = defaultReleases
