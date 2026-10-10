package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// browser is the one Chrome every device's tab runs in.
type browser struct {
	cfg    config
	ctx    context.Context // the browser's own; a tab is a child of it
	cancel func()
}

func newBrowser(parent context.Context, cfg config) (*browser, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		// Inside a container Chrome runs as root, which its sandbox refuses; the container is the
		// sandbox there.
		chromedp.NoSandbox,
		chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("mute-audio", true),
	)
	if path := chromePath(cfg.chrome); path != "" {
		opts = append(opts, chromedp.ExecPath(path))
	}
	actx, acancel := chromedp.NewExecAllocator(parent, opts...)
	bctx, bcancel := chromedp.NewContext(actx, chromedp.WithErrorf(quiet))
	// Starting it now rather than with the first device, so a browser that cannot start says so at
	// once rather than when somebody first opens the page.
	if err := chromedp.Run(bctx); err != nil {
		bcancel()
		acancel()
		return nil, err
	}
	return &browser{cfg: cfg, ctx: bctx, cancel: func() { bcancel(); acancel() }}, nil
}

func (b *browser) close() { b.cancel() }

// initScript is what runs in the tab before any of Home Assistant's own code: the sign-in, the
// sidebar kept closed, the page kept on dashboards, and with kiosk the top bar hidden. Frames get
// none of that, the token least of all; with music, Music Assistant's own frame gets musicScript.
func initScript(origin, tokens, allowed []byte, kiosk, music bool) string {
	extra, frame := "", ""
	if kiosk {
		extra = kioskScript
	}
	if music {
		frame = musicScript
	}
	return fmt.Sprintf(`(() => {
  if (location.origin !== %s) return;
  if (window.top !== window) {%s
    return;
  }
  localStorage.setItem("hassTokens", %s); localStorage.setItem("dockedSidebar", '"always_hidden"');
  const allowed = new Set(%s);
  const ok = (u) => {
    try {
      const p = new URL(u, location.href);
      if (p.origin !== location.origin) return false;
      return allowed.has(p.pathname.replace(/^\/+/, "").split("/")[0]);
    } catch (e) { return false; }
  };
  for (const name of ["pushState", "replaceState"]) {
    const real = history[name].bind(history);
    history[name] = (state, title, url) => { if (url === undefined || url === null || ok(url)) return real(state, title, url); };
  }
%s
})();`, origin, frame, tokens, allowed, extra)
}

// musicScript keeps Music Assistant, in its panel's frame, out of its settings: anyone can touch the
// screen, and the settings are where its music sources and players are set up. Its menu stays, for the
// library, the queue and the players; only the way to the settings is hidden, and a page that gets
// there anyway - a link elsewhere, the back button - goes straight back to Music Assistant's home.
// It is a single-page app that moves with the history API and its #/ addresses, so it is watched
// after each move, and every second for whatever moves without either.
const musicScript = `
    if (location.pathname.startsWith("/api/hassio_ingress/")) {
      const away = () => {
        if (/^#\/settings/.test(location.hash)) location.replace(location.pathname + location.search + "#/");
      };
      for (const name of ["pushState", "replaceState"]) {
        const real = history[name].bind(history);
        history[name] = (...a) => { const r = real(...a); away(); return r; };
      }
      addEventListener("hashchange", away);
      addEventListener("popstate", away);
      setInterval(away, 1000);
      const hide = () => {
        if (!document.head || document.getElementById("techo5-no-settings")) return;
        const s = document.createElement("style");
        s.id = "techo5-no-settings";
        s.textContent = 'a[href^="#/settings"]{display:none!important}';
        document.head.appendChild(s);
      };
      addEventListener("DOMContentLoaded", hide);
      setInterval(hide, 1000);
      away();
    }`

