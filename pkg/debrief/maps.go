package debrief

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"sort"
	"strings"

	"github.com/vsfg7/atc/pkg/terrain"
)

// The report's maps: SVG over a shaded-relief picture of the DCS terrain
// (pkg/terrain's grids; a plain background on maps without one), in the
// style of the squadron's BFM debrief: tracks with a white halo, dashed
// missiles with an open circle at launch, a numbered ✕ for each kill in the
// killer's colour, the fields, a scale bar.

var (
	bluePalette = []string{"#0d47a1", "#1e88e5", "#00897b", "#6a1b9a", "#0097a7", "#3949ab", "#00695c", "#5e35b1"}
	redPalette  = []string{"#b71c1c", "#ef6c00", "#ad1457", "#6d4c41", "#c62828", "#e65100", "#880e4f", "#4e342e"}
	sideColour  = map[string]string{"blue": "#1f5fa8", "red": "#c0392b", "": "#555"}
)

// view is a map's window and size.
type view struct {
	lat0, lat1, lon0, lon1 float64
	W, H                   float64
}

func (v view) xy(lat, lon float64) (float64, float64) {
	return (lon - v.lon0) / (v.lon1 - v.lon0) * v.W, (v.lat1 - lat) / (v.lat1 - v.lat0) * v.H
}

// fit frames the points, at least minNM across, with a margin.
func fit(lats, lons []float64, W, H, minNM, pad float64) view {
	if len(lats) == 0 {
		return view{42, 43, 42, 43.4, W, H}
	}
	la0, la1, lo0, lo1 := lats[0], lats[0], lons[0], lons[0]
	for i := range lats {
		la0, la1 = math.Min(la0, lats[i]), math.Max(la1, lats[i])
		lo0, lo1 = math.Min(lo0, lons[i]), math.Max(lo1, lons[i])
	}
	cl := (la0 + la1) / 2
	k := math.Cos(cl * math.Pi / 180)
	spanY := math.Max(math.Max(la1-la0, (lo1-lo0)*k*H/W), minNM/60) * pad
	spanX := spanY * W / H / k
	cx := (lo0 + lo1) / 2
	return view{cl - spanY/2, cl + spanY/2, cx - spanX/2, cx + spanX/2, W, H}
}

// terrainFor is the grid covering a point, if any.
func terrainFor(lat, lon float64) *terrain.Grid {
	for _, name := range []string{"caucasus", "kola", "syria", "persiangulf", "germany", "iraq"} {
		if g, err := terrain.ForMap(name); err == nil && g.Contains(lat, lon) {
			return g
		}
	}
	return nil
}

var tint = []struct {
	h       float64
	r, g, b float64
}{
	{0, 176, 199, 146}, {200, 196, 210, 160}, {600, 222, 214, 166}, {1200, 214, 190, 148},
	{2000, 190, 162, 128}, {3000, 168, 150, 136}, {4000, 232, 229, 224}, {5600, 255, 255, 255},
}

func tintAt(h float64) (float64, float64, float64) {
	if h <= tint[0].h {
		return tint[0].r, tint[0].g, tint[0].b
	}
	for i := 1; i < len(tint); i++ {
		if h <= tint[i].h {
			a, b := tint[i-1], tint[i]
			f := (h - a.h) / (b.h - a.h)
			return a.r + (b.r-a.r)*f, a.g + (b.g-a.g)*f, a.b + (b.b-a.b)*f
		}
	}
	l := tint[len(tint)-1]
	return l.r, l.g, l.b
}

