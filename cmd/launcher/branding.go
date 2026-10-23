package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg" // DecodeConfig for uploaded logos
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Squadron name and logo (operator 2026-10-23: "have a place to upload a .jpg
// or png of a logo they choose"). Setup's first step and the Settings page set
// them; every launcher page shows them through /branding.js, and /logo.png
// serves the uploaded logo in place of the built-in vSFG-7 patch. Kept in
// branding.json and branding/logo.(png|jpg) next to the launcher.

type brandingFile struct {
	Name string `json:"name,omitempty"`
	Logo string `json:"logo,omitempty"` // file name under branding/, e.g. logo.png
}

var brandingMu sync.Mutex

const maxLogoBytes = 2 << 20

func brandingPath() string { return filepath.Join(rootDir, "branding.json") }
func brandingDir() string  { return filepath.Join(rootDir, "branding") }

func readBranding() brandingFile {
	var b brandingFile
	if raw, err := os.ReadFile(brandingPath()); err == nil {
		_ = json.Unmarshal(raw, &b)
	}
	return b
}

func writeBranding(b brandingFile) error {
	raw, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	tmp := brandingPath() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, brandingPath())
}

// squadronName is what the pages call the install: the name set in Setup, or
// vSFG-7 where the vSFG-7 pack (hand-made start files) is, else Readback ATC.
func squadronName(b brandingFile) string {
	if b.Name != "" {
		return b.Name
	}
	for _, t := range settingsTheatres {
		if theatreHasHandMadeFiles(t.Key) {
			return "vSFG-7"
		}
	}
	return "Readback ATC"
}

