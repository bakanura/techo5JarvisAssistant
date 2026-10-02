# Jarvis Crown night mode and Do Not Disturb contract

Night mode and Do Not Disturb are deliberately separate controls.

## Night mode

Night mode owns only the screen/brightness behavior. It may turn the panel off or leave the selected
night-light/red-clock presentation running. Normal music may continue without keeping the display at
full daytime brightness.

A user interaction or explicit maintenance surface (Wi-Fi/setup) remains usable. Incoming phone calls
and ringing alarms/timers wake a dark panel; at night their backlight is capped to a gentler level.
Sunrise alarms retain their own gradual brightness curve and override that cap while active.

When an alarm/timer ends, the normal night policy takes over again. A reminder alone does not light a
sleeping room, but remains visible once the screen is deliberately woken.

## Do Not Disturb

DND is the proactive-communication/privacy boundary. While enabled:

- incoming SIP calls are turned away;
- intercom and Drop In are turned away;
- house announcements remain visual but make no sound;
- Home Assistant proactive announcements/questions do not play and cannot open the microphone;
- doorbell camera popups remain visible, but their local cue/camera audio are suppressed.

DND deliberately does **not** suppress:

- alarms;
- timers;
- explicit local interactions;
- outgoing calls;
- replies to a request the user initiated.

Night mode does not silently enable or disable DND. Home Assistant may automate both together if the
house wants a bedtime routine, while the device keeps their semantics independent and predictable.
