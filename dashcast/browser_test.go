package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// A tab opens in the browser started for it. The first device's connection panicked in chromedp
// ("WithBrowserOption can only be used when allocating a new browser") because open handed the tab a
// browser option (techo5#26); nothing ran a real browser, so no test saw it. This one does.
//
// Where there is no browser - a PC without Chrome - it is skipped, unless DASHCAST_REQUIRE_BROWSER
// is set: the image build runs it inside the image with that set, so the browser dashcast ships
// failing to start is a failure there, never a skip.
func TestOpenATabInTheRunningBrowser(t *testing.T) {
	required := os.Getenv("DASHCAST_REQUIRE_BROWSER") != ""
	noBrowser := func(why string, args ...any) {
		t.Helper()
		if required {
			t.Fatalf(why, args...)
		}
		t.Skipf(why, args...)
	}
	if chromePath("") == "" {
		noBrowser("no Chrome or headless-shell to run")
	}
	ha := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!doctype html><title>dashboard</title><p>a dashboard"))
	}))
	defer ha.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	b, err := newBrowser(ctx, config{ha: ha.URL})
	if err != nil {
		noBrowser("the browser did not start: %v", err)
	}
	defer b.close()

	for i, kiosk := range []bool{false, true} { // a second tab too: every device gets one
		tab, closeTab, err := b.open(ctx, "/lovelace/0", 480, 480, map[string]bool{"lovelace": true}, kiosk, false)
		if err != nil {
			t.Fatalf("tab %d: %v", i+1, err)
		}
		if tab == nil {
			t.Fatalf("tab %d: no tab", i+1)
		}
		// The script put in before the page ran, whole: a mistake anywhere in it, the kiosk part
		// included, stops all of it, and the history guard is its last word before kiosk's.
		var guarded bool
		if err := chromedp.Run(tab, chromedp.Evaluate(`history.pushState.toString().includes("ok(url)")`, &guarded)); err != nil {
			t.Fatalf("tab %d: %v", i+1, err)
		}
		if !guarded {
			t.Errorf("tab %d (kiosk %v): the script put in before the page did not run", i+1, kiosk)
		}
		closeTab()
	}
}

// The top bar is hidden only for a screen that asks, and asking changes nothing else.
func TestKioskIsOnlyWhereAsked(t *testing.T) {
	plain := initScript([]byte(`"http://ha"`), []byte(`"{}"`), []byte(`["lovelace"]`), false, false)
	kiosk := initScript([]byte(`"http://ha"`), []byte(`"{}"`), []byte(`["lovelace"]`), true, false)
	if strings.Contains(plain, "techo5-kiosk") {
		t.Error("a screen that did not ask has its header hidden")
	}
	if !strings.Contains(kiosk, "techo5-kiosk") || strings.Replace(kiosk, kioskScript, "", 1) != plain {
		t.Error("kiosk is not the plain script with the header hidden after it")
	}
}

// Music Assistant's frame guard goes only into a music session's script, only for frames, and never
// with the sign-in: a frame is handed no token.
func TestMusicGuardIsOnlyInMusicFrames(t *testing.T) {
	plain := initScript([]byte(`"http://ha"`), []byte(`"{}"`), []byte(`["ma"]`), true, false)
	music := initScript([]byte(`"http://ha"`), []byte(`"{}"`), []byte(`["ma"]`), true, true)
	if strings.Contains(plain, "techo5-no-settings") {
		t.Error("a dashboard session has Music Assistant's guard")
	}
	if strings.Replace(music, musicScript, "", 1) != plain {
		t.Error("music is not the plain script with the frame guard in it")
	}
	frame, _, _ := strings.Cut(music, "localStorage")
	if !strings.Contains(frame, "window.top !== window") || !strings.Contains(frame, musicScript) {
		t.Error("the frame guard is not in the frames' branch, before the sign-in")
	}
}