// customLogo is the uploaded logo's path, "" when the built-in one is used.
func customLogo(b brandingFile) string {
	if b.Logo == "" {
		return ""
	}
	p := filepath.Join(brandingDir(), filepath.Base(b.Logo))
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

// checkLogo accepts a PNG or JPEG of sensible size and returns its extension.
func checkLogo(data []byte) (string, error) {
	if len(data) == 0 || len(data) > maxLogoBytes {
		return "", fmt.Errorf("the logo must be a .png or .jpg under 2 MB")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("that is not a .png or .jpg image")
	}
	if cfg.Width < 16 || cfg.Height < 16 || cfg.Width > 4096 || cfg.Height > 4096 {
		return "", fmt.Errorf("the logo must be between 16 and 4096 pixels on a side (it is %dx%d)", cfg.Width, cfg.Height)
	}
	switch format {
	case "png":
		return "png", nil
	case "jpeg":
		return "jpg", nil
	}
	return "", fmt.Errorf("the logo must be a .png or .jpg (this is %s)", format)
}

// saveLogo stores an uploaded logo and points branding.json at it.
func saveLogo(data []byte) error {
	ext, err := checkLogo(data)
	if err != nil {
		return err
	}
	brandingMu.Lock()
	defer brandingMu.Unlock()
	if err := os.MkdirAll(brandingDir(), 0o755); err != nil {
		return err
	}
	name := "logo." + ext
	if err := os.WriteFile(filepath.Join(brandingDir(), name), data, 0o644); err != nil {
		return err
	}
	for _, other := range []string{"logo.png", "logo.jpg"} {
		if other != name {
			os.Remove(filepath.Join(brandingDir(), other))
		}
	}
	b := readBranding()
	b.Logo = name
	return writeBranding(b)
}

func brandingSnapshot() map[string]any {
	brandingMu.Lock()
	b := readBranding()
	brandingMu.Unlock()
	out := map[string]any{"name": squadronName(b), "nameSet": b.Name != "", "customLogo": false, "logoVersion": 0}
	if p := customLogo(b); p != "" {
		out["customLogo"] = true
		out["wide"] = logoIsWide(p)
		if st, err := os.Stat(p); err == nil {
			out["logoVersion"] = st.ModTime().Unix()
		}
	}
	return out
}

// handleBranding: GET the name and logo state (viewer); POST (control) JSON
// {"op": "name", "name": ...} | {"op": "logo", "data": "<data: URL or base64>"} | {"op": "reset-logo"}.
func handleBranding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, brandingSnapshot())
		return
	}
	if code, err := controlAllowed(r); err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	var req struct {
		Op   string `json:"op"`
		Name string `json:"name"`
		Data string `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxLogoBytes*2)).Decode(&req); err != nil {
		http.Error(w, "bad request (is the logo over 2 MB?)", http.StatusBadRequest)
		return
	}
	switch req.Op {
	case "name":
		name := strings.TrimSpace(req.Name)
		if len(name) > 40 || strings.ContainsAny(name, "<>\"&") {
			http.Error(w, "keep the name under 40 characters, without < > \" &", http.StatusBadRequest)
			return
		}
		brandingMu.Lock()
		b := readBranding()
		b.Name = name
		err := writeBranding(b)
		brandingMu.Unlock()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	case "logo":
		data := req.Data
		if i := strings.Index(data, ";base64,"); i >= 0 && strings.HasPrefix(data, "data:") {
			data = data[i+len(";base64,"):]
		}
		raw, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			http.Error(w, "the logo did not arrive whole - try again", http.StatusBadRequest)
			return
		}
		if err := saveLogo(raw); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	case "reset-logo":
		brandingMu.Lock()
		b := readBranding()
		b.Logo = ""
		err := writeBranding(b)
		brandingMu.Unlock()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	default:
		http.Error(w, "op must be name, logo or reset-logo", http.StatusBadRequest)
		return
	}
	snap := brandingSnapshot()
	snap["ok"] = true
	writeJSON(w, snap)
}

// serveLogo is the uploaded logo, else the built-in vSFG-7 patch. Not cached
// for long, so a new logo shows on the next page load.
func serveLogo(w http.ResponseWriter, r *http.Request) {
	brandingMu.Lock()
	p := customLogo(readBranding())
	brandingMu.Unlock()
	w.Header().Set("Cache-Control", "no-cache")
	if p != "" {
		if strings.HasSuffix(p, ".jpg") {
			w.Header().Set("Content-Type", "image/jpeg")
		} else {
			w.Header().Set("Content-Type", "image/png")
		}
		http.ServeFile(w, r, p)
		return
	}
	serveBuiltInLogo(w)
}

// builtInLogo is the logo with none uploaded: the vSFG-7 patch where the
// vSFG-7 pack is (the rigs), else the Readback ATC tower.
func builtInLogo() string {
	if squadronName(brandingFile{}) == "vSFG-7" {
		return "logo.png"
	}
	return "readback.png"
}

func serveBuiltInLogo(w http.ResponseWriter) {
	data, err := uiFS.ReadFile(builtInLogo())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(data)
}

// logoIsWide: a banner rather than a badge (the Readback logo is 1600x600).
// The pages show it whole, and the tab icon falls back to the built-in one.
func logoIsWide(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	return err == nil && cfg.Height > 0 && float64(cfg.Width)/float64(cfg.Height) > 1.6
}

// serveIcon is the browser-tab icon: the uploaded logo when it is roughly
// square, else the built-in one (a banner is unreadable at 16 pixels).
func serveIcon(w http.ResponseWriter, r *http.Request) {
	brandingMu.Lock()
	p := customLogo(readBranding())
	brandingMu.Unlock()
	w.Header().Set("Cache-Control", "no-cache")
	if p != "" && !logoIsWide(p) {
		serveLogo(w, r)
		return
	}
	serveBuiltInLogo(w)
}

// brandingJS puts the squadron's name and logo on a page (the header label,
// the patch image, the tab title and icon) and gives Setup and Settings the
// save and upload helpers. Every page loads it.
const brandingJS = `(() => {
  let shownName = 'vSFG-7'; // what the page's own title starts with
  window.applyBranding = b => {
    const v = '/logo.png?v=' + (b.logoVersion || 0);
    document.querySelectorAll('.logo').forEach(e => { e.textContent = b.name; e.style.display = b.wide ? 'none' : ''; });
    document.querySelectorAll('img.patch, img.squadron-logo').forEach(e => {
      e.src = v; e.alt = b.name + ' logo';
      // A banner logo already carries the name: show it whole, not in a circle.
      if (b.wide) {
        e.style.width = 'auto'; e.style.borderRadius = '4px'; e.style.boxShadow = 'none'; e.style.objectFit = 'contain';
        if (e.classList.contains('patch')) e.style.height = '46px'; // the header: big enough to read
      } else { e.style.width = ''; e.style.height = ''; e.style.borderRadius = ''; e.style.boxShadow = ''; e.style.objectFit = ''; }
    });
    if (document.title.startsWith(shownName)) document.title = b.name + document.title.slice(shownName.length);
    shownName = b.name;
    let icon = document.querySelector('link[rel="icon"]');
    if (!icon) { icon = document.createElement('link'); icon.rel = 'icon'; document.head.appendChild(icon); }
    icon.href = '/icon.png?v=' + (b.logoVersion || 0);
    window.squadron = b;
    document.dispatchEvent(new CustomEvent('branding', { detail: b }));
  };
  // squadronSave POSTs a change ({op: 'name'|'logo'|'reset-logo', ...}) and
  // shows the result on this page; it throws with the server's reason.
  window.squadronSave = async body => {
    const r = await fetch('/api/branding', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    const t = await r.text();
    if (!r.ok) throw new Error(t.trim() || ('HTTP ' + r.status));
    const b = JSON.parse(t);
    window.applyBranding(b);
    return b;
  };
  // squadronUploadLogo reads a chosen file and saves it as the logo.
  window.squadronUploadLogo = async file => {
    if (!file) throw new Error('Choose a .png or .jpg first.');
    if (!/^image\/(png|jpeg)$/.test(file.type)) throw new Error('The logo must be a .png or .jpg.');
    if (file.size > 2 * 1024 * 1024) throw new Error('The logo must be under 2 MB.');
    const data = await new Promise((res, rej) => {
      const fr = new FileReader(); fr.onload = () => res(fr.result); fr.onerror = () => rej(new Error('Could not read the file.'));
      fr.readAsDataURL(file);
    });
    return window.squadronSave({ op: 'logo', data });
  };
  fetch('/api/branding').then(r => r.json()).then(window.applyBranding).catch(() => { /* keep the built-in header */ });
})();
`

func serveBrandingJS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	io.WriteString(w, brandingJS)
}
