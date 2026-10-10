package config

// Security is what the device opens to the network besides its link to Home Assistant: an SSH
// server, and the camera and screen pages on the web port. Home Assistant's own link is encrypted
// with the device key and is not a setting.
type Security struct {
	SSH    bool `json:"ssh"`
	Camera bool `json:"camera_web"`
	Screen bool `json:"screen_web"`

	// USB is USB debugging: the root shell on the USB serial port, and USB devices let in. Off, the
	// port shows a computer nothing and takes nothing plugged into it.
	USB bool `json:"usb_debug"`
}

// Everything closed on a device nobody has set: SSH has no key until Home Assistant sends one, and
// the web pages have no login, so they are for someone who asked for them.
func defaultSecurity() Security { return Security{} }

type SecurityWriter struct{ st *Store }

func (w SecurityWriter) SSH(v bool) error {
	return w.st.Update(func(c *Config) { c.Security.SSH = v })
}

func (w SecurityWriter) Camera(v bool) error {
	return w.st.Update(func(c *Config) { c.Security.Camera = v })
}

func (w SecurityWriter) Screen(v bool) error {
	return w.st.Update(func(c *Config) { c.Security.Screen = v })
}

func (w SecurityWriter) USB(v bool) error {
	return w.st.Update(func(c *Config) { c.Security.USB = v })
}
