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
	accentAmerican   = "American"
	accentGeorgian   = "Georgian"
	accentRussian    = "Russian"
	accentGerman     = "German"
	accentTurkish    = "Turkish"
	accentArabic     = "Arabic"
	accentIsraeli    = "Israeli"
	accentGreek      = "Greek"
	accentNorwegian  = "Norwegian"
	accentFinnish    = "Finnish"
)

// Country accents, operator 2026-11-15 ("do the country accents for ours"):
// a field speaks English with the accent of the country it sits in. This
// replaces the 2026-10-14 British / Australian alternation for every field
// with a country row below; fields with no row (and Senaki, whose Lisa /
// senaki_5 shifts are the operator's pick) keep the old hash.
var countryAccentByICAO = map[string]string{
	// Exact fields first: the US bases and the RAF sovereign base inside the
	// countries whose prefix would otherwise claim them.
	"LTAG": accentAmerican, // Incirlik
	"LCRA": accentBritish,  // Akrotiri, RAF
	"ETAR": accentAmerican, // Ramstein
	"ETAD": accentAmerican, // Spangdahlem
	"UG5X": "",             // Senaki: Lisa / senaki_5, left as cast
	"XRMF": accentRussian,  // Mozdok
}

// countryAccentByPrefix is the ICAO region prefix's country.
var countryAccentByPrefix = map[string]string{
	"UG": accentGeorgian,
	"ED": accentGerman,
	"EN": accentNorwegian,
	"EF": accentFinnish,
	"LT": accentTurkish,
	"LL": accentIsraeli,
	"LC": accentGreek,
	"OR": accentArabic, "OJ": accentArabic, "OL": accentArabic, "OS": accentArabic, "OM": accentArabic,
}

// maleVoices are the OpenAI TTS voices that read as a man; the rest
// (nova, shimmer, coral, sage, alloy, marin) read as a woman.
var maleVoices = map[string]bool{"onyx": true, "ash": true, "echo": true, "fable": true, "ballad": true, "verse": true, "cedar": true}

func voiceIsMale(voice string) bool { return maleVoices[strings.ToLower(voice)] }

// fieldAccent is the field's fixed accent.
func fieldAccent(icao string) string {
	icao = strings.ToUpper(strings.TrimSpace(icao))
	if a, ok := countryAccentByICAO[icao]; ok {
		if a != "" {
			return a
		}
		return hashAccent(icao)
	}
	if len(icao) >= 2 {
		if a, ok := countryAccentByPrefix[icao[:2]]; ok {
			return a
		}
	}
	return hashAccent(icao)
}

// hashAccent is the 2026-10-14 cast: British or Australian, fixed per field.
func hashAccent(icao string) string {
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
	switch accent {
	case accentAustralian:
		return who + " with a clear, natural Australian accent"
	case accentAmerican:
		return who + " with a clear General American accent"
	case accentGeorgian, accentRussian, accentGerman, accentTurkish, accentArabic,
		accentIsraeli, accentGreek, accentNorwegian, accentFinnish:
		// A native speaker of that country reading English: clearly accented
		// but easy to understand on a noisy radio.
		return who + " with a clear, natural " + accent + " accent, a native of " + accentCountry[accent] + " speaking English, easy to understand"
	}
	return who + " with a crisp southern British (Received Pronunciation) accent"
}

var accentCountry = map[string]string{
	accentGeorgian: "Georgia", accentRussian: "Russia", accentGerman: "Germany", accentTurkish: "Turkey",
	accentArabic: "the Middle East", accentIsraeli: "Israel", accentGreek: "Greece",
	accentNorwegian: "Norway", accentFinnish: "Finland",
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
	if v, ok := countryVoices[accentGender{accent, male}]; ok {
		return elTower(v.id, v.name)
	}
	switch {
	case accent == accentNorwegian:
		// Operator 2026-11-19: Kola has no Lisa; Norwegian fields speak in Terry.
		return elTower(countryVoices[accentGender{accentNorwegian, true}].id, "Terry")
	case male:
		return elTower(elGeorge, "George")
	case accent == accentAustralian:
		return elTower(elLisa, "Lisa")
	}
	return elTower(elAlice, "Alice")
}

// countryVoices are ElevenLabs library voices that speak English with the
// country's accent (picked from the shared-voice library by accent, 2026-11-15;
// not yet auditioned over SRS). A country with no voice of that gender
// (Georgian, Israeli and Norwegian women) falls to the British / Australian
// voice, and an owner withdrawing a voice falls to OpenAI like any other role.
type accentGender struct {
	accent string
	male   bool
}
type countryVoice struct{ id, name string }

var countryVoices = map[accentGender]countryVoice{
	{accentAmerican, true}:  {elEric, "Eric"},
	{accentAmerican, false}: {elDeparture11, "Jodi"},
	{accentRussian, true}:   {"IGRoDkfYV2Frq7Ly0PRu", "Maxim"},
	{accentRussian, false}:  {"UV6yBNpEWyJBMHdNQx6U", "Anna"},
	{accentGerman, true}:    {"77oOXh74EdB2OgGQFHcG", "Christoph"},
	{accentGerman, false}:   {"Z0r0nsdjMqaOiXkRUHRX", "Susanne"},
	{accentTurkish, true}:   {"5L0OEG0zknJVXGx75T6N", "Atlas"},
	{accentTurkish, false}:  {"4Eq5uDjZLBd4UhQpKwuP", "Artemis"},
	{accentArabic, true}:    {"7UsSKSrq1KQIERpStCBd", "Rayan"},
	{accentArabic, false}:   {"YykvWD3tb84K50ewNzsi", "Salma"},
	{accentGreek, true}:     {"cir0mbQgrFs79o0HZw73", "Mike"},
	{accentGreek, false}:    {"1gkXJMvrzBWAwt0XqBaa", "Eleni"},
	{accentFinnish, true}:   {"72W7QSsY6RzGSUhdueKs", "Bjorn"},
	{accentFinnish, false}:  {"WtzwR2kNGVNuaTGaYLfK", "Liis"},
	{accentGeorgian, true}:  {"2oMnzJn2otqHVjyCboYJ", "Giorgi"},
	{accentIsraeli, true}:   {"8sZ9Z3Jy28CGLLCmKEHn", "Viktor"},
	{accentNorwegian, true}: {"s6U9BTPmhNGeAfGdcn5K", "Terry"},
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
