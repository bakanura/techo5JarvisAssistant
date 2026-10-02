package mute

import "testing"

func TestRemoteUnmuteRequiresLocalPermissionAndEncryption(t *testing.T) {
	for _, tc := range []struct {
		name               string
		allowed, encrypted bool
		want               bool
	}{
		{"neither", false, false, false},
		{"permission alone", true, false, false},
		{"encryption alone", false, true, false},
		{"both", true, true, true},
	} {
		if got := canRemoteUnmute(tc.allowed, tc.encrypted); got != tc.want {
			t.Errorf("%s: canRemoteUnmute=%v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestMuteStatusIsTruthfulAndPhysicalWins(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		physical, software, refused bool
		want                        string
	}{
		{"live", false, false, false, "unmuted"},
		{"software cut", false, true, false, "muted"},
		{"remote refusal", false, true, true, "remote unmute not permitted"},
		{"physical wins", true, false, true, "physical mute active"},
		{"physical wins over software", true, true, false, "physical mute active"},
	} {
		if got := statusLabel(tc.physical, tc.software, tc.refused); got != tc.want {
			t.Errorf("%s: status=%q, want %q", tc.name, got, tc.want)
		}
	}
}
