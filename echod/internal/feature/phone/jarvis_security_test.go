package phone

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HuskerMinion/techo5/echod/internal/config"
)

func TestJarvisAccountCannotPersistPlaintextMode(t *testing.T) {
	old := accountPath
	accountPath = filepath.Join(t.TempDir(), "phone.json")
	t.Cleanup(func() { accountPath = old })

	if err := saveAccount(Account{Server: "sip.example", Username: "100", Password: "secret", Plain: true}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(accountPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"plain":true`) {
		t.Fatalf("plaintext downgrade survived save: %s", b)
	}
	a, err := loadAccount()
	if err != nil {
		t.Fatal(err)
	}
	if a.Plain {
		t.Fatal("plaintext downgrade survived reload")
	}
}

func TestSafeCallerTextBoundsAndStripsControls(t *testing.T) {
	got := safeCallerText("  Alice\x1b[2J\n" + strings.Repeat("x", 100))
	if strings.ContainsAny(got, "\x1b\n\r") {
		t.Fatalf("control text survived: %q", got)
	}
	if len([]rune(got)) > callerTextMost {
		t.Fatalf("caller text kept %d runes, want <= %d", len([]rune(got)), callerTextMost)
	}
}

func TestDoNotDisturbTurnsAwayIncomingSIPCalls(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	if !incomingCallsAllowed() {
		t.Fatal("incoming calls started disabled")
	}
	if err := config.Set().Home().DoNotDisturb(true); err != nil {
		t.Fatal(err)
	}
	if incomingCallsAllowed() {
		t.Fatal("incoming SIP calls remained allowed under DND")
	}
}
