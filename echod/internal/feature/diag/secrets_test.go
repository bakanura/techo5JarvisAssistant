package diag

import (
	"strings"
	"testing"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/lib/redact"
)

func TestRegisterSecretsRemovesExactCredentialValues(t *testing.T) {
	c := config.Config{}
	c.Home.HouseWord = "house-word-123456"
	c.Brain.Key = "brain-bearer-123456"
	c.Home.Reolink.Pass = "camera-password-123456"
	c.Dashboard.Key = "dashcast-key-123456"
	c.Calendar.Links = []config.CalendarLink{{URL: "https://calendar.invalid/private-token-123456"}}

	r := redact.New()
	registerSecrets(r, c, "api-psk-123456", []byte(`{"url":"https://ha.invalid","token":"ha-token-123456"}`))
	in := strings.Join([]string{
		c.Home.HouseWord,
		c.Brain.Key,
		c.Home.Reolink.Pass,
		c.Dashboard.Key,
		c.Calendar.Links[0].URL,
		"api-psk-123456",
		"ha-token-123456",
	}, "\n")
	got := r.Text(in)
	for _, secret := range strings.Split(in, "\n") {
		if strings.Contains(got, secret) {
			t.Fatalf("diagnostics still contain secret %q in %q", secret, got)
		}
	}
}
