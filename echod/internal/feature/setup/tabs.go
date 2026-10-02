package setup

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
)

// The page's tabs: the device's own settings sections, in the order its screen lists them, so what is
// learned on one is where it is on the other. Each is a plain link (?tab=), and only the tab asked for
// is drawn: no script, and a save comes back to the tab it was made on.
//
// There is no Display tab. What the screen shows is set on the screen, which is where it can be seen
// changing.

type tab struct{ id, title, blurb string }

var tabs = []tab{
	{"sound", "Sound & Voice", "Announcements, radio"},
	{"alarms", "Alarms & Timers", "Alarms, timers and reminders"},
	{"connections", "Connections", "Wi-Fi"},
	{"weather", "Weather & Calendar", "Where it is, units, calendars"},
	{"photos", "Photos", "Pictures for the slideshow"},
	{"privacy", "Privacy & Security", "What this device shares"},
	{"general", "General", "Name, time zone, help"},
}

// defaultTab is where the page opens: the section people come to this page for most.
const defaultTab = "alarms"

// tabOf is the tab a request names, or the default for none or one that is not a tab.
func tabOf(id string) string {
	for _, t := range tabs {
		if t.id == id {
			return id
		}
	}
	return defaultTab
}

// head starts every page: the document and its style, in the device's own colors. The five colors
// are the theme's; everything between them is mixed from them, so a Custom theme works as well.
func head(w http.ResponseWriter) {
	p := pageColors()
	c := p.Hex()
	scheme, ok, bad := "dark", "#8bc34a", "#ff8a65"
	if p.Light() {
		scheme, ok, bad = "light", "#3f7a1c", "#b3401e"
	}
	fmt.Fprintf(w, `<!doctype html><html lang="en"><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Jarvis Show setup</title>
<style>
 :root{--bg:%s;--accent:%s;--text:%s;--dim:%s;--line:%s;--ok:%s;--bad:%s;color-scheme:%s;
  --font-ui:"Segoe UI","Noto Sans",system-ui,sans-serif;
  --font-display:"Aptos","Segoe UI","Noto Sans",system-ui,sans-serif;
  --canvas:var(--bg);--shell:color-mix(in srgb,#fff 72%%,var(--bg));
  --card:color-mix(in srgb,#fff 86%%,var(--bg));--card-strong:color-mix(in srgb,#fff 94%%,var(--bg));
  --field:color-mix(in srgb,#fff 58%%,var(--bg));--drawer:color-mix(in srgb,#fff 72%%,var(--bg));
  --accent-soft:color-mix(in srgb,var(--accent) 16%%,transparent);
  --accent-soft-strong:color-mix(in srgb,var(--accent) 24%%,transparent);
  --dimtext:color-mix(in srgb,var(--dim) 70%%,var(--text));
  --card-radius:18px;--control-radius:12px;
  --shadow:0 18px 38px color-mix(in srgb,var(--text) 12%%,transparent);
  --shadow-soft:0 10px 24px color-mix(in srgb,var(--text) 8%%,transparent)}
 *{box-sizing:border-box}
 body{font:16px/1.5 var(--font-ui);margin:0;min-height:100vh;color:var(--text);background:
  radial-gradient(circle at top right,color-mix(in srgb,#fff 72%%,transparent),transparent 30%%),
  radial-gradient(circle at 12%% 28%%,var(--accent-soft),transparent 30%%),
  linear-gradient(180deg,var(--canvas),color-mix(in srgb,var(--canvas) 70%%,var(--shell) 30%%))}
 .wrap{max-width:72rem;margin:0 auto;padding:2rem 1.25rem 3rem}
 h1,h2,legend{font-family:var(--font-display);letter-spacing:-.015em}
 h1{font-size:1.65rem;margin:0 0 .15rem} h2{font-size:1.2rem;margin:0 0 .8rem} p.sub{color:var(--dimtext);margin:0 0 1.4rem}
 fieldset{border:1px solid color-mix(in srgb,var(--line) 78%%,transparent);border-radius:var(--card-radius);margin:0 0 1rem;padding:1.15rem;min-width:0;background:var(--card);box-shadow:var(--shadow-soft);backdrop-filter:blur(12px)}
 legend{padding:0 .5rem;color:color-mix(in srgb,var(--accent) 76%%,var(--text));font-weight:650}
 label{display:block;margin:.6rem 0 .2rem;color:var(--dimtext)}
 select,input,textarea{font:inherit;width:100%%;min-height:42px;padding:.58rem .72rem;border-radius:var(--control-radius);border:1px solid color-mix(in srgb,var(--line) 82%%,transparent);background:var(--field);color:inherit;box-shadow:inset 0 1px 0 color-mix(in srgb,#fff 48%%,transparent)}
 select:focus,input:focus,textarea:focus{outline:2px solid var(--accent-soft-strong);outline-offset:1px;border-color:var(--accent)}
 input[type=checkbox],input[type=radio]{width:auto;margin-right:.4rem}
 button{font:inherit;padding:.6rem 1.15rem;border:1px solid color-mix(in srgb,var(--accent) 78%%,var(--text));border-radius:999px;background:linear-gradient(135deg,var(--accent),color-mix(in srgb,var(--accent) 66%%,var(--text)));color:#fff;font-weight:650;cursor:pointer;box-shadow:0 8px 18px var(--accent-soft)}
 a{color:var(--accent)}
 .note{color:var(--dimtext);font-size:.9rem} .ok{color:var(--ok)} .bad{color:var(--bad)}
 button.quiet{background:var(--card-strong);color:var(--text);border:1px solid var(--line);font-weight:550;padding:.4rem .9rem;box-shadow:none}
 .playing{color:var(--ok);display:flex;align-items:center;gap:.7rem;flex-wrap:wrap}
 .banner{border:1px solid color-mix(in srgb,var(--ok) 42%%,var(--bg));background:linear-gradient(135deg,color-mix(in srgb,var(--ok) 12%%,var(--card)),var(--card));border-radius:14px;padding:.65rem .85rem;margin:0 0 1rem;box-shadow:var(--shadow-soft)}
 .banner.bad{border-color:color-mix(in srgb,var(--bad) 50%%,var(--bg));background:color-mix(in srgb,var(--bad) 10%%,var(--bg))}
 .layout{display:grid;grid-template-columns:14rem minmax(0,1fr);gap:1.5rem;align-items:start}
 .layout>*{min-width:0}
 nav.rail{display:flex;flex-direction:column;gap:.3rem;position:sticky;top:1rem;padding:.55rem;border:1px solid color-mix(in srgb,var(--line) 72%%,transparent);border-radius:var(--card-radius);background:color-mix(in srgb,var(--card) 90%%,transparent);box-shadow:var(--shadow-soft);backdrop-filter:blur(12px)}
 nav.rail a{display:block;padding:.65rem .8rem;border-radius:12px;color:var(--text);text-decoration:none;border:1px solid transparent}
 nav.rail a small{display:block;color:var(--dimtext);font-size:.8rem;line-height:1.3}
 nav.rail a:hover{background:var(--accent-soft)}
 nav.rail a.on{background:linear-gradient(135deg,var(--accent-soft),color-mix(in srgb,var(--card) 88%%,var(--accent) 12%%));border-color:color-mix(in srgb,var(--accent) 30%%,var(--line));color:color-mix(in srgb,var(--accent) 72%%,var(--text));font-weight:650}
 @media (max-width:44rem){
  .layout{grid-template-columns:minmax(0,1fr);gap:1rem}
  nav.rail{flex-direction:row;overflow-x:auto;position:static;padding-bottom:.3rem;border-bottom:1px solid var(--line)}
  nav.rail a{white-space:nowrap;padding:.45rem .8rem}
  nav.rail a small{display:none}
 }
 details{border:1px solid color-mix(in srgb,var(--line) 78%%,transparent);border-radius:14px;margin:0 0 .65rem;background:var(--drawer);box-shadow:0 4px 12px color-mix(in srgb,var(--text) 5%%,transparent)}
 details>summary{list-style:none;cursor:pointer;padding:.7rem .9rem;display:flex;gap:.7rem;align-items:center;flex-wrap:wrap}
 details>summary::-webkit-details-marker{display:none}
 details>summary::after{content:"›";margin-left:auto;color:var(--dimtext)}
 details[open]>summary::after{transform:rotate(90deg)}
 details .body{padding:0 .9rem .9rem;border-top:1px solid var(--line)}
 .when{font-weight:600;font-variant-numeric:tabular-nums;min-width:4.6rem}
 .what{flex:1;min-width:8rem}
 .where{color:var(--dimtext);font-size:.85rem}
 .chip{font-size:.72rem;letter-spacing:.04em;text-transform:uppercase;padding:.1rem .5rem;border-radius:999px;border:1px solid var(--line);color:var(--dimtext)}
 .chip.rem{color:var(--accent);border-color:color-mix(in srgb,var(--accent) 45%%,var(--bg))}
 .chip.tim{color:var(--ok);border-color:color-mix(in srgb,var(--ok) 45%%,var(--bg))}
 .off{opacity:.55}
 .row{display:flex;gap:.6rem;flex-wrap:wrap;align-items:flex-end}
 .row>*{flex:1;min-width:7rem}
 .days{display:flex;gap:.3rem;flex-wrap:wrap;margin:.2rem 0}
 .days label,.kinds label{margin:0;display:flex;align-items:center;padding:.3rem .7rem;border:1px solid var(--line);border-radius:999px;color:var(--text);font-size:.9rem}
 .kinds{display:flex;gap:.4rem;flex-wrap:wrap}
 .btns{display:flex;gap:.5rem;flex-wrap:wrap;margin-top:.8rem;align-items:center}
 .btns form{display:inline}
</style>`, c[0], c[1], c[2], c[3], c[4], ok, bad, scheme)
}

// nav is the tabs, the one being shown marked.
func nav(w http.ResponseWriter, on string) {
	fmt.Fprint(w, `<nav class="rail">`)
	for _, t := range tabs {
		cls := ""
		if t.id == on {
			cls = ` class="on" aria-current="page"`
		}
		fmt.Fprintf(w, `<a href="/setup?tab=%s"%s>%s<small>%s</small></a>`,
			t.id, cls, html.EscapeString(t.title), html.EscapeString(t.blurb))
	}
	fmt.Fprint(w, `</nav>`)
}

// hidden is the fields every form carries: the session it came from, what it saves, and the tab to
// come back to.
func hidden(w http.ResponseWriter, token, what, tab string) {
	fmt.Fprintf(w, `<input type="hidden" name="token" value="%s"><input type="hidden" name="what" value="%s"><input type="hidden" name="tab" value="%s">`,
		html.EscapeString(token), html.EscapeString(what), html.EscapeString(tab))
}

// back is where a save sends the browser: the tab it came from, with what happened.
func back(tab, key, value string) string {
	q := url.Values{"tab": {tabOf(tab)}}
	if key != "" {
		q.Set(key, value)
	}
	return "/setup?" + q.Encode()
}
