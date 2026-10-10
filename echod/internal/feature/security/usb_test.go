package security

import (
	"os"
	"path/filepath"
	"testing"
)

// The flag is what boot.sh goes by before the daemon runs, so on and off have to leave the file
// there and gone, and off twice must not be an error.
func TestUSBFlagFollowsTheSwitch(t *testing.T) {
	old := usbFlag
	usbFlag = filepath.Join(t.TempDir(), "usb_debug")
	defer func() { usbFlag = old }()

	if err := writeUSBFlag(true); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(usbFlag); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("on: flag %v, %v", fi, err)
	}
	for range 2 {
		if err := writeUSBFlag(false); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(usbFlag); !os.IsNotExist(err) {
		t.Fatalf("off: flag still there (%v)", err)
	}
}

// The installer's mark keeps the daemon's hands off the port for the first boot.
func TestInstallMarkIsTheInstallers(t *testing.T) {
	old := usbInstall
	usbInstall = filepath.Join(t.TempDir(), "usb_install")
	defer func() { usbInstall = old }()

	if installing() {
		t.Fatal("no mark read as installing")
	}
	if err := os.WriteFile(usbInstall, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if !installing() {
		t.Fatal("the installer's mark was not seen")
	}
}
