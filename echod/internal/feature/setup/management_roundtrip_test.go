package setup

import (
	"net/url"
	"strings"
	"testing"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/feature/wakeword"
)

func TestBrainAndListeningRoundTripThroughHTTP(t *testing.T) {
	f, c := in(t)

	to := post(t, f, c, url.Values{
		"what":     {"brain"},
		"tab":      {"sound"},
		"mode":     {""},
		"stt":      {"10.0.0.20:10300"},
		"tts":      {"10.0.0.20:10200"},
		"voice":    {"de_DE-thorsten-medium"},
		"language": {"de"},
		"llm":      {"https://ollama.example.test/v1/"},
		"model":    {"qwen-test"},
		"search":   {"http://10.0.0.30:8888/"},
		"prompt":   {"Keep answers concise."},
		"key":      {"test-key-not-a-real-secret"},
	})
	if to.Query().Get("problem") != "" || to.Query().Get("tab") != "sound" {
		t.Fatalf("brain save returned %s", to)
	}
	b := config.Get().Brain
	if b.Mode != config.BrainHomeAssistant || b.Language != "de" || b.Model != "qwen-test" ||
		b.LLM != "https://ollama.example.test/v1" || b.Search != "http://10.0.0.30:8888" ||
		b.Key != "test-key-not-a-real-secret" {
		t.Fatalf("brain did not round-trip: %+v", b)
	}
	body := get(f, "/setup?tab=sound", c).Body.String()
	for _, want := range []string{"qwen-test", "de_DE-thorsten-medium", "set; leave empty to keep it"} {
		if !strings.Contains(body, want) {
			t.Errorf("sound tab lost %q after save", want)
		}
	}
	if strings.Contains(body, "test-key-not-a-real-secret") {
		t.Error("brain key was rendered back into the setup page")
	}

	to = post(t, f, c, url.Values{
		"what":      {"listening"},
		"tab":       {"sound"},
		"followup":  {"6"},
		"followups": {"2"},
	})
	if to.Query().Get("problem") != "" {
		t.Fatalf("listening save returned %s", to)
	}
	if got := int(wakeword.FollowUp(0).Seconds()); got != 6 {
		t.Errorf("follow-up seconds = %d, want 6", got)
	}
	if got := wakeword.FollowUps(0); got != 2 {
		t.Errorf("follow-up count = %d, want 2", got)
	}
}

func TestAutomaticUpdatesRoundTripThroughHTTP(t *testing.T) {
	f, c := in(t)
	to := post(t, f, c, url.Values{
		"what": {"update-auto"},
		"tab":  {"general"},
		"auto": {"yes"},
	})
	if to.Query().Get("problem") != "" {
		t.Fatalf("update-auto save returned %s", to)
	}
	if !config.Get().Update.AutoInstall {
		t.Fatal("automatic updates were not persisted")
	}
	body := get(f, "/setup?tab=general", c).Body.String()
	if !strings.Contains(body, `name="auto" value="yes" checked`) {
		t.Error("General did not render the persisted automatic-update state")
	}
}