// In a real browser: Music Assistant's frame, sent to its settings, comes back to its home page, both
// when it loads there and when it moves there by itself.
func TestMusicSettingsAreLeft(t *testing.T) {
	if chromePath("") == "" {
		if os.Getenv("DASHCAST_REQUIRE_BROWSER") != "" {
			t.Fatal("no Chrome or headless-shell to run")
		}
		t.Skip("no Chrome or headless-shell to run")
	}
	ha := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if strings.HasPrefix(r.URL.Path, "/api/hassio_ingress/") {
			_, _ = w.Write([]byte("<!doctype html><title>ma</title><a href=\"#/settings\">settings</a>"))
			return
		}
		_, _ = w.Write([]byte(`<!doctype html><title>panel</title><iframe id="f" src="/api/hassio_ingress/abc/#/settings/providers"></iframe>`))
	}))
	defer ha.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	b, err := newBrowser(ctx, config{ha: ha.URL})
	if err != nil {
		if os.Getenv("DASHCAST_REQUIRE_BROWSER") != "" {
			t.Fatal(err)
		}
		t.Skipf("the browser did not start: %v", err)
	}
	defer b.close()
	tab, closeTab, err := b.open(ctx, "/ma", 480, 480, map[string]bool{"ma": true}, true, true)
	if err != nil {
		t.Fatal(err)
	}
	defer closeTab()
	hash := func() string {
		var h string
		_ = chromedp.Run(tab, chromedp.Evaluate(`document.getElementById("f")?.contentWindow?.location?.hash ?? ""`, &h))
		return h
	}
	settle := func(want string) {
		t.Helper()
		for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(100 * time.Millisecond) {
			if hash() == want {
				return
			}
		}
		t.Fatalf("the frame is at %q, want %q", hash(), want)
	}
	settle("#/")
	var hidden string
	_ = chromedp.Run(tab, chromedp.Evaluate(`getComputedStyle(document.getElementById("f").contentDocument.querySelector("a")).display`, &hidden))
	if hidden != "none" {
		t.Errorf("the way to the settings shows (display %q)", hidden)
	}
	_ = chromedp.Run(tab, chromedp.Evaluate(`document.getElementById("f").contentWindow.history.pushState(null, "", "#/settings/players")`, nil))
	settle("#/")
}

// The backstop keepOnDashboards runs finds Music Assistant's frame inside shadow roots, where Home
// Assistant keeps it, and takes it out of the settings even with no guard in the frame.
func TestMusicBackstopReachesTheFrame(t *testing.T) {
	if chromePath("") == "" {
		if os.Getenv("DASHCAST_REQUIRE_BROWSER") != "" {
			t.Fatal("no Chrome or headless-shell to run")
		}
		t.Skip("no Chrome or headless-shell to run")
	}
	ha := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if strings.HasPrefix(r.URL.Path, "/api/hassio_ingress/") {
			_, _ = w.Write([]byte("<!doctype html><title>ma</title>"))
			return
		}
		_, _ = w.Write([]byte(`<!doctype html><title>panel</title><div id="host"></div><script>
			const outer = document.getElementById("host").attachShadow({mode: "open"});
			const inner = outer.appendChild(document.createElement("div")).attachShadow({mode: "open"});
			const f = document.createElement("iframe");
			f.src = "/api/hassio_ingress/abc/#/settings";
			inner.appendChild(f);
			window.frame = f;
		</script>`))
	}))
	defer ha.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	b, err := newBrowser(ctx, config{ha: ha.URL})
	if err != nil {
		if os.Getenv("DASHCAST_REQUIRE_BROWSER") != "" {
			t.Fatal(err)
		}
		t.Skipf("the browser did not start: %v", err)
	}
	defer b.close()
	tab, closeTab, err := b.open(ctx, "/ma", 480, 480, map[string]bool{"ma": true}, true, false)
	if err != nil {
		t.Fatal(err)
	}
	defer closeTab()
	hash := func() string {
		var h string
		_ = chromedp.Run(tab, chromedp.Evaluate(`window.frame?.contentWindow?.location?.hash ?? ""`, &h))
		return h
	}
	for end := time.Now().Add(5 * time.Second); hash() != "#/settings" && time.Now().Before(end); time.Sleep(100 * time.Millisecond) {
	}
	var moved bool
	if err := chromedp.Run(tab, chromedp.Evaluate(musicSettingsOut, &moved)); err != nil || !moved {
		t.Fatalf("the backstop did not move the frame (%v, %v); it is at %q", moved, err, hash())
	}
	for end := time.Now().Add(5 * time.Second); hash() != "#/" && time.Now().Before(end); time.Sleep(100 * time.Millisecond) {
	}
	if h := hash(); h != "#/" {
		t.Fatalf("the frame is at %q, want #/", h)
	}
}

