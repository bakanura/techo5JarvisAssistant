package display

import (
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"golang.org/x/image/font"
	"golang.org/x/text/unicode/norm"
)

// inFont is s with every character the face has no glyph for put another way, so a song or a place
// never shows the font's empty box. Song titles from Music Assistant and Home Assistant's names bring
// characters the Go fonts lack: the typographer's hyphen in "Un‐Break My Heart", narrow spaces,
// ligatures, emoji. A character becomes its plain lookalike, else its letter without the accent, else
// it goes; a letter of a script the fonts don't have at all becomes "?" so a word stays a word.
func inFont(face font.Face, s string) string {
	if face == nil || isASCII(s) {
		return s
	}
	var b strings.Builder
	changed := false
	for i, c := range s {
		if hasGlyph(face, c) {
			if changed {
				b.WriteRune(c)
			}
			continue
		}
		if !changed {
			b.WriteString(s[:i])
			changed = true
		}
		b.WriteString(standIn(face, c))
	}
	if !changed {
		return s
	}
	return b.String()
}

// standIn is what is drawn for c, which the face has no glyph for.
func standIn(face font.Face, c rune) string {
	if alt, ok := lookalikes[c]; ok && allGlyphs(face, alt) {
		return alt
	}
	switch {
	case unicode.IsSpace(c):
		return " "
	case unicode.In(c, unicode.Cf, unicode.Mn, unicode.Me, unicode.Variation_Selector):
		return "" // invisible joiners and marks
	}
	// "ﬁ" is "fi", a full-width "Ａ" is "A", "ǅ" is "Dž"; an accent the font can't put on is left off.
	var b strings.Builder
	for _, d := range norm.NFKD.String(string(c)) {
		if d != c && hasGlyph(face, d) {
			b.WriteRune(d)
		}
	}
	if b.Len() > 0 {
		return b.String()
	}
	if unicode.IsLetter(c) || unicode.IsNumber(c) {
		return "?"
	}
	return "" // a symbol or an emoji: nothing is better than a box
}

// lookalikes are the plain characters that read the same as ones the fonts may lack. Each is used
// only when the face lacks the original and has the lookalike.
var lookalikes = map[rune]string{
	'‐': "-", '‑': "-", '‒': "–", '–': "-", '—': "–", '―': "–",
	'⁃': "-", '−': "-", '﹣': "-", '－': "-",
	'‘': "'", '’': "'", '‚': "'", '‛': "'", '′': "'", 'ʼ': "'",
	'“': `"`, '”': `"`, '„': `"`, '‟': `"`, '″': `"`, '«': `"`, '»': `"`,
	'‹': "'", '›': "'",
	'…': "...", '•': "·", '‧': "·", '・': "·", '∙': "·",
	'⁄': "/", '∕': "/", '×': "x", '✕': "x", '✖': "x",
	'™': "TM", '®': "(R)", '©': "(C)", '№': "No.",
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func allGlyphs(face font.Face, s string) bool {
	for _, c := range s {
		if !hasGlyph(face, c) {
			return false
		}
	}
	return true
}

// hasGlyph says whether face draws c. The answers are kept: a title is measured and drawn many times
// a second while it fits itself to the column.
func hasGlyph(face font.Face, c rune) bool {
	if c < utf8.RuneSelf {
		return true
	}
	k := glyphKey{face, c}
	if v, ok := glyphKnown.Load(k); ok {
		return v.(bool)
	}
	_, ok := face.GlyphAdvance(c)
	glyphKnown.Store(k, ok)
	return ok
}

type glyphKey struct {
	face font.Face
	c    rune
}

var glyphKnown sync.Map // glyphKey → bool
