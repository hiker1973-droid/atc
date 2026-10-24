package main

import (
	"fmt"
	"io"
	"io/fs"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Colour modes (operator 2026-10-23: "dashboard color modes", Dark, Light,
// Night and Auto). Every launcher page loads /theme.js and /theme.css in its
// <head>. theme.js applies the person's choice (kept in this browser) before
// the page draws; theme.css redefines each page's palette for Light and
// Night. Colours a page sets directly are written var(--hx-ff9492, #ff9492),
// so Dark is exactly as before and this file works out a Light and a Night
// version of every one of them: Light darkens bright text colours to read on
// white and turns dark fills into pale tints of the same hue; Night pulls
// every hue towards red and amber (so it does not spoil night vision) while
// keeping blue side, red side, OK and warning apart.

const themeJS = `(() => {
  const KEY = 'readbackTheme';
  const MODES = ['dark', 'light', 'night', 'auto'];
  const ICON = { dark: '◐', light: '☀', night: '☾', auto: 'A' };
  const NAME = { dark: 'Dark', light: 'Light', night: 'Night (red, for a dark room)', auto: 'Auto (follows Windows)' };
  let mode = 'dark';
  try { mode = localStorage.getItem(KEY) || 'dark'; } catch (e) {}
  if (!MODES.includes(mode)) mode = 'dark';
  const dark = window.matchMedia ? window.matchMedia('(prefers-color-scheme: dark)') : null;
  const apply = () => {
    const t = mode === 'auto' ? (dark && !dark.matches ? 'light' : 'dark') : mode;
    document.documentElement.dataset.theme = t;
    document.documentElement.style.colorScheme = t === 'light' ? 'light' : 'dark';
    const b = document.getElementById('themeBtn');
    if (b) { b.textContent = ICON[mode]; b.title = 'Colours: ' + NAME[mode] + ' - click for the next'; }
  };
  apply();
  if (dark && dark.addEventListener) dark.addEventListener('change', () => mode === 'auto' && apply());
  window.readbackTheme = {
    modes: MODES, names: NAME,
    get: () => mode,
    set: m => { if (!MODES.includes(m)) return; mode = m; try { localStorage.setItem(KEY, m); } catch (e) {} apply(); },
  };
  // The header button: one click moves to the next mode.
  document.addEventListener('DOMContentLoaded', () => {
    const header = document.querySelector('header');
    if (!header || document.getElementById('themeBtn')) return;
    const b = document.createElement('button');
    b.id = 'themeBtn';
    b.type = 'button';
    b.className = 'theme-btn';
    b.onclick = () => window.readbackTheme.set(MODES[(MODES.indexOf(mode) + 1) % MODES.length]);
    const slot = header.querySelector('.theme-slot');
    if (slot) slot.appendChild(b); else header.appendChild(b);
    apply();
  });
})();
`

// The pages' own palettes, per mode. The dashboard and the other pages use
// some different names; both sets are here.
const lightPalette = `--bg:#eef2f7;--panel:#ffffff;--panel-2:#f2f5f9;--border:#c9d3e0;--text:#1b2635;--muted:#5b6b80;--label:#46566c;
--gold:#8a5d00;--gold-dim:#f1e2b6;--blue:#1f5fbf;--blue-dim:#d5e3f7;--green:#1a7f37;--green-dim:#b9e2c4;--red:#c62828;
--amber:#946200;--orange:#b5541c;`

const nightPalette = `--bg:#070302;--panel:#110704;--panel-2:#190b06;--border:#3a190d;--text:#e6a07c;--muted:#8c5640;--label:#b37252;
--gold:#e0a050;--gold-dim:#5a3410;--blue:#c7a991;--blue-dim:#3a2418;--green:#ecd2aa;--green-dim:#4a3018;--red:#ff4a2a;
--amber:#e5893a;--orange:#e5793a;`

var hexVarRE = regexp.MustCompile(`--hx-([0-9a-f]{6})`)

var (
	themeCSSOnce sync.Once
	themeCSS     string
)

// buildThemeCSS reads every page for its --hx- colours and writes both modes.
func buildThemeCSS() string {
	seen := map[string]bool{}
	fs.WalkDir(uiFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".html") {
			return nil
		}
		b, _ := uiFS.ReadFile(p)
		for _, m := range hexVarRE.FindAllStringSubmatch(string(b), -1) {
			seen[m[1]] = true
		}
		return nil
	})
	for _, m := range hexVarRE.FindAllStringSubmatch(brandingJS, -1) { // the shared header script
		seen[m[1]] = true
	}
	var hexes []string
	for h := range seen {
		hexes = append(hexes, h)
	}
	sort.Strings(hexes)
	var b strings.Builder
	b.WriteString("/* Readback ATC colour modes: made by the launcher (theme.go). */\n")
	write := func(mode, palette string, f func(h, s, l float64) (float64, float64, float64)) {
		fmt.Fprintf(&b, "html[data-theme=%s] {\n%s\n", mode, palette)
		for _, x := range hexes {
			r, g, bl := hexRGB(x)
			h, s, l := rgbToHSL(r, g, bl)
			h, s, l = f(h, s, l)
			fmt.Fprintf(&b, "--hx-%s:%s;", x, hslToHex(h, s, l))
		}
		b.WriteString("\n}\n")
	}
	write("light", lightPalette, func(h, s, l float64) (float64, float64, float64) {
		switch {
		case l >= 0.97: // white text on a coloured button stays white
			return h, s, l
		case l > 0.45: // a light colour drawn on dark: darken it to read on white
			return h, math.Min(s, 0.75), 0.32
		case l < 0.3: // a dark fill: a pale tint of the same hue
			return h, math.Min(s, 0.5), 0.93
		}
		return h, s, l
	})
	write("night", nightPalette, func(h, s, l float64) (float64, float64, float64) {
		// Pull the hue into red-amber, keeping a quarter of its distance so
		// the colours stay apart; quieter and a little dimmer.
		// Blues and greens go towards amber and yellow, purples towards red:
		// never round through magenta.
		d := h - 15
		if h > 255 {
			d = h - 15 - 360
		}
		return math.Mod(15+d*0.25+360, 360), math.Min(s, 0.7) * 0.85, l * 0.82
	})
	// Things the pages draw that are not colours: the header button.
	b.WriteString(`.theme-btn { background: transparent; color: inherit; border: 1px solid var(--border); border-radius: 5px;
  font: 600 0.8rem/1 system-ui, "Segoe UI", sans-serif; width: 1.9rem; height: 1.7rem; cursor: pointer; opacity: 0.8; padding: 0; }
.theme-btn:hover { opacity: 1; }
html[data-theme=light] img.patch { box-shadow: 0 0 0 1px var(--border); }
html[data-theme=night] img { filter: sepia(0.6) saturate(0.7) brightness(0.8); }
`)
	return b.String()
}

func hexRGB(h string) (float64, float64, float64) {
	v, _ := strconv.ParseUint(h, 16, 32)
	return float64(v>>16&255) / 255, float64(v>>8&255) / 255, float64(v&255) / 255
}

func rgbToHSL(r, g, b float64) (float64, float64, float64) {
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l := (mx + mn) / 2
	if mx == mn {
		return 0, 0, l
	}
	d := mx - mn
	s := d / (1 - math.Abs(2*l-1))
	var h float64
	switch mx {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return math.Mod(h*60+360, 360), s, l
}

func hslToHex(h, s, l float64) string {
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	to := func(v float64) int { return int(math.Round(math.Max(0, math.Min(1, v+m)) * 255)) }
	return fmt.Sprintf("#%02x%02x%02x", to(r), to(g), to(b))
}

func serveThemeJS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	io.WriteString(w, themeJS)
}

func serveThemeCSS(w http.ResponseWriter, _ *http.Request) {
	themeCSSOnce.Do(func() { themeCSS = buildThemeCSS() })
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	io.WriteString(w, themeCSS)
}
