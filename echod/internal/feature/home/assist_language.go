package home

import (
	"context"
	"log/slog"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/i18n"
	"github.com/HuskerMinion/techo5/echod/internal/lib/hass"
)

// The screen's "Match Assistant" language is whatever Home Assistant's voice pipeline for this device is
// set to. Nobody changes that often, so it is asked every few minutes rather than followed, and a minute
// after the start when Home Assistant was not there yet.
const (
	assistLanguageEvery = 10 * time.Minute
	assistLanguageRetry = time.Minute
)

func (f *Feature) assistLanguageLoop(ctx context.Context) {
	for {
		wait := assistLanguageEvery
		if !refreshAssistLanguage(ctx) {
			wait = assistLanguageRetry
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// refreshAssistLanguage asks Home Assistant for the pipeline's language and hands it to the screen. It
// is false when Home Assistant could not be asked; a language the screen has no words for is still an
// answer, and the screen stays English for it.
func refreshAssistLanguage(ctx context.Context) bool {
	if !hass.Get().Ready() {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	lang, err := hass.Get().AssistLanguage(ctx, ownMAC())
	if err != nil {
		slog.Debug("home: asking Home Assistant for the assistant's language", "err", err)
		return false
	}
	if lang != "" {
		i18n.SetAssistant(lang)
	}
	return true
}
