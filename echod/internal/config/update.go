package config

import "time"

// Update is which releases the device follows. The channel is a name rather than a URL: where each
// one points is compiled in, so this cannot be used to send the device somewhere else for its next
// binary.
type Update struct {
	Channel string `json:"channel"`

	// LastVersion is the build Home Assistant was last told about. It moves only once the telling has
	// happened, so a version that changed while nothing was listening is still reported later.
	LastVersion string `json:"last_version"`

	// AutoInstall installs an update the channel offers by itself, overnight while nothing is playing,
	// for a device nobody is going to press Install on: one with no Home Assistant, far away.
	AutoInstall bool `json:"auto_install,omitempty"`

	// Told is the last update the device said out loud was ready, and ToldAt when it last said so: it
	// says it again every few hours until the update is installed.
	Told   string    `json:"told,omitempty"`
	ToldAt time.Time `json:"told_at,omitzero"`
}

const DefaultChannel = "stable"

func defaultUpdate() Update {
	return Update{Channel: DefaultChannel}
}

type UpdateWriter struct{ st *Store }

func (w UpdateWriter) Channel(v string) error {
	return w.st.Update(func(c *Config) { c.Update.Channel = v })
}

func (w UpdateWriter) AutoInstall(v bool) error {
	return w.st.Update(func(c *Config) { c.Update.AutoInstall = v })
}

func (w UpdateWriter) Told(v string, at time.Time) error {
	return w.st.Update(func(c *Config) { c.Update.Told, c.Update.ToldAt = v, at })
}

func (w UpdateWriter) LastVersion(v string) error {
	return w.st.Update(func(c *Config) { c.Update.LastVersion = v })
}
