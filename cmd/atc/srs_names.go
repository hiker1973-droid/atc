package main

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

// SRS client names fix callsigns Whisper half-hears. Every SRS client is
// logged in under the pilot's DCS name ("Venom 203 | Yoda", "Raider 302|
// Jedi"), the server sends those names to every client, and each voice packet
// carries the sender's client GUID. Day 2026-10-19 one radio was heard as both
// "Venom 203" and "Venom 023", and Senaki Tower switched the emergency jet's
// callsign between them mid-approach.
//
// The fix works on the transcript, before any role parses it, so towers,
// Command, Hitman, Texaco, Marshal, Deckboss and Paddles all get it: the first
// callsign in the call with the SRS name's word ("Venom") whose number is a
// near miss of the SRS number (the same digits reordered, or one digit off) is
// rewritten to the SRS number. Anything else is left alone - a different name
// word, a number further off, or a call that already says the right number.

type srsNameBook struct {
	mu    sync.RWMutex
	names map[string]string // client GUID -> SRS name
}

var srsNames = &srsNameBook{names: make(map[string]string)}

// srsClient is the part of an SRS client record we read.
type srsClient struct {
	ClientGuid string
	Name       string
}

// note reads one SRS server message: a sync carries every client
// ("Clients"), an update carries one ("Client").
func (b *srsNameBook) note(line []byte) {
	var msg struct {
		Client  *srsClient
		Clients []srsClient
	}
	if json.Unmarshal(line, &msg) != nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	add := func(c srsClient) {
		if c.ClientGuid != "" && strings.TrimSpace(c.Name) != "" {
			b.names[c.ClientGuid] = strings.TrimSpace(c.Name)
		}
	}
	if msg.Client != nil {
		add(*msg.Client)
	}
	for _, c := range msg.Clients {
		add(c)
	}
}

func (b *srsNameBook) name(origin string) string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.names[origin]
}

// srsNameCallsignRe: "Venom 203 | Yoda" -> Venom, 203; "Raider 3-0-2" -> Raider, 3-0-2.
var srsNameCallsignRe = regexp.MustCompile(`^([A-Za-z]+)[\s-]*((?:\d[\s-]?){2,4})`)

// srsCallsign is the callsign word and number in an SRS name, "" when the
// name is not a callsign ("Yoda", "GCI Darkstar [BOT]").
func srsCallsign(name string) (word, num string) {
	if i := strings.IndexByte(name, '|'); i >= 0 {
		name = name[:i]
	}
	m := srsNameCallsignRe.FindStringSubmatch(strings.TrimSpace(name))
	if m == nil {
		return "", ""
	}
	return m[1], onlyDigits(m[2])
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// nearMiss: same length, not equal, and the same digits reordered or one off.
func nearMiss(heard, srs string) bool {
	if len(heard) != len(srs) || heard == srs {
		return false
	}
	diff := 0
	for i := range heard {
		if heard[i] != srs[i] {
			diff++
		}
	}
	if diff == 1 {
		return true
	}
	a, b := []byte(heard), []byte(srs)
	sort.Slice(a, func(i, j int) bool { return a[i] < a[j] })
	sort.Slice(b, func(i, j int) bool { return b[i] < b[j] })
	return string(a) == string(b)
}

// correctCallsignText rewrites a half-heard callsign in text to the one in
// the SRS name. ok is false when nothing changed.
func correctCallsignText(text, srsName string) (string, bool) {
	word, num := srsCallsign(srsName)
	if word == "" {
		return text, false
	}
	re := regexp.MustCompile(`(?i)\b(` + regexp.QuoteMeta(word) + `)\b([\s,.]*)((?:\d[\s-]?){2,4})`)
	locs := re.FindAllStringSubmatchIndex(text, -1)
	for _, m := range locs {
		if onlyDigits(text[m[6]:m[7]]) == num {
			return text, false // the call already says the right number
		}
	}
	for _, m := range locs {
		heard := text[m[6]:m[7]]
		if !nearMiss(onlyDigits(heard), num) {
			continue
		}
		// Keep a trailing space or dash the digits group swallowed.
		tail := heard[len(strings.TrimRight(heard, " -")):]
		return text[:m[6]] + num + tail + text[m[7]:], true
	}
	return text, false
}

// srsCorrect is correctCallsignText for the client that sent the call.
func srsCorrect(text, origin string) string {
	name := srsNames.name(origin)
	if name == "" {
		return text
	}
	fixed, ok := correctCallsignText(text, name)
	if ok {
		log.Info().Str("heard", text).Str("fixed", fixed).Str("srsName", name).
			Msg("callsign corrected from the SRS client name")
	}
	return fixed
}
