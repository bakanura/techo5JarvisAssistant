package sendspin

import (
	"net"
	"testing"
)

func mustIP(t *testing.T, s string) net.IP {
	t.Helper()
	ip := net.ParseIP(s)
	if ip == nil {
		t.Fatalf("bad test IP %q", s)
	}
	return ip
}

func TestListenerOnlyAdmitsPairedServerIP(t *testing.T) {
	l := &listener{serverIP: mustIP(t, "10.0.0.40")}
	for _, tc := range []struct {
		remote string
		want   bool
	}{
		{"10.0.0.40:42000", true},
		{"10.0.0.41:42000", false},
		{"garbage", false},
	} {
		if got := l.allowed(tc.remote); got != tc.want {
			t.Fatalf("allowed(%q)=%v want %v", tc.remote, got, tc.want)
		}
	}
}

func TestUnpairedListenerAdmitsNobody(t *testing.T) {
	var l listener
	if l.allowed("10.0.0.40:42000") {
		t.Fatal("unpaired listener admitted a peer")
	}
}

func TestNormalizeServerIP(t *testing.T) {
	if got, err := normalizeServerIP(" 10.0.0.40 "); err != nil || got != "10.0.0.40" {
		t.Fatalf("normalize IPv4 = %q, %v", got, err)
	}
	if got, err := normalizeServerIP(""); err != nil || got != "" {
		t.Fatalf("normalize empty = %q, %v", got, err)
	}
	if _, err := normalizeServerIP("music-assistant.local"); err == nil {
		t.Fatal("hostname accepted; exact source-IP pairing would no longer be deterministic")
	}
}

func TestNormalizeServerIPAcceptsOnlyLiteralAddresses(t *testing.T) {
	for _, good := range []string{"10.0.0.40", " 10.0.0.40 ", "fd00::34"} {
		if got, err := normalizeServerIP(good); err != nil || got == "" {
			t.Errorf("normalizeServerIP(%q) = %q, %v", good, got, err)
		}
	}
	for _, bad := range []string{"music-assistant.local", "https://10.0.0.40", "10.0.0.40:8928", "not-an-ip"} {
		if got, err := normalizeServerIP(bad); err == nil {
			t.Errorf("normalizeServerIP(%q) = %q, want error", bad, got)
		}
	}
	if got, err := normalizeServerIP(" "); err != nil || got != "" {
		t.Errorf("unpair = %q, %v", got, err)
	}
}
