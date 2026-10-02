# Jarvis Crown camera, doorbell and proactive conversation contract

Jarvis Crown keeps camera/doorbell behavior on the existing Home Assistant + TECHO5 surfaces rather
than giving the device camera credentials of its own.

## Camera and doorbell actions

- `home_show_camera` shows a configured `camera.*` entity for the requested time.
- `home_show_camera_sound` does the same and may ask Home Assistant to route that camera's audio to
  this Crown media player.
- `home_doorbell` is the proactive front-door path. It immediately puts the camera on screen and
  gives a short local doorbell cue. Under DND/quiet hours the visual popup still appears, but the cue
  and camera audio are suppressed.
- All HA-triggered camera/list/doorbell actions require the encrypted ESPHome API link. They fail
  closed while the device still has the all-zero adoption key.
- Camera entity IDs are restricted to `camera.*` (plus the device-local camera internally), and
  names shown on screen are printable and length-bounded.

Opening a doorbell view never opens the microphone. If an automation wants an Alexa-like question
after an announcement (for example, "Someone is at the door. Do you want to see them?"), use Home
Assistant Assist Satellite's announcement/question path with `StartConversation`. Jarvis Crown's DND
privacy rule suppresses that proactive microphone opening.

## Screen ownership

Calls and ringing alarms remain above camera views. A camera is above the Jarvis dashboard while it
is active. The dashboard request/state is retained behind it, so dismissing the camera or reaching
its timeout returns to the same Jarvis dashboard automatically.

The camera's bottom control can mute/unmute camera audio without closing the picture. Tapping
elsewhere closes the view.

Routine logs never include the text of proactive HA announcements/questions; they record only that
text was present and whether `StartConversation` was requested.