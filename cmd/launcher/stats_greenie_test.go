package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGreenieDataRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	// No roster file: a PC that doesn't run the board answers 404.
	rec := httptest.NewRecorder()
	handleGreenieData(rec, httptest.NewRequest("GET", "/api/greenie/data?file=passes", nil))
	if rec.Code != 404 {
		t.Fatalf("without a roster: %d, want 404", rec.Code)
	}

	os.WriteFile(filepath.Join(home, "readback-pilots.csv"), []byte("callsign,pilot\nRaider 1,Ace\n"), 0o600)
	body := `{"rows":[["2026-11-05","21:00","Raider 312","4","OK","III","note"],["","","","","","",""]]}`
	rec = httptest.NewRecorder()
	handleGreenieSave(rec, httptest.NewRequest("POST", "/api/greenie/save?file=passes", strings.NewReader(body)))
	if rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handleGreenieData(rec, httptest.NewRequest("GET", "/api/greenie/data?file=passes", nil))
	var got struct {
		Header []string
		Rows   [][]string
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Rows) != 1 || got.Rows[0][2] != "Raider 312" || got.Rows[0][5] != "III" || len(got.Header) != 7 {
		t.Fatalf("round trip = %+v", got)
	}
	// An unknown file is refused.
	rec = httptest.NewRecorder()
	handleGreenieData(rec, httptest.NewRequest("GET", "/api/greenie/data?file=../x", nil))
	if rec.Code != 404 {
		t.Errorf("unknown file: %d, want 404", rec.Code)
	}
}
