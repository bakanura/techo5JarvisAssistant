package i18n

import "testing"

func TestBase(t *testing.T) {
	for in, want := range map[string]string{"de": "de", "de-CH": "de", "pt_BR": "pt", " EN-us ": "en", "": ""} {
		if got := base(in); got != want {
			t.Errorf("base(%q) = %q, want %q", in, got, want)
		}
	}
}
