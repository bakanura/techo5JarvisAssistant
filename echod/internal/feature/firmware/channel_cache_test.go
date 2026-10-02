package firmware

import (
	"testing"

	"github.com/HuskerMinion/techo5/echod/internal/update"
)

func TestCachedManifestNeverCrossesUpdateChannels(t *testing.T) {
	u := &Firmware{}
	dev := update.Manifest{Version: "v1.2.0-dev.4"}
	u.rememberFound(update.Dev, dev)
	if got, ok := u.cachedFound(update.Dev); !ok || got.Version != dev.Version {
		t.Fatalf("dev cache = %+v, %v", got, ok)
	}
	if got, ok := u.cachedFound(update.Stable); ok || got.Version != "" {
		t.Fatalf("dev manifest leaked into stable cache: %+v, %v", got, ok)
	}

	u.clearFound(update.Stable)
	if _, ok := u.cachedFound(update.Dev); ok {
		t.Fatal("switching to stable left the old dev offer installable")
	}
}