// kioskScript hides Home Assistant's top bar, for a screen too small to give it the room (techo5#27).
// There are two of them, each inside its component's own shadow root where a page-wide style cannot
// reach: a dashboard's (hui-root, which the Energy page uses too) and the other pages' (History,
// Logbook: ha-top-app-bar-fixed). A style goes into each as it appears, and --header-height set to
// nothing there moves the page up into the room. Found by following the path the frontend builds, not
// by searching the whole page, which on a Pi every second would cost more than it saves.
const kioskScript = `
  const css = {
    "hui-root": ":host{--header-height:0px!important;--safe-area-inset-top:0px!important;min-height:100vh!important;height:100vh!important} .header{display:none!important} #view,.view{padding-top:0!important;margin-top:0!important;min-height:100vh!important;height:100vh!important}",
    "ha-top-app-bar-fixed": ":host{--header-height:0px!important;--safe-area-inset-top:0px!important;min-height:100vh!important;height:100vh!important} header.top-app-bar{display:none!important} .mdc-top-app-bar--fixed-adjust,.content{padding-top:0!important;margin-top:0!important;min-height:100vh!important}",
  };
  const dress = (el) => {
    const rule = css[el.tagName.toLowerCase()];
    if (!rule || !el.shadowRoot || el.shadowRoot.getElementById("techo5-kiosk")) return false;
    const s = document.createElement("style");
    s.id = "techo5-kiosk";
    s.textContent = rule;
    el.shadowRoot.appendChild(s);
    return true;
  };
  const look = () => {
    const main = document.querySelector("home-assistant")?.shadowRoot?.querySelector("home-assistant-main")?.shadowRoot;
    if (!main) return;
    let moved = false;
    if (!main.getElementById("jarvis-show-shell")) {
      moved = true;
      const shell = document.createElement("style");
      shell.id = "jarvis-show-shell";
      shell.textContent = ":host{--header-height:0px!important;--safe-area-inset-top:0px!important;min-height:100vh!important;height:100vh!important} ha-sidebar{display:none!important;width:0!important;min-width:0!important} #view,.view{padding-left:0!important;margin-left:0!important;padding-top:0!important;margin-top:0!important;min-height:100vh!important;height:100vh!important}";
      main.appendChild(shell);
    }
    for (const panel of main.querySelectorAll("*")) {
      if (!panel.tagName.toLowerCase().startsWith("ha-panel-") || !panel.shadowRoot) continue;
      for (const el of panel.shadowRoot.querySelectorAll("hui-root, ha-top-app-bar-fixed")) moved = dress(el) || moved;
      for (const inner of panel.shadowRoot.querySelectorAll("*")) {
        if (inner.shadowRoot) for (const el of inner.shadowRoot.querySelectorAll("hui-root, ha-top-app-bar-fixed")) moved = dress(el) || moved;
      }
    }
    // A card that measured the room it had while the bar was still there keeps that size until the
    // window changes: wall-clock-card sets its max-height to the height below its top edge, and kept
    // 56px of the screen empty at the bottom. Telling the page the window changed has it measure again.
    if (moved) requestAnimationFrame(() => dispatchEvent(new Event("resize")));
  };
  setInterval(look, 1000);
  addEventListener("load", look);`

// chromePath is the browser to run: the one named, or the headless-shell image's, or whatever
// chromedp finds for itself.
func chromePath(named string) string {
	if named != "" {
		return named
	}
	for _, p := range []string{"/headless-shell/headless-shell"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome", "headless-shell"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// open is a new tab showing path at w by h, signed in to Home Assistant, in the dark theme a screen
// in a room wants. The tab closes with ctx.
func (b *browser) open(ctx context.Context, path string, w, h int, allowed map[string]bool, kiosk, music bool) (context.Context, func(), error) {
	// No options: a tab of a browser already running takes none of the browser's, and chromedp
	// panics if it is given one (WithErrorf is one) - which it did on every device's first
	// connection (techo5#26). The browser has its quiet logger from newBrowser.
	tab, cancel := chromedp.NewContext(b.ctx)
	stop := context.AfterFunc(ctx, cancel)

	// The frontend keeps its sign-in in local storage; putting a long-lived token there before any
	// of its code runs signs it in without the login page. The sidebar is kept closed for the same
	// reason: a screen this size has no room for it.
	tokens, _ := json.Marshal(map[string]any{
		"access_token": b.cfg.token, "token_type": "Bearer", "expires_in": 1800,
		"hassUrl": b.cfg.ha, "clientId": b.cfg.ha + "/", "expires": 9999999999999, "refresh_token": "",
	})
	quoted, _ := json.Marshal(string(tokens))
	// And the frontend is kept on dashboards: it moves between its pages with the history API, and a
	// move to anywhere not allowed is refused before it happens.
	firsts := make([]string, 0, len(allowed))
	for p := range allowed {
		firsts = append(firsts, p)
	}
	list, _ := json.Marshal(firsts)
	// Only in Home Assistant's own top-level page: a card can frame another page, and one on the same
	// machine would otherwise be handed the token too.
	origin, _ := json.Marshal(haOrigin(b.cfg.ha))
	script := initScript(origin, quoted, list, kiosk, music)
	err := chromedp.Run(tab,
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(script).Do(ctx)
			return err
		}),
		emulation.SetDeviceMetricsOverride(int64(w), int64(h), 1, false),
		emulation.SetTouchEmulationEnabled(true).WithMaxTouchPoints(1),
		emulation.SetEmulatedMedia().WithFeatures([]*emulation.MediaFeature{{Name: "prefers-color-scheme", Value: "dark"}}),
		chromedp.Navigate(b.cfg.ha+path),
	)
	if err != nil {
		stop()
		cancel()
		return nil, nil, err
	}
	return tab, func() { stop(); cancel() }, nil
}

// haOrigin is Home Assistant's address as a page sees its own origin: the scheme and host in lower
// case, and the port only when it is not the scheme's own - which is how a browser writes
// location.origin, so the two can be compared as strings.
func haOrigin(ha string) string {
	u, err := url.Parse(ha)
	if err != nil {
		return ha
	}
	scheme, host, port := strings.ToLower(u.Scheme), strings.ToLower(u.Hostname()), u.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]" // IPv6
	}
	if port != "" {
		return scheme + "://" + host + ":" + port
	}
	return scheme + "://" + host
}

// quiet is where chromedp's own complaints go: events from a newer Chrome than it knows the names of
// ("unhandled node event"), which say nothing about dashcast. Anything else it has to say goes to
// the log at debug.
func quiet(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if strings.Contains(msg, "unhandled") {
		return
	}
	slog.Debug("chromedp", "said", msg)
}