// relief is the shaded-relief picture behind a map, as a data: URI, or ""
// where there is no grid.
func relief(v view) string {
	g := terrainFor((v.lat0+v.lat1)/2, (v.lon0+v.lon1)/2)
	if g == nil {
		return ""
	}
	scale := 1.6
	W, H := int(v.W*scale), int(v.H*scale)
	dLat := (v.lat1 - v.lat0) / float64(H)
	dLon := (v.lon1 - v.lon0) / float64(W)
	h := make([]float64, W*H)
	for y := 0; y < H; y++ {
		lat := v.lat1 - (float64(y)+0.5)*dLat
		for x := 0; x < W; x++ {
			h[y*W+x] = g.HeightM(lat, v.lon0+(float64(x)+0.5)*dLon)
		}
	}
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	az, al := 315*math.Pi/180, 45*math.Pi/180
	my := dLat * 111320
	for y := 0; y < H; y++ {
		lat := v.lat1 - (float64(y)+0.5)*dLat
		mx := dLon * 111320 * math.Cos(lat*math.Pi/180)
		for x := 0; x < W; x++ {
			hc := h[y*W+x]
			if hc <= 0.5 && g.HeightM(lat, v.lon0+(float64(x)+0.5)*dLon) <= 0.5 {
				img.Set(x, y, color.RGBA{170, 204, 226, 255}) // sea
				continue
			}
			at := func(xx, yy int) float64 {
				xx, yy = max(0, min(W-1, xx)), max(0, min(H-1, yy))
				return h[yy*W+xx]
			}
			gx := (at(x+1, y) - at(x-1, y)) * 2 / (2 * mx)
			gy := (at(x, y-1) - at(x, y+1)) * 2 / (2 * my)
			slope := math.Atan(math.Hypot(gx, gy))
			aspect := math.Atan2(-gx, gy)
			shade := math.Max(0, math.Sin(al)*math.Cos(slope)+math.Cos(al)*math.Sin(slope)*math.Cos(az-aspect))
			f := 0.55 + 0.5*shade
			r, gg, b := tintAt(hc)
			img.Set(x, y, color.RGBA{uint8(math.Min(255, r*f)), uint8(math.Min(255, gg*f)), uint8(math.Min(255, b*f)), 255})
		}
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 82})
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

// svgMap is a map under construction.
type svgMap struct {
	v view
	b strings.Builder
}

