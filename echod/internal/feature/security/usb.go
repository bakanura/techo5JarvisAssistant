package security

import (
	"log/slog"
	"os"
	"os/exec"

	"github.com/HuskerMinion/techo5/echod/internal/config"
	"github.com/HuskerMinion/techo5/echod/internal/layout"
)

// USB debugging. The port is the easiest way into a device someone can hold: it carries a root
// shell on serial, and anything plugged into it as a host (a keyboard, a stick) would be taken in.
// So both stay shut unless the switch is on. boot.sh decides at boot from usbFlag, before the
// daemon runs; the daemon keeps the flag and switches the port while it runs. The rescue
// environment in the boot image keeps its own serial shell: it is what is left when a system
// does not start.
var (
	usbFlag = layout.StateDir + "/usb_debug" // T5_USB_DEBUG in techo5-lib.sh
	usbTool = "/usr/local/sbin/techo5-usb"
)

// usbAvailable reports whether the port is the daemon's to open and close: a slot boot of a root
// filesystem that has the tool.
func usbAvailable() bool {
	if layout.OnAndroid() {
		return false
	}
	if _, err := os.Stat(slotMarker); err != nil {
		return false
	}
	_, err := os.Stat(usbTool)
	return err == nil
}

// usbInstall is there while the installer watches the first boot over the serial console: it
// writes "install" into the flag, boot.sh opens the port and moves the mark here, and the
// installer closes the port itself once it has seen the daemon start. /run is gone at the next
// boot, so an install cut off halfway doesn't keep the port open.
var usbInstall = "/run/techo5/usb_install"

// settleUSB sets the port to the switch when the two differ, and once at start: boot.sh went by the
// flag, and the flag may be older than the setting. While the installer's flag is there the port is
// left as it is, until somebody flips the switch.
func (f *Feature) settleUSB() {
	if !usbAvailable() {
		return
	}
	if !f.usbTouched.Load() && installing() {
		return
	}
	want := config.Get().Security.USB
	if f.usbApplied != nil && *f.usbApplied == want {
		return
	}
	if err := writeUSBFlag(want); err != nil {
		slog.Error("usb: keeping the flag for the next boot failed", "err", err)
	}
	arg := "off"
	if want {
		arg = "on"
	}
	if out, err := exec.Command(usbTool, arg).CombinedOutput(); err != nil {
		slog.Error("usb: switching the port failed", "to", arg, "err", err, "out", string(out))
		return
	}
	slog.Info("usb: debugging", "on", want)
	f.usbApplied = &want
	f.Changed.Emit(struct{}{})
}

func installing() bool {
	_, err := os.Stat(usbInstall)
	return err == nil
}

func writeUSBFlag(on bool) error {
	if !on {
		if err := os.Remove(usbFlag); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return os.WriteFile(usbFlag, []byte("on\n"), 0o600)
}
