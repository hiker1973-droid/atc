package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const sampleServerLog = `=== Log opened UTC 2026-10-22 18:00:00
2026-10-22 18:00:00.000 INFO    Scripting (Main): event:type=mission start,event_id=1,t=0,linked_event_id=0,
2026-10-22 18:01:00.000 INFO    Scripting (Main): event:type=takeoff,initiatorPilotName=VENOM 1 | DABS,place=CVN-72,t=60,initiator_unit_type=FA-18C_hornet,initiator_coalition=2,initiator_ws_type1=1,event_id=2,linked_event_id=0,
2026-10-22 18:31:00.000 INFO    Scripting (Main): event:type=landing quality mark,initiatorPilotName=VENOM 1 | DABS,place=CVN-72,t=1860,initiator_unit_type=FA-18C_hornet,comment=LSO: GRADE:_OK_ : WIRE# 3,initiator_coalition=2,event_id=3,linked_event_id=0,
2026-10-22 18:31:00.000 INFO    Scripting (Main): event:type=land,initiatorPilotName=VENOM 1 | DABS,place=CVN-72,t=1860,initiator_unit_type=FA-18C_hornet,initiator_coalition=2,initiator_ws_type1=1,event_id=4,linked_event_id=0,
`

func TestStatsUploadAndExport(t *testing.T) {
	defer func(r string) { rootDir = r }(rootDir)
	rootDir = t.TempDir()
	t.Setenv("USERPROFILE", t.TempDir()) // no DCS logs of this PC's own
	upload := func(body string) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("log", "dcs.log")
		fw.Write([]byte(body))
		mw.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/stats/upload", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		rec := httptest.NewRecorder()
		handleStatsUpload(rec, req)
		return rec
	}
	if rec := upload("2026-10-22 18:00:00.000 INFO    EDCORE (Main): a game client's log\n"); rec.Code != http.StatusBadRequest {
		t.Fatalf("a client log was kept: %d", rec.Code)
	}
	if rec := upload(sampleServerLog); rec.Code != 200 {
		t.Fatalf("upload %d %s", rec.Code, rec.Body)
	}
	upload(sampleServerLog) // the same log twice: its run counts once

	rec := httptest.NewRecorder()
	handleStats(rec, httptest.NewRequest(http.MethodGet, "/api/stats?period=all", nil))
	var got struct {
		Sessions int
		Pilots   []statsPilot
		Board    []statsBoard
	}
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Sessions != 1 || len(got.Pilots) != 1 || got.Pilots[0].Name != "Dabs" || got.Pilots[0].FlightMin != 30 || got.Pilots[0].Traps != 1 {
		t.Fatalf("stats %+v", got)
	}
	if len(got.Board) != 1 || got.Board[0].Avg != 5 || got.Board[0].Passes[0].Fill != "#00843d" {
		t.Fatalf("board %+v", got.Board)
	}

	rec = httptest.NewRecorder()
	handleStatsCSV(rec, httptest.NewRequest(http.MethodGet, "/api/stats/csv?kind=passes&period=all", nil))
	if body := rec.Body.String(); !strings.Contains(body, "Dabs,_OK_,3,5,CVN-72") || !strings.Contains(rec.Header().Get("Content-Disposition"), "greenie board - all time.csv") {
		t.Fatalf("passes CSV %q\n%s", rec.Header().Get("Content-Disposition"), body)
	}
	rec = httptest.NewRecorder()
	handleStatsCSV(rec, httptest.NewRequest(http.MethodGet, "/api/stats/csv?kind=pilots&period=all", nil))
	if body := rec.Body.String(); !strings.Contains(body, "Dabs,blue,F/A-18C,1,0.5,1,1,0,0,0,0,0,0,0") {
		t.Fatalf("pilots CSV\n%s", body)
	}
	tot, _, label := statsTotals(httptest.NewRequest(http.MethodGet, "/api/stats?period=all", nil))
	if page := statsHTML(tot, label); !strings.Contains(page, "<b>Dabs</b>") || !strings.Contains(page, "Greenie board") || !strings.Contains(page, "#00843d") {
		t.Fatal("stats page lacks the pilot or the board")
	}
}
