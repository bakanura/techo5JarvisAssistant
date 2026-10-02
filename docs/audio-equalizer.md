# Jarvis Show audio equalizer

Jarvis Show keeps TECHO5's vendor-aware speaker tuning and its existing Home Assistant entities:

- `Speaker EQ`
- `Bass` (-6..+6 dB)
- `Treble` (-6..+6 dB)

Bass and treble are listener shelves in front of the vendor protection/tuning chain. They are not a
replacement EQ implementation and they do not bypass the driver's protection.

## One state path

Touchscreen controls, Home Assistant number entities and Direct Brain all use the same persisted tone
state and the same live DSP setter. Both shelves are stored atomically so the exported state cannot
briefly describe a different EQ from the running speaker.

## Direct Brain tools

Direct Brain exposes:

- `set_speaker_eq`
- `set_equalizer`
- `adjust_equalizer`
- `reset_equalizer`

Examples:

- `Mehr Bass` -> `adjust_equalizer(bass_delta_db=+1)`
- `Weniger Höhen` -> `adjust_equalizer(treble_delta_db=-1)`
- `Bass auf plus drei dB` -> `set_equalizer(bass_db=3)`
- `Equalizer zurücksetzen` -> both listener shelves return to 0 dB

A bass/treble request enables Speaker EQ first so it has an audible effect. If the underlying tuned DSP
cannot be enabled, Direct Brain reports failure instead of claiming the change was applied.
