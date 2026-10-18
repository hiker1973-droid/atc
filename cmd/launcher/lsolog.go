package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Paddles daily report (operator 2026-10-18: "run a daily paddles report on
// traps and data collected"). /api/log serves only the last 32 KB; the report
// (docs/paddles-report, run nightly on the dev box) needs the whole day, so
// /api/lso-log?date=YYYY-MM-DD returns every non-debug line of that local day
// from logs\atc-lso.log and its rotated copies (.4 oldest .. .1, then the live
// file). No date: yesterday.
func handleLSOLog(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		http.Error(w, "date must be YYYY-MM-DD", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if err := writeLSOLog(w, filepath.Join(rootDir, "logs"), date); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// writeLSOLog copies the lines of one day (the "time" field's date) from the
// Paddles logs in dir to w, oldest first.
func writeLSOLog(w io.Writer, dir, date string) error {
	want := `"time":"` + date + "T"
	for i := 4; i >= 0; i-- {
		path := filepath.Join(dir, "atc-lso.log")
		if i > 0 {
			path += fmt.Sprintf(".%d", i)
		}
		f, err := os.Open(path)
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			if strings.Contains(line, want) && !strings.Contains(line, `"level":"debug"`) {
				if _, err := io.WriteString(w, line+"\n"); err != nil {
					f.Close()
					return err
				}
			}
		}
		f.Close()
	}
	return nil
}
