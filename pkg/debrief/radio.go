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
	At       time.Time
	Station  string // the log it came from: "atc-ugsb" -> "UGSB", "atc-command" -> "COMMAND"; a Command channel: "TEXACO 3-1"
	Kind     string // pilot | atc | report | unheard
	Text     string
	Callsign string // the aircraft it was from or to: "Raider 302" ("" when not known)
	Report   string // a comms report's kind: "no_clearance"
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
	[]byte(`"message":"ATC request"`),
	// Command (with the tanker and Paddles channels it runs) and Marshal
	// log their own way.
	[]byte(`"message":"Command heard"`), []byte(`"message":"Command transcribed"`), []byte(`"message":"Command TX"`),
	[]byte(`"message":"Marshal heard"`), []byte(`"message":"Marshal TX"`),
}

// channelStation names a Command channel as a station: "Texaco-3-1" -> "TEXACO 3-1".
func channelStation(ch string) string {
	ch = strings.ToUpper(strings.TrimSpace(ch))
	if strings.Contains(ch, "COMMAND") { // "vSFG-7-Command": Command's own
		return "COMMAND"
	}
	if i := strings.IndexByte(ch, '-'); i > 0 {
		ch = ch[:i] + " " + ch[i+1:]
	}
	return ch
}

func readRadioFile(path, station string, from, to time.Time, rad *Radio) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	lastPilot := -1                  // this file's last pilot call, for the line that names its callsign
	chanOf := map[string]RadioLine{} // each callsign's last call on a Command channel, for Command's replies
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
			Channel  string    `json:"channel"`
		}
		if json.Unmarshal(b, &l) != nil || l.Time.Before(from) || l.Time.After(to) {
			continue
		}
		line := RadioLine{At: l.Time, Station: station}
		switch {
		case l.Message == "ATC request" || l.Message == "Command transcribed":
			// the call just logged, with the callsign Readback matched it to
			if lastPilot >= 0 && l.Time.Sub(rad.Lines[lastPilot].At) <= 3*time.Second {
				p := &rad.Lines[lastPilot]
				// the callsign Readback matched, unless it is the bare "302" and the words had more
				if CallsignKey(l.Callsign) != "" && (p.Callsign == "" || hasLetter(l.Callsign)) {
					p.Callsign = l.Callsign
				}
				if p.Callsign != "" && p.Station != station {
					chanOf[CallsignKey(p.Callsign)] = *p
				}
			}
			continue
		case l.Message == "recognized" || l.Message == "Command heard" || l.Message == "Marshal heard":
			line.Kind, line.Text = "pilot", l.Text
			line.Callsign = CallsignIn(l.Text)
			if l.Channel != "" {
				line.Station = channelStation(l.Channel)
				if line.Callsign != "" {
					chanOf[CallsignKey(line.Callsign)] = line
				}
			}
		case l.Message == "Tower TX" || strings.HasPrefix(l.Message, "TX via ") || l.Message == "Command TX" || l.Message == "Marshal TX":
			line.Kind, line.Text = "atc", l.Text
			line.Callsign = CallsignIn(l.Text)
			if line.Callsign == "" && CallsignKey(l.Callsign) != "" {
				line.Callsign = l.Callsign
			}
			if l.Message == "Command TX" { // on the channel the call it answers came in on
				if c, ok := chanOf[CallsignKey(line.Callsign)]; ok && line.Callsign != "" && l.Time.Sub(c.At) < 90*time.Second {
					line.Station = c.Station
				} else if line.Callsign == "" && digitsOf(l.Callsign) != "" { // logged as "302": that number's last call
					for k, c := range chanOf {
						if digitsOf(k) == digitsOf(l.Callsign) && l.Time.Sub(c.At) < 90*time.Second {
							line.Station, line.Callsign = c.Station, c.Callsign
						}
					}
				}
			}
		case l.Message == "comms report":
			line.Kind, line.Text = "report", strings.TrimSpace(l.Kind+": "+l.Detail)
			line.Callsign, line.Report = l.Callsign, l.Kind
		default:
			rad.Unheard++
			line.Kind, line.Text = "unheard", "a call was not understood"
		}
		if line.Text != "" {
			rad.Lines = append(rad.Lines, line)
			if line.Kind == "pilot" {
				lastPilot = len(rad.Lines) - 1
			}
		}
	}
}
