package debrief

import (
	"bufio"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ServerLog is the summary of a dcs.log: the server-health side of a
// debrief. The log is mostly the engine's own chatter, so this keeps the
// session's span, the mission loads, player and network lines, and the
// errors (grouped, with counts).
type ServerLog struct {
	Start, End time.Time
	Events     []string  // "21:44:10Z NET: ..." in order, capped
	Problems   []Problem // ERROR and ALERT lines grouped, most frequent first
	Levels     map[string]int
}

// Problem is one kind of error with how often it came up.
type Problem struct {
	Level, Source, Text string
	Count               int
	First               time.Time
}

const (
	maxServerEvents   = 150
	maxServerProblems = 40
)

var (
	dcsLine = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\.\d+ (\w+)\s+([^:]+): (.*)$`)
	numbers = regexp.MustCompile(`\d+(\.\d+)?`)
	// interesting picks the lines about the session rather than the engine.
	interesting = regexp.MustCompile(`(?i)\b(mission|loadMission|player|client|connect|disconnect|kick|ban|slot|crash|eject|shot down|killed|pilot dead|server started|server stopped|pause|unpause|simulation|start_mission|stop_mission)\b`)
	// noise is engine chatter that matches the words above anyway.
	noise = regexp.MustCompile(`(?i)degenerate triangle|Loaded C:|Added sound path|_crash'|lua-.*\.dll|terrain|texture|shader|font`)
)

// ParseDCSLogFile reads a dcs.log.
func ParseDCSLogFile(path string) (*ServerLog, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseDCSLog(f)
}

// ParseDCSLog summarises a dcs.log stream (its times are UTC).
func ParseDCSLog(r io.Reader) (*ServerLog, error) {
	s := &ServerLog{Levels: map[string]int{}}
	problems := map[string]*Problem{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 256*1024), 4*1024*1024)
	for sc.Scan() {
		m := dcsLine.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		t, err := time.Parse("2006-01-02 15:04:05", m[1])
		if err != nil {
			continue
		}
		if s.Start.IsZero() {
			s.Start = t
		}
		s.End = t
		level, source, text := m[2], strings.TrimSpace(m[3]), strings.TrimSpace(m[4])
		s.Levels[level]++
		switch level {
		case "ERROR", "ALERT", "ERROR_ONCE":
			key := level + "|" + source + "|" + numbers.ReplaceAllString(text, "#")
			if p := problems[key]; p != nil {
				p.Count++
			} else {
				problems[key] = &Problem{Level: level, Source: source, Text: clip(text, 220), Count: 1, First: t}
			}
		}
		if (strings.HasPrefix(source, "NET") || strings.HasPrefix(source, "Dispatcher") || strings.HasPrefix(source, "WORLDGENERAL") ||
			strings.HasPrefix(source, "APP") || interesting.MatchString(text)) && !noise.MatchString(text) && len(s.Events) < maxServerEvents {
			s.Events = append(s.Events, t.Format("15:04:05Z")+" "+source+": "+clip(text, 200))
		}
	}
	for _, p := range problems {
		s.Problems = append(s.Problems, *p)
	}
	sort.Slice(s.Problems, func(i, j int) bool {
		if s.Problems[i].Count != s.Problems[j].Count {
			return s.Problems[i].Count > s.Problems[j].Count
		}
		return s.Problems[i].First.Before(s.Problems[j].First)
	})
	if len(s.Problems) > maxServerProblems {
		s.Problems = s.Problems[:maxServerProblems]
	}
	return s, sc.Err()
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
