package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// The greenie board's data files, edited from the Stats page (operator
// 2026-11-06): the roster, the passes added by hand and the grade
// corrections. They live in this PC's home folder beside the costs file and
// are read by docs/paddles-report/greenie_post.py; only the dev box has them,
// so every call is a 404 anywhere else and the panel stays hidden.

var greenieFiles = map[string]struct {
	file   string
	header []string
}{
	"pilots":    {"readback-pilots.csv", []string{"callsign", "pilot", "board label", "group", "aliases"}},
	"passes":    {"readback-manual-passes.csv", []string{"date", "time", "callsign", "wire", "grade", "case", "note"}},
	"overrides": {"readback-grade-overrides.csv", []string{"date", "callsign", "wire", "grade", "note"}},
}

func greeniePath(name string) (string, []string, bool) {
	g, ok := greenieFiles[name]
	if !ok {
		return "", nil, false
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", nil, false
	}
	// Only a PC that runs the board has the roster file.
	if _, err := os.Stat(filepath.Join(home, greenieFiles["pilots"].file)); err != nil {
		return "", nil, false
	}
	return filepath.Join(home, g.file), g.header, true
}

// handleGreenieData: GET ?file=pilots|passes|overrides -> {header, rows}.
func handleGreenieData(w http.ResponseWriter, r *http.Request) {
	path, header, ok := greeniePath(r.URL.Query().Get("file"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	rows := [][]string{}
	if b, err := os.ReadFile(path); err == nil {
		b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
		cr := csv.NewReader(bytes.NewReader(b))
		cr.FieldsPerRecord = -1
		all, _ := cr.ReadAll()
		for i, rec := range all {
			if i == 0 && len(rec) > 0 && strings.EqualFold(rec[0], header[0]) {
				continue
			}
			for len(rec) < len(header) {
				rec = append(rec, "")
			}
			rows = append(rows, rec[:len(header)])
		}
	}
	writeJSON(w, map[string]any{"header": header, "rows": rows})
}

// handleGreenieSave: POST ?file= {rows:[[...]]} replaces the file (the old one is kept as .bak).
func handleGreenieSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	path, header, ok := greeniePath(r.URL.Query().Get("file"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	var in struct {
		Rows [][]string `json:"rows"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var buf bytes.Buffer
	buf.WriteString("\xef\xbb\xbf")
	cw := csv.NewWriter(&buf)
	cw.Write(header)
	n := 0
	for _, rec := range in.Rows {
		clean := make([]string, len(header))
		blank := true
		for i := range clean {
			if i < len(rec) {
				clean[i] = strings.TrimSpace(rec[i])
				if clean[i] != "" {
					blank = false
				}
			}
		}
		if !blank {
			cw.Write(clean)
			n++
		}
	}
	cw.Flush()
	if old, err := os.ReadFile(path); err == nil {
		os.WriteFile(path+".bak", old, 0o600)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "rows": n})
}

// greeniePython finds the Python that runs docs/paddles-report (READBACK_PYTHON, else the usual install).
func greeniePython() string {
	if p := os.Getenv("READBACK_PYTHON"); p != "" {
		return p
	}
	if la := os.Getenv("LOCALAPPDATA"); la != "" {
		p := filepath.Join(la, "Programs", "Python", "Python312", "python.exe")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "python"
}

// handleGreenieRefresh: POST -> redraw the board now and update the Discord message.
func handleGreenieRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	if _, _, ok := greeniePath("pilots"); !ok {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 150*time.Second)
	defer cancel()
	script := filepath.Join(rootDir, "docs", "paddles-report", "greenie_post.py")
	out, err := exec.CommandContext(ctx, greeniePython(), "-I", "-B", script, "--refresh").CombinedOutput()
	res := map[string]any{"ok": err == nil, "output": strings.TrimSpace(string(out))}
	if err != nil {
		res["error"] = err.Error()
	}
	writeJSON(w, res)
}
