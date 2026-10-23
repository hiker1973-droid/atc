package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vsfg7/atc/pkg/debrief"
)

const testACMI = "FileType=text/acmi/tacview\nFileVersion=2.2\n" +
	"0,ReferenceTime=2016-06-21T10:00:00Z,RecordingTime=2026-10-23T00:15:00Z,Title=Test Mission\n#0\n" +
	"101,T=42.0|42.2|50,Type=Air+FixedWing,Name=F-16C_50,Pilot=Dabs,Group=Venom 1,Coalition=Allies\n" +
	"#600\n0,Event=TakenOff|101|Dabs has taken off from Senaki-Kolkhi.\n" +
	"#3000\n0,Event=Landed|101|Dabs has landed at Senaki-Kolkhi.\n"

// TestReportFlow: upload a recording, preview, generate against a stand-in
// Claude, then read and delete the saved debrief.
func TestReportFlow(t *testing.T) {
	defer func(r string) { rootDir = r }(rootDir)
	rootDir = t.TempDir()
	os.MkdirAll(filepath.Join(rootDir, "logs"), 0o755)
	os.WriteFile(filepath.Join(rootDir, "logs", "atc-ugsb.log"),
		[]byte(`{"level":"info","text":"Senaki Tower, Dabs, request taxi.","time":"2026-10-23T00:20:00Z","message":"recognized"}`+"\n"), 0o644)
	oldEnv := userEnv
	defer func() { userEnv = oldEnv }()
	key := ""
	userEnv = func(name string) string {
		if name == "ANTHROPIC_API_KEY" {
			return key
		}
		return ""
	}
	var sent string
	claude := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct{ Content string } `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		sent = body.Messages[0].Content
		w.Write([]byte(`{"content":[{"type":"text","text":"## Summary\nDabs flew a clean sortie."}],"stop_reason":"end_turn","usage":{"input_tokens":900,"output_tokens":120}}`))
	}))
	defer claude.Close()
	defer func(a string) { debrief.API = a }(debrief.API)
	debrief.API = claude.URL

	// Preview from an upload.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("acmi", "test.acmi")
	fw.Write([]byte(testACMI))
	mw.WriteField("radio", "1")
	mw.WriteField("matchTacview", "1")
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/reports/prepare", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	handleReportPrepare(rec, req)
	if rec.Code != 200 {
		t.Fatalf("prepare %d: %s", rec.Code, rec.Body)
	}
	var prep struct {
		ID, Title, Records string
	}
	json.Unmarshal(rec.Body.Bytes(), &prep)
	if !strings.Contains(prep.Records, "Dabs - F-16C_50") || !strings.Contains(prep.Records, "request taxi") ||
		!strings.HasPrefix(prep.Title, "Test Mission - 23 Oct 2026 00:15Z") {
		t.Fatalf("title %q records:\n%s", prep.Title, prep.Records)
	}

	gen := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handleReportGenerate(rec, httptest.NewRequest(http.MethodPost, "/api/reports/generate",
			strings.NewReader(`{"id":"`+prep.ID+`","model":"claude-sonnet-5-5"}`)))
		return rec
	}
	if rec := gen(); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "no Claude API key") {
		t.Fatalf("without a key: %d %s", rec.Code, rec.Body)
	}
	key = "sk-ant-test-key-0123456789"
	rec = gen()
	if rec.Code != 200 || sent != prep.Records {
		t.Fatalf("generate %d %s (sent the preview: %v)", rec.Code, rec.Body, sent == prep.Records)
	}
	var out struct{ Meta reportMeta }
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Meta.Usage.InputTokens != 900 || out.Meta.ReportNo != "AAR-20261023-0015" || out.Meta.Mission != "Test Mission" || len(out.Meta.Facts) == 0 {
		t.Fatalf("report meta %+v", out.Meta)
	}

	// The AAR page: header, data block, Claude's sections, escaped.
	rec = httptest.NewRecorder()
	handleAARPage(rec, httptest.NewRequest(http.MethodGet, "/reports/aar?name="+out.Meta.Name, nil))
	page := rec.Body.String()
	for _, want := range []string{"AFTER ACTION REPORT", "AAR-20261023-0015", "<th>Sorties</th><td>1 takeoffs</td>", "<h2>Summary</h2>", "clean sortie", "Reviewed and approved"} {
		if !strings.Contains(page, want) {
			t.Errorf("AAR page lacks %q", want)
		}
	}
	rec = httptest.NewRecorder()
	handleReportFile(rec, httptest.NewRequest(http.MethodGet, "/api/reports/file?download=1&name="+out.Meta.Name, nil))
	if !strings.Contains(rec.Body.String(), "| Mission | Test Mission |") || !strings.Contains(rec.Header().Get("Content-Disposition"), "AAR-20261023-0015 Test Mission.md") {
		t.Fatalf("download %q\n%s", rec.Header().Get("Content-Disposition"), rec.Body)
	}
	if s := savedReports(); len(s) != 1 || s[0].Name != out.Meta.Name {
		t.Fatalf("saved %+v", s)
	}

	rec = httptest.NewRecorder()
	handleReportFile(rec, httptest.NewRequest(http.MethodGet, "/api/reports/file?name="+out.Meta.Name, nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "clean sortie") {
		t.Fatalf("read back %d %s", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	handleReportFile(rec, httptest.NewRequest(http.MethodGet, "/api/reports/file?name=..%5Csettings", nil))
	if rec.Code == http.StatusOK {
		t.Fatalf("a path for a name was served")
	}
}

// A path the page sends must be one of the listed files.
func TestReportLocalPathMustBeListed(t *testing.T) {
	list := []localSource{{Path: `C:\Users\x\Saved Games\DCS\Logs\dcs.log`}}
	if _, err := pickLocal(`C:\Windows\win.ini`, list); err == nil {
		t.Fatal("an unlisted file was accepted")
	}
	if p, err := pickLocal(`c:\users\x\saved games\dcs\logs\dcs.log`, list); err != nil || p != list[0].Path {
		t.Fatalf("listed file: %q %v", p, err)
	}
}