func newMap(v view, class string) *svgMap {
	m := &svgMap{v: v}
	fmt.Fprintf(&m.b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" class="%s">`, v.W, v.H, class)
	if bg := relief(v); bg != "" {
		fmt.Fprintf(&m.b, `<image href="%s" width="%.0f" height="%.0f" preserveAspectRatio="none"/>`, bg, v.W, v.H)
	} else {
		fmt.Fprintf(&m.b, `<rect width="%.0f" height="%.0f" fill="#eef0e6"/>`, v.W, v.H)
	}
	m.graticule()
	return m
}

func (m *svgMap) graticule() {
	span := m.v.lat1 - m.v.lat0
	step := 1.0
	switch {
	case span < 0.6:
		return
	case span < 2:
		step = 0.5
	}
	for lat := math.Ceil(m.v.lat0/step) * step; lat < m.v.lat1; lat += step {
		_, y := m.v.xy(lat, m.v.lon0)
		fmt.Fprintf(&m.b, `<line x1="0" y1="%.1f" x2="%.0f" y2="%.1f" stroke="#5c7184" stroke-opacity=".25" stroke-width=".6"/><text x="3" y="%.1f" class="grat">%g°N</text>`, y, m.v.W, y, y-2, lat)
	}
	for lon := math.Ceil(m.v.lon0/step) * step; lon < m.v.lon1; lon += step {
		x, _ := m.v.xy(m.v.lat0, lon)
		fmt.Fprintf(&m.b, `<line x1="%.1f" y1="0" x2="%.1f" y2="%.0f" stroke="#5c7184" stroke-opacity=".25" stroke-width=".6"/><text x="%.1f" y="%.0f" class="grat">%g°E</text>`, x, x, m.v.H, x+2, m.v.H-3, lon)
	}
}

func (m *svgMap) path(pts []Point, col string, w float64, dash string, op float64) {
	if len(pts) < 2 {
		return
	}
	var d strings.Builder
	for i, p := range pts {
		x, y := m.v.xy(p.Lat, p.Lon)
		if i == 0 {
			fmt.Fprintf(&d, "M%.1f,%.1f", x, y)
		} else {
			fmt.Fprintf(&d, "L%.1f,%.1f", x, y)
		}
	}
	da := ""
	if dash != "" {
		da = ` stroke-dasharray="` + dash + `"`
	}
	fmt.Fprintf(&m.b, `<path d="%s" fill="none" stroke="#fff" stroke-opacity=".7" stroke-width="%.1f" stroke-linejoin="round"/>`, d.String(), w+1.6)
	fmt.Fprintf(&m.b, `<path d="%s" fill="none" stroke="%s" stroke-opacity="%.2f" stroke-width="%.1f" stroke-linejoin="round"%s/>`, d.String(), col, op, w, da)
}

func (m *svgMap) kill(lat, lon float64, n int, col string, small bool) {
	x, y := m.v.xy(lat, lon)
	if small {
		fmt.Fprintf(&m.b, `<g transform="translate(%.1f,%.1f)"><path d="M-5,-5L5,5M-5,5L5,-5" stroke="#fff" stroke-width="4.5"/><path d="M-5,-5L5,5M-5,5L5,-5" stroke="%s" stroke-width="2.4"/></g>`, x, y, col)
		return
	}
	fmt.Fprintf(&m.b, `<g transform="translate(%.1f,%.1f)"><path d="M-6,-6L6,6M-6,6L6,-6" stroke="#fff" stroke-width="5"/><path d="M-6,-6L6,6M-6,6L6,-6" stroke="%s" stroke-width="2.6"/><circle cx="11" cy="-10" r="7.5" fill="%s" stroke="#fff" stroke-width="1.2"/><text x="11" y="-6.6" text-anchor="middle" class="kn">%d</text></g>`, x, y, col, col, n)
}

func (m *svgMap) field(f Field) {
	x, y := m.v.xy(f.Lat, f.Lon)
	if x < 0 || y < 0 || x > m.v.W || y > m.v.H {
		return
	}
	name := strings.ToUpper(strings.Split(f.Name, "-")[0])
	ly, anchor, lx := y-10, "middle", x
	if ly < 14 { // keep the name inside the map
		ly = y + 20
	}
	half := float64(len(name)) * 4
	switch {
	case x-half < 2:
		anchor, lx = "start", x+8
	case x+half > m.v.W-2:
		anchor, lx = "end", x-8
	}
	fmt.Fprintf(&m.b, `<circle cx="%.1f" cy="%.1f" r="6" fill="%s" stroke="#fff" stroke-width="1.6"/><text x="%.1f" y="%.1f" text-anchor="%s" class="lab %s" style="font-size:12px">%s</text>`,
		x, y, sideColour[f.Side], lx, ly, anchor, f.Side, html.EscapeString(name))
}

func (m *svgMap) label(lat, lon float64, text, sub, col string) {
	x, y := m.v.xy(lat, lon)
	fmt.Fprintf(&m.b, `<circle cx="%.1f" cy="%.1f" r="3.2" fill="%s" stroke="#fff"/><text x="%.1f" y="%.1f" class="lab" style="fill:%s">%s <tspan style="font-weight:400">%s</tspan></text>`,
		x, y, col, x+5, y-5, col, html.EscapeString(text), html.EscapeString(sub))
}

func (m *svgMap) arrow(a, b Point, col string) {
	x0, y0 := m.v.xy(a.Lat, a.Lon)
	x1, y1 := m.v.xy(b.Lat, b.Lon)
	ang := math.Atan2(y1-y0, x1-x0) * 180 / math.Pi
	fmt.Fprintf(&m.b, `<path d="M0,0L-9,-4.5L-9,4.5Z" fill="%s" stroke="#fff" stroke-width=".8" transform="translate(%.1f,%.1f) rotate(%.0f)"/>`, col, x1, y1, ang)
}

func (m *svgMap) launch(p Point, col string) {
	x, y := m.v.xy(p.Lat, p.Lon)
	fmt.Fprintf(&m.b, `<circle cx="%.1f" cy="%.1f" r="3.4" fill="#fff" stroke="%s" stroke-width="1.8"/>`, x, y, col)
}

// scale draws a bar of a round number of nm and the north mark.
func (m *svgMap) scale() {
	nmAcross := (m.v.lat1 - m.v.lat0) * 60 * m.v.W / m.v.H
	n := 10.0
	for _, c := range []float64{5, 10, 20, 50, 100} {
		if c <= nmAcross/5 {
			n = c
		}
	}
	L := n / 60 / (m.v.lat1 - m.v.lat0) * m.v.H
	fmt.Fprintf(&m.b, `<g transform="translate(%.1f,%.1f)"><rect x="-8" y="-18" width="%.1f" height="28" fill="#fff" opacity=".85"/><line x1="0" y1="0" x2="%.1f" y2="0" stroke="#222" stroke-width="2"/><line x1="0" y1="-5" x2="0" y2="5" stroke="#222" stroke-width="2"/><line x1="%.1f" y1="-5" x2="%.1f" y2="5" stroke="#222" stroke-width="2"/><text x="%.1f" y="-6" text-anchor="middle" class="lab">%g nm</text></g><text x="%.0f" y="16" text-anchor="middle" class="lab">N ↑</text>`,
		m.v.W-L-22, m.v.H-18, L+16, L, L, L, L/2, n, m.v.W/2)
}

func (m *svgMap) String() string { return m.b.String() + "</svg>" }

// jetColours gives each jet in a picture its own colour, blues and reds.
func jetColours(jets []*Object) map[*Object]string {
	sorted := append([]*Object(nil), jets...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Side() != sorted[j].Side() {
			return sorted[i].Side() < sorted[j].Side()
		}
		return ShortName(sorted[i].Pilot) < ShortName(sorted[j].Pilot)
	})
	out := map[*Object]string{}
	nb, nr := 0, 0
	byPilot := map[string]string{}
	for _, o := range sorted {
		if c, ok := byPilot[o.Pilot+o.Side()]; ok { // a pilot's second jet keeps his colour
			out[o] = c
			continue
		}
		if o.Side() == "red" {
			out[o] = redPalette[nr%len(redPalette)]
			nr++
		} else {
			out[o] = bluePalette[nb%len(bluePalette)]
			nb++
		}
		byPilot[o.Pilot+o.Side()] = out[o]
	}
	return out
}

