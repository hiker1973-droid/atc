package main

import (
	"strings"
	"testing"
)

func TestThemeCSS(t *testing.T) {
	css := buildThemeCSS()
	for _, want := range []string{"html[data-theme=light]", "html[data-theme=night]", "--bg:#eef2f7", "--hx-8b949e:", "--hx-ff6a2b:"} {
		if !strings.Contains(css, want) {
			t.Errorf("theme.css lacks %q", want)
		}
	}
	// Light: a bright text colour goes dark enough to read on white; a dark
	// fill becomes a pale tint.
	r, g, b := hexRGB(mustHex(t, css, "light", "7ee787"))
	if _, _, l := rgbToHSL(r, g, b); l > 0.4 {
		t.Errorf("light 7ee787 lightness %.2f", l)
	}
	r, g, b = hexRGB(mustHex(t, css, "light", "3d0f0f"))
	if _, _, l := rgbToHSL(r, g, b); l < 0.85 {
		t.Errorf("light 3d0f0f lightness %.2f", l)
	}
	// Night: blue goes towards amber, never magenta.
	r, g, b = hexRGB(mustHex(t, css, "night", "1f6feb"))
	if h, _, _ := rgbToHSL(r, g, b); h > 90 && h < 345 {
		t.Errorf("night 1f6feb hue %.0f", h)
	}
}

func mustHex(t *testing.T, css, mode, hex string) string {
	t.Helper()
	block := css[strings.Index(css, "html[data-theme="+mode+"]"):]
	i := strings.Index(block, "--hx-"+hex+":#")
	if i < 0 {
		t.Fatalf("%s has no %s", mode, hex)
	}
	return block[i+len("--hx-"+hex+":#") : i+len("--hx-"+hex+":#")+6]
}
