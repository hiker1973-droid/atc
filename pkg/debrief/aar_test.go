package debrief

import (
	"strings"
	"testing"
)

func TestFacts(t *testing.T) {
	rec, err := ParseACMI(strings.NewReader(sampleACMI))
	if err != nil {
		t.Fatal(err)
	}
	in := Inputs{Squadron: "vSFG-7", Tacview: rec}
	if n := ReportNumber(in); n != "AAR-20261022-1800" {
		t.Fatalf("report number %s", n)
	}
	got := map[string]string{}
	for _, f := range Facts(in, []string{"Tacview x.acmi"}) {
		got[f.Label] = f.Value
	}
	want := map[string]string{
		"Unit":             "vSFG-7",
		"Mission":          "Caucasus Training, V1.24",
		"Date":             "22 October 2026",
		"Time":             "1800Z - 1820Z (20 min)",
		"Players":          "1 (F-16C_50)",
		"Sorties":          "1 takeoffs",
		"Weapons expended": "1 (1 AIM_120C)",
		"Kills (likely)":   "1 air, 0 ground/sea",
		"Losses":           "0 player aircraft, 0 friendly aircraft in all",
		"Sources":          "Tacview x.acmi",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

func TestHTML(t *testing.T) {
	md := "## 1. Mission Overview\n**BLUF:** Venom *won*.\n\n- 18:00Z - takeoff\n- 18:05Z - <script>x</script>\n\n" +
		"| A | B |\n|---|---|\n| 1 | `2` |\n\n1. Fix it [Flight leads]\n2. Again\n"
	h := HTML(md)
	for _, want := range []string{"<h2>1. Mission Overview</h2>", "<p><b>BLUF:</b> Venom <i>won</i>.</p>", "<ul>\n<li>18:00Z - takeoff</li>",
		"&lt;script&gt;", "<td><code>2</code></td>", "<ol>\n<li>Fix it [Flight leads]</li>"} {
		if !strings.Contains(h, want) {
			t.Errorf("HTML lacks %q:\n%s", want, h)
		}
	}
	if strings.Contains(h, "<script>") {
		t.Fatal("markup got through")
	}
}
