// Package terrain answers "can a radar at A see an aircraft at B over the
// ground between them?" from a coarse height grid of a DCS map (GCI phase 2,
// operator 2026-10-21: terrain masking for the AWACS picture).
//
// Grids are built by tools/terrain-grid/build_grid.py from public SRTM-derived
// elevation (the DCS maps here are real geography, so it matches the DCS ground
// closely) and stored as gzip'd little-endian int16 metres, row by row from the
// south-west corner. See grids/README.md for what each covers.
package terrain

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
)

// Grid is a regular latitude / longitude height grid.
type Grid struct {
	Name           string
	LatMin, LonMin float64 // south-west corner, degrees
	Step           float64 // degrees between samples
	Rows, Cols     int
	h              []int16 // metres, Rows x Cols, row 0 = LatMin
}

// header is the JSON line that precedes the heights in a grid file.
type header struct {
	Name   string  `json:"name"`
	LatMin float64 `json:"latMin"`
	LonMin float64 `json:"lonMin"`
	Step   float64 `json:"step"`
	Rows   int     `json:"rows"`
	Cols   int     `json:"cols"`
}

// Load reads a grid file: gzip( JSON header line + Rows*Cols int16 LE ).
func Load(gz []byte) (*Grid, error) {
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	nl := bytes.IndexByte(raw, '\n')
	if nl < 0 {
		return nil, fmt.Errorf("terrain: no header")
	}
	var hd header
	if err := json.Unmarshal(raw[:nl], &hd); err != nil {
		return nil, fmt.Errorf("terrain: header: %w", err)
	}
	body := raw[nl+1:]
	if hd.Rows <= 1 || hd.Cols <= 1 || len(body) != 2*hd.Rows*hd.Cols {
		return nil, fmt.Errorf("terrain: %s: %d bytes for %dx%d", hd.Name, len(body), hd.Rows, hd.Cols)
	}
	g := &Grid{Name: hd.Name, LatMin: hd.LatMin, LonMin: hd.LonMin, Step: hd.Step, Rows: hd.Rows, Cols: hd.Cols,
		h: make([]int16, hd.Rows*hd.Cols)}
	if err := binary.Read(bytes.NewReader(body), binary.LittleEndian, g.h); err != nil {
		return nil, err
	}
	return g, nil
}

// New builds a grid from heights in metres (row 0 = latMin), for tests and tools.
func New(name string, latMin, lonMin, step float64, rows, cols int, h []int16) *Grid {
	return &Grid{Name: name, LatMin: latMin, LonMin: lonMin, Step: step, Rows: rows, Cols: cols, h: h}
}

// Contains: the point is inside the grid.
func (g *Grid) Contains(lat, lon float64) bool {
	return lat >= g.LatMin && lon >= g.LonMin &&
		lat <= g.LatMin+float64(g.Rows-1)*g.Step && lon <= g.LonMin+float64(g.Cols-1)*g.Step
}

// HeightM is the ground height in metres (bilinear); 0 outside the grid (sea).
func (g *Grid) HeightM(lat, lon float64) float64 {
	if !g.Contains(lat, lon) {
		return 0
	}
	fy := (lat - g.LatMin) / g.Step
	fx := (lon - g.LonMin) / g.Step
	y0, x0 := int(fy), int(fx)
	y1, x1 := min(y0+1, g.Rows-1), min(x0+1, g.Cols-1)
	ty, tx := fy-float64(y0), fx-float64(x0)
	at := func(y, x int) float64 { return float64(g.h[y*g.Cols+x]) }
	a := at(y0, x0)*(1-tx) + at(y0, x1)*tx
	b := at(y1, x0)*(1-tx) + at(y1, x1)*tx
	return a*(1-ty) + b*ty
}

const (
	earthRadiusM = 6371000.0
	kEffective   = 4.0 / 3.0 // standard refraction: radar sees a slightly flatter earth
	ftPerM       = 3.28084
	sampleM      = 2000.0 // ground check every 2 km along the line
)

// Visible reports whether a straight radar line from (lat1, lon1, alt1 ft MSL)
// to (lat2, lon2, alt2 ft MSL) clears the ground. Earth curvature (4/3 earth)
// is applied, so a line that dips under the horizon is blocked too. The end
// points themselves are not tested - the aircraft are where they are.
func (g *Grid) Visible(lat1, lon1, alt1Ft, lat2, lon2, alt2Ft float64) bool {
	d := haversineM(lat1, lon1, lat2, lon2)
	n := int(d / sampleM)
	if n < 2 {
		return true
	}
	h1, h2 := alt1Ft/ftPerM, alt2Ft/ftPerM
	re := kEffective * earthRadiusM
	for i := 1; i < n; i++ {
		f := float64(i) / float64(n)
		d1 := d * f
		// Height of the straight line above the local surface of the sphere,
		// relative to its height at the ends: the curvature bulge d1*d2/2Re.
		line := h1 + (h2-h1)*f - d1*(d-d1)/(2*re)
		lat, lon := lat1+(lat2-lat1)*f, lon1+(lon2-lon1)*f
		if g.HeightM(lat, lon) > line {
			return false
		}
	}
	return true
}

func haversineM(lat1, lon1, lat2, lon2 float64) float64 {
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dp, dl := p2-p1, (lon2-lon1)*math.Pi/180
	a := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return 2 * earthRadiusM * math.Asin(math.Sqrt(a))
}
