package terrain

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"math"
	"testing"
)

// A flat sea-level grid 40-44N, 40-44E with a 3,000 m ridge along 42.8N.
func ridge() *Grid {
	const step, rows, cols = 0.05, 81, 81
	h := make([]int16, rows*cols)
	for x := 0; x < cols; x++ {
		h[56*cols+x] = 3000 // row 56 = 42.80N
	}
	return New("test", 40, 40, step, rows, cols, h)
}

func TestHeight(t *testing.T) {
	g := ridge()
	if v := g.HeightM(42.8, 41.0); math.Abs(v-3000) > 0.01 {
		t.Errorf("on the ridge %v", v)
	}
	if v := g.HeightM(42.825, 41.0); math.Abs(v-1500) > 0.01 {
		t.Errorf("half way off the ridge %v (bilinear)", v)
	}
	if v := g.HeightM(50, 41); v != 0 {
		t.Errorf("outside the grid %v", v)
	}
}

func TestVisible(t *testing.T) {
	g := ridge()
	for _, c := range []struct {
		name       string
		lat1, alt1 float64
		lat2, alt2 float64
		want       bool
	}{
		// AWACS at 41N 30,000 ft, target at 43N (120 nm north) with the ridge 12 nm
		// in front of it: the radar line passes the ridge at ~800 m.
		{"low target behind the ridge", 41, 30000, 43, 500, false},
		{"high target over the ridge", 41, 30000, 43, 25000, true},
		// The same ridge half way to a low target (radar line ~3,900 m there) does not mask.
		{"ridge half way", 43.4, 30000, 42.2, 500, true},
		// Both on the same side: nothing in between.
		{"same side, low", 41, 30000, 41.8, 500, true},
		// Far and low over flat sea: blocked by the earth's curve (horizon ~ 220 nm for
		// 30,000 ft / 500 ft; 4 degrees of latitude = 240 nm).
		{"over the horizon", 40.0, 30000, 44.0, 0, false},
	} {
		if got := g.Visible(c.lat1, 41, c.alt1, c.lat2, 41, c.alt2); got != c.want {
			t.Errorf("%s: visible %v, want %v", c.name, got, c.want)
		}
	}
}

func TestLoad(t *testing.T) {
	var raw bytes.Buffer
	raw.WriteString(`{"name":"t","latMin":40,"lonMin":40,"step":0.5,"rows":2,"cols":3}` + "\n")
	binary.Write(&raw, binary.LittleEndian, []int16{0, 10, 20, 30, 40, 50})
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	w.Write(raw.Bytes())
	w.Close()
	g, err := Load(gz.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if g.Rows != 2 || g.Cols != 3 || g.HeightM(40.5, 41.0) != 50 {
		t.Errorf("loaded %+v, h(40.5,41)=%v", g, g.HeightM(40.5, 41.0))
	}
}

// The embedded Caucasus grid: an AWACS south of the main ridge over Kutaisi
// at 25,000 ft loses a jet low on the north side (Nalchik) and sees it high.
func TestCaucasusGrid(t *testing.T) {
	g, err := ForMap("Caucasus")
	if err != nil {
		t.Fatal(err)
	}
	if h := g.HeightM(43.355, 42.439); h < 5000 {
		t.Errorf("Elbrus %.0f m, want > 5000", h)
	}
	if h := g.HeightM(42.241, 42.058); h > 50 {
		t.Errorf("Senaki %.0f m, want < 50", h)
	}
	if g.Visible(42.2, 42.5, 25000, 43.5, 43.6, 2500) {
		t.Error("low jet at Nalchik visible over the main ridge")
	}
	if !g.Visible(42.2, 42.5, 25000, 43.5, 43.6, 30000) {
		t.Error("high jet at Nalchik masked")
	}
	if !g.Visible(42.2, 41.0, 25000, 41.7, 41.5, 1000) {
		t.Error("low jet over the sea off Batumi masked")
	}
	if _, err := ForMap("nevada"); err == nil {
		t.Error("nevada: want no grid")
	}
}

// The embedded Syria grid (Foothold): an AWACS over Cyprus at 25,000 ft loses
// a jet low over Damascus behind the Lebanon ranges and sees it high; a jet low
// over the sea off Latakia stays in sight.
func TestSyriaGrid(t *testing.T) {
	g, err := ForMap("syria")
	if err != nil {
		t.Fatal(err)
	}
	if h := g.HeightM(33.411, 36.516); h < 500 || h > 700 {
		t.Errorf("Damascus %.0f m, want ~616", h)
	}
	if g.Visible(34.8, 33.0, 25000, 33.5, 36.5, 3000) {
		t.Error("low jet over Damascus visible over the Lebanon ranges")
	}
	if !g.Visible(34.8, 33.0, 25000, 33.5, 36.5, 25000) {
		t.Error("high jet over Damascus masked")
	}
	if !g.Visible(34.8, 33.0, 25000, 35.5, 35.6, 1000) {
		t.Error("low jet over the sea off Latakia masked")
	}
}

// The embedded Kola grid (zoom-7 tiles): an AWACS over the Vestfjord at
// 25,000 ft loses a jet at 1,500 ft in the Bardufoss valleys behind the
// Norwegian mountains and sees it at 20,000 ft; a jet low over the sea north
// of Tromso stays in sight.
func TestKolaGrid(t *testing.T) {
	g, err := ForMap("kola")
	if err != nil {
		t.Fatal(err)
	}
	if h := g.HeightM(67.822, 20.337); h < 400 || h > 520 {
		t.Errorf("Kiruna %.0f m, want ~459", h)
	}
	if g.Visible(68.4, 15.0, 25000, 68.6, 18.5, 1500) {
		t.Error("low jet near Bardufoss visible over the mountains")
	}
	if !g.Visible(68.4, 15.0, 25000, 68.6, 18.5, 20000) {
		t.Error("high jet near Bardufoss masked")
	}
	if !g.Visible(69.5, 16.0, 25000, 70.0, 19.0, 500) {
		t.Error("low jet over the sea north of Tromso masked")
	}
}