// maSidebar is Music Assistant's sidebar and user menu as its 2.10 frontend draws them, cut down to
// what the guard goes by: the System group holding only the settings, the user menu with Profile and
// Edit menu under the name, and the Home Assistant button in the footer.
const maSidebar = `<!doctype html><title>ma</title>
<div data-slot="sidebar-group" id="library"><div data-slot="sidebar-group-label">Library</div><a href="#/artists">Artists</a></div>
<div data-slot="sidebar-group" id="system"><div data-slot="sidebar-group-label">System</div><a href="#/settings">Settings</a></div>
<div data-slot="sidebar-footer"><button id="ha" aria-label="Home Assistant"><img src="data:image/svg+xml,%3csvg%3e%3c/svg%3e"><span>Home Assistant</span></button><button id="collapse"><svg></svg></button></div>
<div data-slot="dropdown-menu-content" role="menu">
  <div data-slot="dropdown-menu-label"><span data-slot="avatar">D</span>dashcastuser</div>
  <div data-slot="dropdown-menu-separator"></div>
  <div data-slot="dropdown-menu-item" id="profile">Profile</div>
  <div data-slot="dropdown-menu-item" id="edit">Edit menu</div>
</div>
<div data-slot="dropdown-menu-content" role="menu" id="other">
  <div data-slot="dropdown-menu-separator"></div>
  <div data-slot="dropdown-menu-item" id="play">Play</div>
</div>
<script>parent.postMessage({type: "home-assistant/toggle-menu"}, "*"); parent.postMessage({type: "done"}, "*");</script>`

// In a real browser: the ways into Music Assistant's settings are hidden and the rest of its menu is
// not, and its Home Assistant button, if pressed anyway, does not reach Home Assistant.
func TestMusicWaysToTheSettingsAreHidden(t *testing.T) {
	if chromePath("") == "" {
		if os.Getenv("DASHCAST_REQUIRE_BROWSER") != "" {
			t.Fatal("no Chrome or headless-shell to run")
		}
		t.Skip("no Chrome or headless-shell to run")
	}
	ha := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if strings.HasPrefix(r.URL.Path, "/api/hassio_ingress/") {
			_, _ = w.Write([]byte(maSidebar))
			return
		}
		_, _ = w.Write([]byte(`<!doctype html><title>panel</title><script>
			window.heard = [];
			addEventListener("message", (e) => window.heard.push(e.data.type));
		</script><iframe id="f" src="/api/hassio_ingress/abc/#/"></iframe>`))
	}))
	defer ha.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	b, err := newBrowser(ctx, config{ha: ha.URL})
	if err != nil {
		if os.Getenv("DASHCAST_REQUIRE_BROWSER") != "" {
			t.Fatal(err)
		}
		t.Skipf("the browser did not start: %v", err)
	}
	defer b.close()
	tab, closeTab, err := b.open(ctx, "/ma", 480, 480, map[string]bool{"ma": true}, true, true)
	if err != nil {
		t.Fatal(err)
	}
	defer closeTab()
	shown := func(id string) string {
		var d string
		_ = chromedp.Run(tab, chromedp.Evaluate(`(() => { const e = document.getElementById("f")?.contentDocument?.getElementById(`+
			strconv.Quote(id)+`); return e ? getComputedStyle(e).display : "missing"; })()`, &d))
		return d
	}
	for end := time.Now().Add(5 * time.Second); shown("system") != "none" && time.Now().Before(end); time.Sleep(100 * time.Millisecond) {
	}
	for _, id := range []string{"system", "profile", "ha"} {
		if d := shown(id); d != "none" {
			t.Errorf("%s shows (display %q)", id, d)
		}
	}
	for _, id := range []string{"library", "edit", "collapse", "play"} {
		if d := shown(id); d == "none" || d == "missing" {
			t.Errorf("%s is hidden too (display %q)", id, d)
		}
	}
	var heard []string
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(100 * time.Millisecond) {
		_ = chromedp.Run(tab, chromedp.Evaluate(`window.heard`, &heard))
		if slices.Contains(heard, "done") {
			break
		}
	}
	if !slices.Contains(heard, "done") || slices.Contains(heard, "home-assistant/toggle-menu") {
		t.Errorf("the page heard %q, want done and no toggle-menu", heard)
	}
}
