package debrief

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RadioLine is one exchange on Readback's frequencies: a pilot's call as
// speech recognition heard it, a controller's reply, or a comms-report
// finding (near miss, no clearance, call not understood...).
type RadioLine struct {
	At      time.Time
	Station string // the log it came from: "atc-ugsb" -> "UGSB", "atc-command" -> "COMMAND"
	Kind    string // pilot | atc | report | unheard
	Text    string
}

// Radio is Readback's side of a session.
type Radio struct {
	Lines     []RadioLine
	Truncated int // lines left out past maxRadioLines
	Unheard   int // calls dropped as unintelligible
}

const maxRadioLines = 1500

// ReadRadio reads every role's log in logDir (atc-*.log and its rotated
// copies) for lines between from and to.
func ReadRadio(logDir string, from, to time.Time) (*Radio, error) {
	files, err := filepath.Glob(filepath.Join(logDir, "atc-*.log*"))
	if err != nil {
		return nil, err
	}
	rad := &Radio{}
	for _, path := range files {
		base := filepath.Base(path)
		if i := strings.Index(base, ".log"); i > 0 {
			base = base[:i]
		}
		station := strings.ToUpper(strings.TrimPrefix(base, "atc-"))
		if station == "SETUP" {
			continue
		}
		readRadioFile(path, station, from, to, rad)
	}
	sort.SliceStable(rad.Lines, func(i, j int) bool { return rad.Lines[i].At.Before(rad.Lines[j].At) })
	// One lost call can log twice (an echo, then its retry): keep one.
	kept := rad.Lines[:0]
	rad.Unheard = 0
	for _, l := range rad.Lines {
		if l.Kind == "unheard" {
			dup := false
			for k := len(kept) - 1; k >= 0 && l.At.Sub(kept[k].At) <= 2*time.Second; k-- {
				if kept[k].Kind == "unheard" && kept[k].Station == l.Station {
					dup = true
					break
				}
			}
			if dup {
				continue
			}
			rad.Unheard++
		}
		kept = append(kept, l)
	}
	rad.Lines = kept
	if n := len(rad.Lines); n > maxRadioLines {
		rad.Truncated = n - maxRadioLines
		rad.Lines = rad.Lines[n-maxRadioLines:] // the end of a session is where the landings are
	}
	return rad, nil
}

var radioMarkers = [][]byte{
	[]byte(`"message":"recognized"`), []byte(`"message":"Tower TX"`), []byte(`"message":"TX via `),
	[]byte(`"message":"comms report"`), []byte(`"message":"Whisper hallucination`), []byte(`"message":"Whisper prompt echo`),
}

func readRadioFile(path, station string, from, to time.Time, rad *Radio) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		b := sc.Bytes()
		hit := false
		for _, m := range radioMarkers {
			if bytes.Contains(b, m) {
				hit = true
				break
			}
		}
		if !hit {
			continue
		}
		var l struct {
			Time     time.Time `json:"time"`
			Message  string    `json:"message"`
			Text     string    `json:"text"`
			Kind     string    `json:"kind"`
			Callsign string    `json:"callsign"`
			Detail   string    `json:"detail"`
		}
		if json.Unmarshal(b, &l) != nil || l.Time.Before(from) || l.Time.After(to) {
			continue
		}
		line := RadioLine{At: l.Time, Station: station}
		switch {
		case l.Message == "recognized":
			line.Kind, line.Text = "pilot", l.Text
		case l.Message == "Tower TX" || strings.HasPrefix(l.Message, "TX via "):
			line.Kind, line.Text = "atc", l.Text
		case l.Message == "comms report":
			line.Kind, line.Text = "report", strings.TrimSpace(l.Kind+": "+l.Detail)
		default:
			rad.Unheard++
			line.Kind, line.Text = "unheard", "a call was not understood"
		}
		if line.Text != "" {
			rad.Lines = append(rad.Lines, line)
		}
	}
}
