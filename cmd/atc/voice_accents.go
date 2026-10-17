package main

import (
	"fmt"
	"hash/fnv"
	"strings"
)

// Accent casting, operator 2026-10-14: "all ATC voices to be British or
// Australian female/male voice. Keep Marshal and Deckboss. Command need to be
// stern English male voice." Ruled the same day: ATIS counts as ATC (Syria's
// local-accent English reads go British/Australian; the second-language pass
// keeps its native speaker), the accent alternates by field, and Hitman 1-1
// is an Australian man.
//
// The accent is a fixed property of the field (its ICAO), so a tower and its
// ATIS sound like the same country's service and the cast is the same on
// every restart. The gender is the voice the start script already gives the
// field. A start script that passes its own --voice-style-tower (Akrotiri's
// raf-british) or a station with the RAF British ATIS style keeps it.

const (
	accentBritish    = "British"
	accentAustralian = "Australian"
)

// maleVoices are the OpenAI TTS voices that read as a man; the rest
// (nova, shimmer, coral, sage, alloy, marin) read as a woman.
var maleVoices = map[string]bool{"onyx": true, "ash": true, "echo": true, "fable": true, "ballad": true, "verse": true, "cedar": true}

func voiceIsMale(voice string) bool { return maleVoices[strings.ToLower(voice)] }

// fieldAccent is the field's fixed accent.
func fieldAccent(icao string) string {
	h := fnv.New32a()
	h.Write([]byte(strings.ToUpper(strings.TrimSpace(icao))))
	if h.Sum32()%2 == 0 {
		return accentBritish
	}
	return accentAustralian
}

func accentPhrase(accent string, male bool) string {
	who := "a woman"
	if male {
		who = "a man"
	}
	if accent == accentAustralian {
		return who + " with a clear, natural Australian accent"
	}
	return who + " with a crisp southern British (Received Pronunciation) accent"
}

// departureSignOff is the tower's farewell on its departure handoff call, in
// the field's accent (operator 2026-10-15: "safe skys, gday"; then "lose the
// safe skys" the same day to save credits).
func departureSignOff(icao string) string {
	if flagVoiceStyleTower == styleTower && fieldAccent(icao) == accentAustralian {
		return "G'day"
	}
	return "Good day"
}

// towerStyleFor is the tower delivery for a field and voice.
func towerStyleFor(icao, voice string) string {
	if flagVoiceStyleTower != styleTower {
		return flagVoiceStyleTower
	}
	return fmt.Sprintf("Speak as a military air traffic control tower controller: %s. "+
		"Clipped, matter-of-fact, brisk. Flat affect, no warmth, no rising intonation. "+
		"Run grouped numbers together as a single phrase rather than reading them one by one.",
		accentPhrase(fieldAccent(icao), voiceIsMale(voice)))
}

// atisEnglishVoice is the voice for a station's English read: the field's
// accent, unless the station has the RAF British style or --voice-style-atis
// was set by hand.
func atisEnglishVoice(st *atisStation) voiceProfile {
	p := atisVoice(st.Voice)
	switch {
	case st.Style == styleATISBritish:
		p.instructions = st.Style
	case flagVoiceStyleATIS != styleATIS:
	default:
		p.instructions = fmt.Sprintf("Read as a recorded military aerodrome information broadcast by %s. "+
			"Even, neutral, unhurried, identical inflection on every phrase. No emphasis anywhere.",
			accentPhrase(fieldAccent(st.ICAO), voiceIsMale(st.Voice)))
	}
	return p
}

// elevenTowerVoice is the ElevenLabs voice for a field's accent and gender.
// Operator 2026-10-14: Hitman's voice is his alone ("Hitman is most important
// for voice"), and every field gets ElevenLabs (Senaki "is a primary base").
// Australian women are Lisa (library voice, picked for Senaki on the Creator
// plan), British women Alice, men George. No tower uses Hitman's voice.
func elevenTowerVoice(accent string, male bool) elevenVoice {
	switch {
	case male:
		return elTower(elGeorge, "George")
	case accent == accentAustralian:
		return elTower(elLisa, "Lisa")
	}
	return elTower(elAlice, "Alice")
}

// towerProfile is the tower's full voice: OpenAI voice + accent instructions,
// and the matching ElevenLabs voice. A start script's own tower style (Akrotiri
// raf-british) reads as British.
func towerProfile(voice string) voiceProfile {
	accent := fieldAccent(flagAirfield)
	if flagVoiceStyleTower != styleTower {
		accent = accentBritish
	}
	eleven := elevenTowerVoice(accent, voiceIsMale(voice))
	if flagElevenTowerVoice != "" {
		eleven = elTower(flagElevenTowerVoice, "Tower")
	}
	return voiceProfile{voice: voice, speed: flagTTSSpeed, instructions: towerStyleFor(flagAirfield, voice),
		eleven: eleven}
}
