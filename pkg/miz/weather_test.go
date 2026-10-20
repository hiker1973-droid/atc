package miz

import (
	"archive/zip"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// writeMiz builds a minimal .miz whose 'mission' entry is the given Lua text.
func writeMiz(t *testing.T, mission string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.miz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("mission")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(mission)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// Mission-editor serialization: bracketed keys.
const bracketedMission = `mission = {
	["weather"] = {
		["atmosphere_type"] = 0,
		["wind"] = {
			["at8000"] = { ["speed"] = 17, ["dir"] = 281, },
			["atGround"] = { ["speed"] = 4.6, ["dir"] = 306, },
		},
		["visibility"] = { ["distance"] = 80000, },
		["fog"] = { ["visibility"] = 0, ["thickness"] = 0, },
		["season"] = { ["temperature"] = 4, },
		["qnh"] = 760,
		["clouds"] = { ["thickness"] = 200, ["density"] = 5, ["base"] = 1500, },
	},
}`

// Server re-save serialization: bare keys. Shape copied from
// "CA V1.16.VFR 5 Towering Cumulus (Winter).miz" as rewritten 2026-09-13, which
// parsed to all zeros under the bracket-only regexes. The trigger string ahead
// of the weather block must not be read as the QNH.
const bareMission = `mission = {
	trig = { funcs = { "a_do_script(\"qnh = 999\")", }, },
	weather = {
		atmosphere_type = 0,
		wind = {
			at8000 = {
				speed = 17,
				dir = 281,
			},
			atGround = {
				speed = 4.6,
				dir = 306,
			},
		},
		enable_fog = false,
		visibility = {
			distance = 80000,
		},
		fog = {
			visibility = 0,
			thickness = 0,
		},
		season = {
			temperature = 4,
		},
		type_weather = 0,
		qnh = 760,
		clouds = {
			thickness = 200,
			density = 5,
			base = 1500,
		},
	},
}`

func TestReadMizWeatherBothKeyStyles(t *testing.T) {
	for name, mission := range map[string]string{"bracketed": bracketedMission, "bare": bareMission} {
		t.Run(name, func(t *testing.T) {
			w, err := ReadMizWeather(writeMiz(t, mission))
			if err != nil {
				t.Fatal(err)
			}
			near := func(field string, got, want float64) {
				if math.Abs(got-want) > 0.01 {
					t.Errorf("%s = %v, want %v", field, got, want)
				}
			}
			near("AltInHg", w.AltInHg, 760*0.0393701)
			near("WindKts", w.WindKts, 4.6*1.94384)
			near("WindDirTrue", w.WindDirTrue, 126) // blowing towards 306 = from 126
			near("VisNm", w.VisNm, 80000/1852.0)
			near("TempC", w.TempC, 4)
			near("CeilFt", w.CeilFt, 1500*3.28084)
		})
	}
}

// Cloud presets (density 0 + a preset name): the ceiling is the lowest broken
// or overcast layer, moved by the mission's base. Shapes from the Foothold v1.12
// and Training V1.31 weather sets, which all read "no ceiling" before 2026-10-19.
func TestReadMizWeatherCloudPresets(t *testing.T) {
	for _, tc := range []struct {
		preset, base string
		ceilFt       float64
		clouds       string
	}{
		{"Preset83", "1981", 1981 * 3.28084, "Scattered Showers 7"},               // BKN at the base: 14 Congestus
		{"Preset37", "914", (5620 - 2520 + 914) * 3.28084, "Scattered Showers 3"}, // SCT low, BKN 3100 m above
		{"Preset44", "1524", 25000, "Scattered Cumulus 1"},                        // scattered only: no ceiling
		{"Preset90", "243", 25000, "VFR Thunderstorms 2"},                         // scattered at 800 ft
		{"NoSuchPreset", "500", 25000, ""},
	} {
		mission := `mission = {
	weather = {
		qnh = 760,
		clouds = {
			thickness = 200,
			density = 0,
			preset = "` + tc.preset + `",
			base = ` + tc.base + `,
			iprecptns = 0,
		},
	},
}`
		w, err := ReadMizWeather(writeMiz(t, mission))
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(w.CeilFt-tc.ceilFt) > 1 || w.Clouds != tc.clouds {
			t.Errorf("%s base %s: ceiling %.0f %q, want %.0f %q", tc.preset, tc.base, w.CeilFt, w.Clouds, tc.ceilFt, tc.clouds)
		}
		if math.Abs(w.AltInHg-760*0.0393701) > 0.01 {
			t.Errorf("%s: QNH lost (%v)", tc.preset, w.AltInHg)
		}
	}
}

func TestReadMizWeatherTheatre(t *testing.T) {
	for _, mission := range []string{
		"mission = {\n[\"theatre\"] = \"Caucasus\",\n[\"weather\"] = {\n[\"qnh\"] = 760,\n},\n}\n",
		"mission = {\ntheatre = \"Caucasus\",\nweather = {\nqnh = 760,\n},\n}\n",
	} {
		w, err := ReadMizWeather(writeMiz(t, mission))
		if err != nil {
			t.Fatal(err)
		}
		if w.Theatre != "Caucasus" {
			t.Fatalf("theatre = %q, want Caucasus", w.Theatre)
		}
	}
}
