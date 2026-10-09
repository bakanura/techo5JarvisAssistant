// Package sysclock sets the system clock for the places that learn the time some other way than NTP:
// Home Assistant's answer, and the update server's. NTP is how the device keeps time; these are for a
// network where it cannot, which would otherwise leave the clock years behind and every certificate
// check failing.
package sysclock

import (
	"os/exec"
	"syscall"
	"time"
)

// Unset is whether the clock is plainly wrong: this kernel starts in 1970, and the RTC, when it kept
// anything, starts it years back. Nothing that runs this was built before 2025.
func Unset() bool { return time.Now().Year() < 2025 }

// Set sets the clock, and writes it to the RTC so the next boot starts from a date that is at least
// close.
func Set(t time.Time) error {
	tv := syscall.NsecToTimeval(t.UnixNano())
	if err := syscall.Settimeofday(&tv); err != nil {
		return err
	}
	_ = exec.Command("hwclock", "-w").Run()
	return nil
}