func trackBetween(o *Object, a, b float64) []Point {
	var out []Point
	for _, p := range o.Track {
		if p.T >= a && p.T <= b {
			out = append(out, p)
		}
	}
	return out
}

func thin(pts []Point, every int) []Point {
	if every <= 1 || len(pts) < 3 {
		return pts
	}
	var out []Point
	for i := 0; i < len(pts); i += every {
		out = append(out, pts[i])
	}
	return append(out, pts[len(pts)-1])
}

// OverviewMap: every player sortie's track, every kill, the fields.
func (s *Session) OverviewMap(W, H float64) string {
	if s.Rec == nil {
		return ""
	}
	var lats, lons []float64
	var jets []*Object
	for _, o := range s.Rec.Aircraft {
		if s.isPlayerJet(o) {
			jets = append(jets, o)
			for _, p := range o.Track {
				lats, lons = append(lats, p.Lat), append(lons, p.Lon)
			}
		}
	}
	for _, f := range s.Fields {
		lats, lons = append(lats, f.Lat), append(lons, f.Lon)
	}
	if len(lats) == 0 {
		return ""
	}
	m := newMap(fit(lats, lons, W, H, 40, 1.16), "map")
	for _, o := range jets {
		m.path(thin(o.Track, 4), sideColour[o.Side()], 1.2, "", 0.45)
	}
	for _, f := range s.Fields {
		m.field(f)
	}
	for _, k := range s.Kills {
		if k.Air && (k.Lat != 0 || k.Lon != 0) {
			m.kill(k.Lat, k.Lon, 0, sideColour[k.KillerSide], true)
		}
	}
	m.scale()
	return m.String()
}

func (s *Session) isPlayerJet(o *Object) bool {
	for _, p := range s.Pilots {
		if p.Key == o.Pilot {
			return true
		}
	}
	return false
}

