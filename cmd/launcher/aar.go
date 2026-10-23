package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vsfg7/atc/pkg/debrief"
)

// The After Action Report as a page and a PDF (operator 2026-10-23: "we want
// it to be a after action report" - standard military AAR, branded PDF).
// renderAAR builds one self-contained HTML document - styles inline, the
// squadron logo embedded - shown in the Reports page and printed to PDF by
// Microsoft Edge from a temporary file, so printing never depends on this
// launcher's own HTTP access rules (Cloudflare Access on the rigs).

var aarTemplate = template.Must(template.New("aar").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>{{.FileName}}</title>
<style>
  @page { size: Letter; margin: 0.6in 0.65in 0.75in;
    @bottom-left { content: "{{.Squadron}} · {{.ReportNo}}"; font: 8pt Arial, sans-serif; color: #7a808a; }
    @bottom-right { content: "Page " counter(page) " of " counter(pages); font: 8pt Arial, sans-serif; color: #7a808a; } }
  * { box-sizing: border-box; }
  html { background: #e9ebee; }
  body { font-family: Arial, Helvetica, sans-serif; font-size: 10.5pt; line-height: 1.45; color: #1d2128;
    background: #fff; max-width: 8.5in; margin: 18px auto; padding: 0.6in 0.65in; box-shadow: 0 2px 14px rgba(0,0,0,.18); }
  .head { display: grid; grid-template-columns: 1fr auto 1fr; align-items: center; gap: 14px; }
  .head .logo img { max-height: 74px; max-width: 2.6in; }
  .head .centre { text-align: center; }
  .head .unit { font-size: 10pt; font-weight: 700; letter-spacing: .12em; text-transform: uppercase; color: #3a3f47; }
  .head .doc { font-size: 19pt; font-weight: 800; letter-spacing: .06em; color: #1d1f23; margin: 2px 0; }
  .head .mission { font-size: 10.5pt; color: #3a3f47; }
  .head .no { text-align: right; font-size: 9pt; color: #3a3f47; }
  .head .no b { display: block; font-size: 10.5pt; color: #1d1f23; }
  .rule { height: 4px; background: #ff6a2b; margin: 10px 0 2px; }
  .rule2 { height: 1px; background: #1d1f23; margin: 0 0 12px; }
  table.data { width: 100%; border-collapse: collapse; margin: 0 0 14px; font-size: 9.5pt; }
  table.data th { text-align: left; width: 1.45in; padding: 3px 8px; background: #f1f2f4; border: 1px solid #d6d9de; color: #3a3f47; font-weight: 700; vertical-align: top; }
  table.data td { padding: 3px 8px; border: 1px solid #d6d9de; vertical-align: top; }
  .body h1 { font-size: 15pt; }
  .body h2 { font-size: 12pt; text-transform: uppercase; letter-spacing: .04em; color: #1d1f23; border-bottom: 1.5px solid #1d1f23;
    padding-bottom: 2px; margin: 16px 0 6px; }
  .body h3 { font-size: 10.5pt; color: #c44d14; margin: 10px 0 3px; }
  .body p { margin: 4px 0; }
  .body ul, .body ol { margin: 3px 0 6px; padding-left: 20px; }
  .body li { margin: 2px 0; }
  .body code { background: #f1f2f4; padding: 0 3px; font-size: 9pt; }
  .body table { border-collapse: collapse; margin: 6px 0; font-size: 9.5pt; }
  .body th, .body td { border: 1px solid #d6d9de; padding: 3px 7px; text-align: left; vertical-align: top; }
  .body th { background: #f1f2f4; }
  .sign { display: grid; grid-template-columns: 1fr 1fr; gap: 28px; margin-top: 26px; font-size: 9pt; color: #3a3f47; }
  .sign div { border-top: 1px solid #1d1f23; padding-top: 3px; }
  .sources { margin-top: 18px; font-size: 7.8pt; color: #7a808a; border-top: 1px solid #d6d9de; padding-top: 5px; }
  @media print {
    html { background: #fff; }
    body { margin: 0; padding: 0; max-width: none; box-shadow: none; }
    tr, li, .sign { break-inside: avoid; }
    h2, h3 { break-after: avoid; }
    .rule, table.data th { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
  }
</style>
</head>
<body>
{{if .ReportNo}}
<div class="head">
  <div class="logo">{{if .Logo}}<img src="{{.Logo}}" alt="">{{end}}</div>
  <div class="centre">
    <div class="unit">{{.Squadron}}</div>
    <div class="doc">AFTER ACTION REPORT</div>
    <div class="mission">{{.Mission}}</div>
  </div>
  <div class="no"><b>{{.ReportNo}}</b>{{.Date}}</div>
</div>
<div class="rule"></div><div class="rule2"></div>
<table class="data">{{range .Facts}}<tr><th>{{.Label}}</th><td>{{.Value}}</td></tr>{{end}}</table>
{{else}}
<div class="head"><div class="logo">{{if .Logo}}<img src="{{.Logo}}" alt="">{{end}}</div><div class="centre"><div class="unit">{{.Squadron}}</div></div><div></div></div>
<div class="rule"></div><div class="rule2"></div>
{{end}}
<div class="body">{{.Body}}</div>
{{if .ReportNo}}
<div class="sign"><div>Prepared by (name, callsign) &nbsp;/&nbsp; date</div><div>Reviewed and approved by (Commanding Officer) &nbsp;/&nbsp; date</div></div>
{{end}}
<div class="sources">Prepared with Readback ATC on {{.Written}} by {{.Model}} from {{.Sources}}. Kills marked "likely" are inferred from
  weapon impact points; pilot radio calls are as speech recognition heard them.</div>
</body>
</html>
`))

// squadronLogoDataURI is the logo the dashboard shows, embedded.
func squadronLogoDataURI() template.URL {
	brandingMu.Lock()
	p := customLogo(readBranding())
	brandingMu.Unlock()
	var data []byte
	if p != "" {
		data, _ = os.ReadFile(p)
	}
	if len(data) == 0 {
		data, _ = uiFS.ReadFile(builtInLogo())
	}
	if len(data) == 0 {
		return ""
	}
	return template.URL("data:" + http.DetectContentType(data) + ";base64," + base64.StdEncoding.EncodeToString(data))
}

func renderAAR(meta reportMeta, md string) ([]byte, error) {
	squadron, _ := brandingSnapshot()["name"].(string)
	date := ""
	if !meta.From.IsZero() {
		date = meta.From.UTC().Format("2 Jan 2006")
	}
	for _, f := range meta.Facts {
		if f.Label == "Date" {
			date = f.Value
		}
	}
	var buf bytes.Buffer
	err := aarTemplate.Execute(&buf, map[string]any{
		"FileName": reportFileName(meta),
		"Squadron": squadron,
		"ReportNo": meta.ReportNo,
		"Mission":  meta.Mission,
		"Date":     date,
		"Facts":    meta.Facts,
		"Logo":     squadronLogoDataURI(),
		"Body":     template.HTML(debrief.HTML(md)),
		"Written":  meta.Created.Format("2 Jan 2006 15:04"),
		"Model":    meta.Model,
		"Sources":  strings.Join(meta.Sources, "; "),
	})
	return buf.Bytes(), err
}

// handleAARPage: GET /reports/aar?name= the report as its printable page.
func handleAARPage(w http.ResponseWriter, r *http.Request) {
	meta, md, err := loadReport(r.URL.Query().Get("name"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	page, err := reportPage(meta, md)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(page)
}

// reportPage is a report's page: the saved debrief-style AAR (.html), or an
// earlier report drawn from its Markdown.
func reportPage(meta reportMeta, md string) ([]byte, error) {
	if meta.HTML {
		return os.ReadFile(filepath.Join(reportsDir(), meta.Name+".html"))
	}
	return renderAAR(meta, md)
}

// handleAARPNG: GET /api/reports/png?name= the report's first page as a
// picture, for the Discord post. Made by Edge on first request and kept.
func handleAARPNG(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	meta, md, err := loadReport(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	pngPath := filepath.Join(reportsDir(), name+".png")
	img, err := os.ReadFile(pngPath)
	if err != nil {
		page, err := reportPage(meta, md)
		if err == nil {
			img, err = screenshotPNG(r.Context(), page)
		}
		if err != nil {
			http.Error(w, "could not make the picture: "+err.Error(), http.StatusInternalServerError)
			return
		}
		os.WriteFile(pngPath, img, 0o644)
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.png"`, reportFileName(meta)))
	w.Write(img)
}

// handleAARPDF: GET /api/reports/pdf?name= the report as a PDF, printed by
// Edge on first request and kept beside the report.
func handleAARPDF(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	meta, md, err := loadReport(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	pdfPath := filepath.Join(reportsDir(), name+".pdf")
	pdf, err := os.ReadFile(pdfPath)
	if err != nil || r.URL.Query().Get("fresh") == "1" {
		page, err := reportPage(meta, md)
		if err == nil {
			pdf, err = printPDF(r.Context(), page)
		}
		if err != nil {
			http.Error(w, "could not make the PDF: "+err.Error()+" - use Print instead", http.StatusInternalServerError)
			return
		}
		os.WriteFile(pdfPath, pdf, 0o644)
	}
	disp := "inline"
	if r.URL.Query().Get("download") == "1" {
		disp = "attachment"
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s.pdf"`, disp, reportFileName(meta)))
	w.Write(pdf)
}

func findEdge() string {
	for _, p := range []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), `Microsoft\Edge\Application\msedge.exe`),
		filepath.Join(os.Getenv("ProgramFiles"), `Microsoft\Edge\Application\msedge.exe`),
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// printPDF prints a page with headless Edge and a throwaway profile.
func printPDF(ctx context.Context, page []byte) ([]byte, error) {
	edge := findEdge()
	if edge == "" {
		return nil, fmt.Errorf("Microsoft Edge was not found")
	}
	dir, err := os.MkdirTemp("", "readback-aar-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	src, out := filepath.Join(dir, "aar.html"), filepath.Join(dir, "aar.pdf")
	if err := os.WriteFile(src, page, 0o644); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, edge, "--headless", "--disable-gpu", "--no-pdf-header-footer", "--no-first-run",
		"--user-data-dir="+filepath.Join(dir, "profile"), "--print-to-pdf="+out,
		"file:///"+filepath.ToSlash(src))
	if b, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(string(b)))
	}
	for i := 0; i < 50; i++ { // Edge can return before the file is complete
		if st, err := os.Stat(out); err == nil && st.Size() > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	return os.ReadFile(out)
}

// screenshotPNG takes the first page of a report (1056 x 816, landscape
// Letter at 96 dpi with the page's margin) with headless Edge.
func screenshotPNG(ctx context.Context, page []byte) ([]byte, error) {
	edge := findEdge()
	if edge == "" {
		return nil, fmt.Errorf("Microsoft Edge was not found")
	}
	dir, err := os.MkdirTemp("", "readback-aar-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	src, out := filepath.Join(dir, "aar.html"), filepath.Join(dir, "aar.png")
	if err := os.WriteFile(src, page, 0o644); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, edge, "--headless", "--disable-gpu", "--hide-scrollbars", "--no-first-run",
		"--user-data-dir="+filepath.Join(dir, "profile"), "--window-size=1010,790", "--screenshot="+out,
		"file:///"+filepath.ToSlash(src))
	if b, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(string(b)))
	}
	return os.ReadFile(out)
}
