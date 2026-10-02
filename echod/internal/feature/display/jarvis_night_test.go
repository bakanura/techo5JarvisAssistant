//go:build !dot && !spot

package display

import "testing"

func TestRingingAlwaysWakesADarkScreen(t *testing.T) {
	if !shouldWakeDarkScreen(false, true, false, true) {
		t.Fatal("a ringing alarm/timer left the panel dark at night")
	}
	if !shouldWakeDarkScreen(false, true, false, false) {
		t.Fatal("a ringing alarm/timer left the panel dark by day")
	}
}

func TestReminderAloneDoesNotWakeSleepingRoom(t *testing.T) {
	if shouldWakeDarkScreen(false, false, true, true) {
		t.Fatal("a reminder alone lit a dark panel at night")
	}
	if !shouldWakeDarkScreen(false, false, true, false) {
		t.Fatal("a daytime reminder did not wake the panel")
	}
}
