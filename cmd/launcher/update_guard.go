package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// recentPilotWindow is how far back an Update looks for pilot radio calls
// before it refuses to restart the roles under a live session.
const recentPilotWindow = 10 * time.Minute

// heardMessages are the role log messages that mean a pilot's call was heard
// (the same set the nightly Health report counts).
var heardMessages = map[string]bool{
	"recognized":     true,
	"Command heard":  true,
	"Marshal heard":  true,
	"Deckboss heard": true,
}

// lastPilotCall returns the time of the newest heard pilot call in any
// logs/atc-*.log on this rig, or the zero time when there is none within
// the window. Only the tail of each log is read.
func lastPilotCall(logDir string, since time.Time) time.Time {
	var newest time.Time
	files, _ := filepath.Glob(filepath.Join(logDir, "atc-*.log"))
	for _, p := range files {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		const tail = 256 * 1024
		if st, err := f.Stat(); err == nil && st.Size() > tail {
			f.Seek(st.Size()-tail, io.SeekStart)
		}
		data, _ := io.ReadAll(f)
		f.Close()
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.Contains(line, `"message"`) {
				continue
			}
			var e struct {
				Time    time.Time `json:"time"`
				Message string    `json:"message"`
			}
			if json.Unmarshal([]byte(line), &e) != nil || !heardMessages[e.Message] {
				continue
			}
			if e.Time.After(since) && e.Time.After(newest) {
				newest = e.Time
			}
		}
	}
	return newest
}