// FightMap: the engagement from a minute before the first shot: tracks,
// missiles, numbered kills, names with the starting altitude.
func (s *Session) FightMap(en *Engagement, W, H float64) (string, []LegendItem) {
	if s.Rec == nil || len(en.jets) == 0 {
		return "", nil
	}
	var lats, lons []float64
	for _, o := range en.jets {
		for _, p := range trackBetween(o, en.from, en.to) {
			lats, lons = append(lats, p.Lat), append(lons, p.Lon)
		}
	}
	for _, sh := range en.Shots {
		if sh.missile != nil {
			for _, p := range sh.missile.Track {
				lats, lons = append(lats, p.Lat), append(lons, p.Lon)
			}
		}
	}
	m := newMap(fit(lats, lons, W, H, 25, 1.12), "fmap")
	col := jetColours(en.jets)
	for _, o := range en.jets {
		pts := trackBetween(o, en.from, en.to)
		if len(pts) < 2 {
			continue
		}
		m.path(pts, col[o], 2.4, "", 1)
		alive := o.End > en.to
		if alive {
			m.arrow(pts[len(pts)-2], pts[len(pts)-1], col[o])
		}
	}
	for _, sh := range en.Shots {
		if sh.missile == nil {
			continue
		}
		c := sideColour[sh.Side]
		if sh.jet != nil {
			c = col[sh.jet]
		}
		m.path(sh.missile.Track, c, 1.3, "4 3", .95)
		m.launch(sh.missile.Track[0], c)
	}
	for _, o := range en.jets {
		if pts := trackBetween(o, en.from, en.to); len(pts) > 1 {
			m.label(pts[0].Lat, pts[0].Lon, jetName(o, en.jets), fmt.Sprintf("%.0fk", pts[0].AltM*3.28084/1000), col[o])
		}
	}
	for _, k := range en.Kills {
		if k.Lat != 0 || k.Lon != 0 {
			m.kill(k.Lat, k.Lon, k.N, sideColour[k.KillerSide], false)
		}
	}
	m.scale()
	var legend []LegendItem
	sorted := append([]*Object(nil), en.jets...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Side() != sorted[j].Side() {
			return sorted[i].Side() < sorted[j].Side()
		}
		return jetName(sorted[i], en.jets) < jetName(sorted[j], en.jets)
	})
	for _, o := range sorted {
		legend = append(legend, LegendItem{jetName(o, en.jets), col[o]})
	}
	return m.String(), legend
}

// LegendItem is a name and its line colour.
type LegendItem struct{ Name, Colour string }

// jetName: the pilot's short name; a second jet the same pilot took in the
// same fight says so.
func jetName(o *Object, jets []*Object) string {
	n := ShortName(firstNonEmpty(o.Pilot, o.Name))
	for _, j := range jets {
		if j != o && j.Pilot == o.Pilot && j.Start < o.Start {
			return n + " (2nd jet)"
		}
	}
	return n
}

// RoutesMap: the whole engagement from the takeoff, with the fields.
func (s *Session) RoutesMap(en *Engagement, W, H float64) string {
	if s.Rec == nil || len(en.jets) == 0 {
		return ""
	}
	var lats, lons []float64
	for _, o := range en.jets {
		for _, p := range trackBetween(o, en.routeFrom, en.to) {
			lats, lons = append(lats, p.Lat), append(lons, p.Lon)
		}
	}
	for _, f := range s.Fields {
		lats, lons = append(lats, f.Lat), append(lons, f.Lon)
	}
	m := newMap(fit(lats, lons, W, H, 40, 1.18), "map")
	col := jetColours(en.jets)
	for _, o := range en.jets {
		m.path(thin(trackBetween(o, en.routeFrom, en.to), 3), col[o], 2.6, "", .9)
	}
	for _, f := range s.Fields {
		m.field(f)
	}
	for _, k := range en.Kills {
		if k.Lat != 0 || k.Lon != 0 {
			m.kill(k.Lat, k.Lon, k.N, sideColour[k.KillerSide], false)
		}
	}
	return m.String()
}
