package question

import (
	"testing"
	"time"
)

func TestReply(t *testing.T) {
	for want, said := range map[Said][]string{
		Yes: {"Yes.", "yeah sure", "Sure!", "OK, do it", "okay go ahead", "yes please", "Install it now.", "Now.",
			"Ja.", "Ja, bitte.", "Klar!", "Gerne", "Mach das.", "Ja, installier es jetzt", "auf jeden Fall", "na klar",
			"Ja, das Update ausführen.", "Updaten.", "Führ es aus", "Aktualisieren",
			"Oui", "Sì, certo", "Sí, claro", "Ja graag"},
		No: {"No.", "no thanks", "Not now.", "later", "Maybe later", "don't", "wait",
			"Nein.", "Nein danke", "Jetzt nicht.", "Später", "nee", "Non", "No, dopo", "Nee, straks"},
		Other: {"", "what time is it", "yes and turn on the lights", "play some music", "thanks",
			"Wie spät ist es?", "Mach das Licht an", "Untertitel im Auftrag des ZDF", "Aus", "Update"},
	} {
		for _, s := range said {
			if got := Reply(s); got != want {
				t.Errorf("%q read as %v, want %v", s, got, want)
			}
		}
	}
}

func TestAnswer(t *testing.T) {
	at := time.Date(2026, 10, 9, 20, 0, 0, 0, time.UTC)
	now = func() time.Time { return at }
	defer func() { now = time.Now }()

	answered := make(chan string, 1)
	ask := func() {
		Ask("update", time.Minute, func() { answered <- "yes" }, func() { answered <- "no" })
	}

	if Answer("yes") {
		t.Fatal("answered with no question open")
	}

	ask()
	if Answer("") || Answer("  ") {
		t.Fatal("silence answered the question")
	}
	if !Answer("Ja, bitte") || <-answered != "yes" {
		t.Fatal("yes was not taken")
	}
	if Answer("yes") {
		t.Fatal("the question stayed open after its answer")
	}

	ask()
	if !Answer("not now") || <-answered != "no" {
		t.Fatal("no was not taken")
	}

	// Something else goes on to the assistant, and the question is over.
	ask()
	if Answer("what time is it") || Answer("yes") {
		t.Fatal("the question outlived an unrelated request")
	}

	// Too late.
	ask()
	at = at.Add(2 * time.Minute)
	if Answer("yes") {
		t.Fatal("an answer after the window was taken")
	}

	ask()
	Withdraw("other")
	Withdraw("update")
	if Answer("yes") {
		t.Fatal("a withdrawn question was answered")
	}
	select {
	case a := <-answered:
		t.Fatalf("unexpected answer %q", a)
	default:
	}
}

// In the turn opened for the question, what is not an answer goes nowhere; after a wake word it goes on
// to the assistant. Either way the question is over.
func TestTakeDropsWhatIsNotAnAnswer(t *testing.T) {
	answered := make(chan string, 1)
	ask := func() {
		Ask("update", time.Minute, func() { answered <- "yes" }, func() { answered <- "no" })
	}

	ask()
	if !Take("Untertitel im Auftrag des ZDF", true) {
		t.Fatal("television in the question's own turn went on to the assistant")
	}
	if Take("ja", true) {
		t.Fatal("the question outlived what was dropped")
	}

	ask()
	if Take("Wie spät ist es?", false) {
		t.Fatal("a request after a wake word was dropped")
	}

	ask()
	if !Take("Ja, das Update ausführen.", true) || <-answered != "yes" {
		t.Fatal("yes was not taken in the question's own turn")
	}

	if Take("Wie spät ist es?", true) {
		t.Fatal("a turn with no question open dropped a request")
	}
}
