package config

// Sendspin is the room's part of a synchronized stream. Jarvis Show deliberately pairs it to one
// Music Assistant server address rather than accepting whichever IoT-LAN peer reaches the port
// first. The server address is provisioned over the encrypted Home Assistant API.
type Sendspin struct {
	Enabled  bool   `json:"enabled"`
	ServerIP string `json:"server_ip,omitempty"`
}

func defaultSendspin() Sendspin { return Sendspin{} }

type SendspinWriter struct{ st *Store }

func (w SendspinWriter) Enabled(v bool) error {
	return w.st.Update(func(c *Config) { c.Sendspin.Enabled = v })
}

func (w SendspinWriter) ServerIP(v string) error {
	return w.st.Update(func(c *Config) { c.Sendspin.ServerIP = v })
}
