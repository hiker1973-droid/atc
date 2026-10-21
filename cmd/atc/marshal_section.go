package main

import (
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/vsfg7/atc/pkg/state"
)

// Sections check in together (operator 2026-10-20): "Union Marshal, Raider 302
// holding hands with 032, marking mom's ...". They hold as one, get one
// Charlie, and land one at a time; Paddles grades each jet on its own pass.
// On Day 2026-10-20 "Raider 303 holding hands with Raider 319" and "Raider 302
// and 031" registered only the lead, so the wingmen were nowhere in the stack.
//
// The section is the lead's stack entry with its Wingmen; each wingman is an
// alias of the lead (marshal_alias.go), so "032 established" or "319 see you
// at ten" reach the same entry, and the lead's "commencing" takes the section
// out of the stack.

// sectionJoinRe is one more jet named right after the lead's callsign:
// "holding hands with 032", "with Raider 319", "and 0-3-1".
var sectionJoinRe = regexp.MustCompile(`(?i)^\s*,?\s*(?:holding hands with|holding hands|in company with|with|and|plus)\s+(?:([a-z]{3,})\s+)?(\d(?:[\s-]?\d){2})\b`)

// marshalWingmen returns the wingmen a check-in names after the lead.
func marshalWingmen(text, lead string) []string {
	rest, ok := splitMarshalAddress(text)
	if !ok {
		rest = text
	}
	rest = normalizeCallsignLocal(rest)
	word, num := splitCallsign(lead)
	if num == "" {
		return nil
	}
	// Find the lead's callsign token, then read the names that follow it.
	pos := -1
	for _, m := range callsignTokenRe.FindAllStringSubmatchIndex(rest, -1) {
		digits := strings.NewReplacer(" ", "", "-", "").Replace(rest[m[4]:m[5]])
		if strings.EqualFold(rest[m[2]:m[3]], word) && digits == num {
			pos = m[1]
			break
		}
	}
	if pos < 0 {
		return nil
	}
	var out []string
	for {
		m := sectionJoinRe.FindStringSubmatchIndex(rest[pos:])
		if m == nil {
			break
		}
		w := word
		if m[2] >= 0 {
			w = rest[pos+m[2] : pos+m[3]]
			if notCallsignWords[strings.ToLower(w)] {
				break
			}
			w = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
		digits := strings.NewReplacer(" ", "", "-", "").Replace(rest[pos+m[4] : pos+m[5]])
		if cs := w + " " + digits; cs != lead {
			out = append(out, cs)
		}
		pos += m[1]
	}
	return out
}

// joinSection makes the wingmen part of the lead's stack entry. A wingman who
// had checked in on his own gives up his slot.
func joinSection(stack *state.MarshalStack, lead string, wingmen []string) {
	for _, w := range wingmen {
		if _, ok := stack.GetAircraft(w); ok {
			stack.Remove(w)
		}
		marshalAliases.mu.Lock()
		marshalAliases.m[w] = lead
		marshalAliases.mu.Unlock()
	}
	stack.SetWingmen(lead, wingmen)
	log.Info().Str("callsign", lead).Strs("wingmen", wingmen).Msg("Marshal: section checked in")
}

// stackTXCallsign is how Marshal addresses a stack entry: "Raider 302 flight"
// for a section, the callsign alone otherwise.
func stackTXCallsign(stack *state.MarshalStack, cs string) string {
	if ac, ok := stack.GetAircraft(cs); ok && len(ac.Wingmen) > 0 {
		return cs + " flight"
	}
	return cs
}
