package display

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

func TestInFontLeavesNoBoxes(t *testing.T) {
	f, err := opentype.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: 34, DPI: 72})
	if err != nil {
		t.Fatal(err)
	}
	for in, want := range map[string]string{
		"Reason That I Sing":  "Reason That I Sing",
		"Un‐Break My Heart":   "Un-Break My Heart",
		"Mýa":                 "Mýa",
		"Caf\u00e9 \uff21BBA": "Café ABBA",
		"Song \U0001F3B5":     "Song ",
		"a​b":                 "ab",
	} {
		got := inFont(face, in)
		if got != want {
			t.Errorf("inFont(%q) = %q, want %q", in, got, want)
		}
		for _, c := range got {
			if !hasGlyph(face, c) {
				t.Errorf("inFont(%q) kept %q, which the font has no glyph for", in, c)
			}
		}
	}
}

func TestSongParts(t *testing.T) {
	for _, c := range []struct{ title, song, version string }{
		{"My Love Is Like...Wo (All Star Mix – Main Pass)", "My Love Is Like...Wo", "All Star Mix – Main Pass"},
		{"Bohemian Rhapsody - Remastered 2011", "Bohemian Rhapsody", "Remastered 2011"},
		{"Song (feat. Somebody) [Radio Edit]", "Song", "feat. Somebody  ·  Radio Edit"},
		{"Symphony No. 5 (Part 2)", "Symphony No. 5 (Part 2)", ""},
		{"(Remix)", "(Remix)", ""},
		{"Reason That I Sing", "Reason That I Sing", ""},
	} {
		song, version := songParts(c.title)
		if song != c.song || version != c.version {
			t.Errorf("songParts(%q) = %q, %q; want %q, %q", c.title, song, version, c.song, c.version)
		}
	}
}
