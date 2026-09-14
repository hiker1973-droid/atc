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
