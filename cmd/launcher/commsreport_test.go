package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCommsReportReadsTheWindowNewestFirst(t *testing.T) {
	defer func(r string) { rootDir = r }(rootDir)
	rootDir = t.TempDir()
	dir := commsReportDir()
	os.MkdirAll(dir, 0o755)
	now := time.Now().UTC()
	line := func(at time.Time, kind, cs string) string {
		b, _ := json.Marshal(CommsLine{Time: at, Airfield: "UGKS", Tower: "Senaki Tower", Kind: kind, Callsign: cs, Detail: "d"})
		return string(b)
	}
	today := []string{
		line(now.Add(-2*time.Hour), "near_miss", "Raider 031"),
		line(now.Add(-30*time.Minute), "departed_without_clearance", "Venom 201"),
		"not json",
	}
	os.WriteFile(filepath.Join(dir, now.Format("2006-01-02")+".jsonl"), []byte(strings.Join(today, "\n")+"\n"), 0o644)
	old := now.AddDate(0, 0, -3)
	os.WriteFile(filepath.Join(dir, old.Format("2006-01-02")+".jsonl"), []byte(line(old, "no_callsign", "")+"\n"), 0o644)

	rec := httptest.NewRecorder()
	handleCommsReport(rec, httptest.NewRequest("GET", "/api/comms-report?hours=24", nil))
	var rep CommsReport
	if err := json.Unmarshal(rec.Body.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Events) != 2 || rep.Events[0].Callsign != "Venom 201" {
		t.Fatalf("want the two events of the last 24 h, newest first: %+v", rep.Events)
	}
	if rep.Counts["near_miss"] != 1 || rep.Counts["no_callsign"] != 0 {
		t.Errorf("counts = %v", rep.Counts)
	}

	rec = httptest.NewRecorder()
	handleCommsReport(rec, httptest.NewRequest("GET", "/api/comms-report?hours=168", nil))
	json.Unmarshal(rec.Body.Bytes(), &rep)
	if len(rep.Events) != 3 {
		t.Errorf("a week should include the old event: %+v", rep.Events)
	}
}
