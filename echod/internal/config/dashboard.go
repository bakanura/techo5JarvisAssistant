package config

// Dashboard is a Home Assistant dashboard on the screen: drawn by the device itself from the
// dashboard's cards, or streamed as pictures from a dashcast server that runs a browser for it.
type Dashboard struct {
	// Mode is how it is shown, one of the DashboardModes; empty is off.
	Mode DashboardMode `json:"mode,omitempty"`

	// Path is the dashboard, as Home Assistant's own address bar has it ("lovelace/0",
	// "dashboard-kitchen/lights"). Empty is the Rooms dashboard when drawn here, and Home
	// Assistant's default one when streamed.
	Path string `json:"path,omitempty"`

	// Server is the dashcast server's address, host:port, and Key what it asks a device for.
	Server string `json:"server,omitempty"`
	Key    string `json:"key,omitempty"`

	// Idle shows the dashboard in place of the clock while nothing else is on the screen, for a
	// screen that is there to show how the house is.
	Idle bool `json:"idle,omitempty"`

	// Kiosk has a streamed dashboard shown without Home Assistant's top bar, which on a small screen
	// is a real slice of it. Only dashcast can do it; a drawn dashboard has no bar to hide.
	Kiosk bool `json:"kiosk,omitempty"`

	// OwnUser has a streamed dashboard signed in as this device's own Home Assistant user, the one
	// whose token it was given, rather than as dashcast's: what it can see is then what that user
	// can. The token goes to dashcast inside the encrypted connection, and a dashcast from before
	// it knew of this ignores it.
	OwnUser bool `json:"own_user,omitempty"`

	// Room is the dashboard for the room the device is in, the page a swipe in from the left goes to
	// from the Show's own dashboard. Empty finds it: a dashboard or a view named after the device's
	// area in Home Assistant. RoomNone is none at all. Nothing is ever made for it here.
	Room string `json:"room,omitempty"`

	// Known is the dashboards Home Assistant had when last asked, kept so the list of them is there
	// from the start and not only once Home Assistant has been asked again.
	Known []DashboardChoice `json:"known,omitempty"`
}

// DashboardChoice is one dashboard view: how a list names it, and its path.
type DashboardChoice struct {
	Label    string `json:"label"`
	Path     string `json:"path"`
	Streamed bool   `json:"streamed,omitempty"` // a built-in page only a browser can show
}

// RoomNone is the Room that turns the room's dashboard off, rather than finding one.
const RoomNone = "none"

type DashboardMode string

const (
	DashboardOff      DashboardMode = ""
	DashboardDrawn    DashboardMode = "drawn"
	DashboardStreamed DashboardMode = "streamed"
)

// DashboardModes is every mode, in the order a list shows them.
func DashboardModes() []DashboardMode {
	return []DashboardMode{DashboardOff, DashboardDrawn, DashboardStreamed}
}

// Label is how Home Assistant and the screen name a mode.
func (m DashboardMode) Label() string {
	switch m {
	case DashboardDrawn:
		return "Drawn on the device"
	case DashboardStreamed:
		return "Streamed"
	}
	return "Off"
}

func defaultDashboard() Dashboard {
	return Dashboard{Mode: DashboardStreamed, Path: "jarvis-display", Idle: true, Kiosk: true}
}

type DashboardWriter struct{ st *Store }

func (w DashboardWriter) Mode(v DashboardMode) error {
	return w.st.Update(func(c *Config) { c.Dashboard.Mode = v })
}

func (w DashboardWriter) Path(v string) error {
	return w.st.Update(func(c *Config) { c.Dashboard.Path = v })
}

// Server sets where the dashcast server is and its key together, since one is no use without the
// other.
func (w DashboardWriter) Server(addr, key string) error {
	return w.st.Update(func(c *Config) { c.Dashboard.Server, c.Dashboard.Key = addr, key })
}

func (w DashboardWriter) Kiosk(v bool) error {
	return w.st.Update(func(c *Config) { c.Dashboard.Kiosk = v })
}

func (w DashboardWriter) OwnUser(v bool) error {
	return w.st.Update(func(c *Config) { c.Dashboard.OwnUser = v })
}

func (w DashboardWriter) Idle(v bool) error {
	return w.st.Update(func(c *Config) { c.Dashboard.Idle = v })
}

func (w DashboardWriter) Known(v []DashboardChoice) error {
	return w.st.Update(func(c *Config) { c.Dashboard.Known = v })
}

func (w DashboardWriter) Room(v string) error {
	return w.st.Update(func(c *Config) { c.Dashboard.Room = v })
}
