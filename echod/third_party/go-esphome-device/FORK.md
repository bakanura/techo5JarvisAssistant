# go-esphome-device, OpenJade's copy

A copy of github.com/ygelfand/go-esphome-device v0.0.4 (MIT, LICENSE), used by echod through a
`replace` in echod/go.mod. The one change: `Info` has `ProjectName` and `ProjectVersion`, and the
device info answer sends them, so Home Assistant can show what firmware runs on the device.
