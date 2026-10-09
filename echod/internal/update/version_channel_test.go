package update

import "testing"

// CI builds rank by their run number whatever channel they name, which is how Home Assistant ranks
// them too, so the card and the device agree. Text order would put stable below staging.
func TestChannelBuildsRankByNumber(t *testing.T) {
	for _, c := range []struct {
		offered, running string
		want             bool
	}{
		{"v1.0.1-stable.10", "v1.0.1-staging.9", true},
		{"v1.0.1-staging.9", "v1.0.1-stable.10", false},
		{"v1.0.1-staging.10", "v1.0.1-dev.9", true},
		{"v1.0.1-dev.11", "v1.0.1-staging.10", true},
		{"v1.0.1-stable.10", "v1.0.1-dev.12", false},
		{"v1.0.1-stable.10", "v1.0.1-stable.9", true},
		{"v1.0.1-dev.12.2", "v1.0.1-dev.12", true},
		{"v1.0.1-stable.12", "v1.0.1-dev.12", true},
		{"v1.0.2-dev.1", "v1.0.1-stable.40", true},
		{"v1.0.1", "v1.0.1-stable.40", true},
		{"v1.0.1-rc.1", "v1.0.1-dev.9", true},
	} {
		if got := Newer(c.offered, c.running); got != c.want {
			t.Errorf("Newer(%s, %s) = %v, want %v", c.offered, c.running, got, c.want)
		}
	}
}
