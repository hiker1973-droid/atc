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

// Daily reports on the dev box (docs/paddles-report, docs/health-report)
// need whole days of logs; /api/log serves only the last 32 KB.
//
//	/api/lso-log?date=YYYY-MM-DD            Paddles (operator 2026-10-18: "run a
//	                                        daily paddles report")
//	/api/day-log?name=<role>&date=...       any role (operator 2026-10-18: "also
//	                                        do a daily health report on
//	                                        ATC/ATIS/Marshall")
//
// Every non-debug line of that local day, from the role's log and its rotated
// copies (.4 oldest .. .1, then the live file). No date: yesterday.
func handleLSOLog(w http.ResponseWriter, r *http.Request) {
	serveDayLog(w, r, "atc-lso.log")
}

func handleDayLog(w http.ResponseWriter, r *http.Request) {
	role := findRole(r.URL.Query().Get("name"))
	if role == nil {
		http.Error(w, "unknown role", http.StatusNotFound)
		return
	}
	serveDayLog(w, r, role.LogFile)
}

func serveDayLog(w http.ResponseWriter, r *http.Request, file string) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		http.Error(w, "date must be YYYY-MM-DD", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if err := writeDayLog(w, filepath.Join(rootDir, "logs"), file, date); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// writeDayLog copies the lines of one day (the "time" field's date) from the
// log file in dir and its rotated copies to w, oldest first.
func writeDayLog(w io.Writer, dir, file, date string) error {
	want := `"time":"` + date + "T"
	for i := 4; i >= 0; i-- {
		path := filepath.Join(dir, file)
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
