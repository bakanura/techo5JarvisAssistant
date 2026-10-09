package home

import "testing"

// A name given in Home Assistant is taken once. Taken, or renamed away from on the setup page, it is not
// taken again, since the device cannot clear it in Home Assistant.
func TestNameToTake(t *testing.T) {
	for _, c := range []struct{ inHA, seen, current, want string }{
		{"Kitchen", "", "Jarvis Show 5", "Kitchen"},
		{"Kitchen", "", "Kitchen", ""},
		{"Kitchen", "Kitchen", "Study", ""},
		{"Hallway", "Kitchen", "Study", "Hallway"},
		{"", "Kitchen", "Kitchen", ""},
		{"a name that is very much too long to fit on the screen", "", "Study", ""},
	} {
		if got := nameToTake(c.inHA, c.seen, c.current); got != c.want {
			t.Errorf("Home Assistant %q, seen %q, called %q: took %q, want %q", c.inHA, c.seen, c.current, got, c.want)
		}
	}
}
