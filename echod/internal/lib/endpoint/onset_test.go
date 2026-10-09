package endpoint

import "testing"

func opens(parts ...[]int16) (bool, int) {
	var o Onset
	var at int
	for _, p := range parts {
		for i := 0; i < len(p); i += Window {
			if o.Feed(p[i : i+Window]) {
				return true, at + i/Window + 1
			}
		}
		at += len(p) / Window
	}
	return false, 0
}

// Somebody speaking up after a quiet room opens it, within the two windows it takes to be sure.
func TestOnsetOpensOnSpeechOverAQuietRoom(t *testing.T) {
	ok, at := opens(tone(20, 120), tone(10, 1200))
	if !ok || at != 22 {
		t.Fatalf("opened %v at window %d, want true at 22", ok, at)
	}
}

// Six seconds of an empty room is nothing, however the leveler has it.
func TestOnsetStaysShutInSilence(t *testing.T) {
	if ok, _ := opens(tone(60, 120)); ok {
		t.Fatal("a quiet room opened it")
	}
}

// The tail of the reply is loud for a moment and then gone; the room is what comes after it.
func TestOnsetIsNotOpenedByTheReplysTail(t *testing.T) {
	if ok, _ := opens(tone(1, 3000), tone(40, 120)); ok {
		t.Fatal("the reply's tail opened it")
	}
	if ok, _ := opens(tone(1, 3000), tone(20, 120), tone(5, 1200)); !ok {
		t.Fatal("speech after the reply's tail did not open it")
	}
}

// Music that was already playing becomes the room, even with its loud bars; a voice over it is still heard.
func TestOnsetTakesSteadyMusicForTheRoom(t *testing.T) {
	var song [][]int16
	for range 15 {
		song = append(song, tone(3, 700), tone(1, 1100))
	}
	if ok, _ := opens(song...); ok {
		t.Fatal("steady music opened it")
	}
	if ok, _ := opens(append(song, tone(5, 3500))...); !ok {
		t.Fatal("a voice over the music did not open it")
	}
}

// One loud window, a clap or a door, is not somebody talking.
func TestOnsetIgnoresOneLoudWindow(t *testing.T) {
	if ok, _ := opens(tone(10, 120), tone(1, 3000), tone(20, 120)); ok {
		t.Fatal("a single loud window opened it")
	}
}
