package config

// Diag is how the device reports on itself.
type Diag struct {
	// Interval is how often the readings that drift are collected, in seconds.
	Interval int `json:"interval"`

	// RemoteADB is a legacy decode-only field. Jarvis Show never exposes adbd over the LAN; a
	// saved true value is cleared during diagnostics restore.
	RemoteADB bool `json:"remote_adb"`

	// InsecureTLS is a legacy decode-only field. Jarvis Show never disables certificate checks; a
	// saved true value is cleared during diagnostics restore.
	InsecureTLS bool `json:"insecure_tls"`

	// MinCores holds cores online that the governor would otherwise park.
	MinCores int `json:"min_cores"`
}

// DefaultInterval is five minutes. The readings move slowly and every one of them costs a read of
// sysfs or proc, so this is a compromise between a stale card and a device busying itself for
// nobody.
const DefaultInterval = 300

const DefaultMinCores = 2

func defaultDiag() Diag {
	return Diag{Interval: DefaultInterval, MinCores: DefaultMinCores}
}

type DiagWriter struct{ st *Store }

func (w DiagWriter) Interval(v int) error {
	return w.st.Update(func(c *Config) { c.Diag.Interval = v })
}

// ClearLegacyRemoteADB is intentionally one-way. Older TECHO5 state files may contain the
// network-adb bit; Jarvis Show migrates it off and provides no setter that can turn it back on.
func (w DiagWriter) ClearLegacyRemoteADB() error {
	return w.st.Update(func(c *Config) { c.Diag.RemoteADB = false })
}

// ClearLegacyInsecureTLS is intentionally one-way. Older TECHO5 state files may contain the
// diagnostic certificate-bypass bit; Jarvis Show migrates it off and provides no setter that can
// turn it back on.
func (w DiagWriter) ClearLegacyInsecureTLS() error {
	return w.st.Update(func(c *Config) { c.Diag.InsecureTLS = false })
}

func (w DiagWriter) MinCores(v int) error {
	return w.st.Update(func(c *Config) { c.Diag.MinCores = v })
}
