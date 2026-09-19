package controller

import "strings"

// fieldSpokenAliases are names pilots use for a field that can't be derived
// from its tower callsign. They are added to towerKeywordAliases' list, so
// "King Hussein Tower" and "H-4 Tower" also match the "[field] tower" check.
//   - Mafraq (OJMF): the role, dashboard and preset card all say King Hussein.
//   - H4 (OJHR): Whisper writes it "H-4" / "H 4".
//   - Bassel Al-Assad (OSLK): the longest word, "al-assad", becomes the
//     primary key, so the first word needs listing; DCS names the field Latakia.
//   - Beirut (OLBA): the airport's own name is Rafic Hariri.
var fieldSpokenAliases = map[string][]string{
	"mafraq tower":          {"king hussein", "hussein"},
	"h4 tower":              {"h 4", "h-4", "h four"},
	"bassel al-assad tower": {"bassel", "basel", "basil", "bassil", "latakia", "lattakia"},
	"beirut tower":          {"beyrouth", "hariri", "rafic hariri"},
}

func extraFieldAliases(towerCallsign string) []string {
	return fieldSpokenAliases[strings.ToLower(towerCallsign)]
}

// addressedByFieldName reports whether a transmission opens with this field's
// name, with or without "Tower", allowing for Whisper misspellings:
//
//	"Akrotiri, Raider 331, ready to taxi"      — pilot dropped "Tower"
//	"Acrotari Tower, Raider 331, radar check"  — Whisper misspelt the field
//
// Both went unanswered on Foothold 2026-09-13. Only the opening words count,
// so "...inbound from Akrotiri" is not an address. Misspellings are only
// forgiven against primaryKey; aliases must match exactly, because the PG
// alias lists already hold loose Whisper guesses ("altitude", "helmet").
func addressedByFieldName(lower, primaryKey string, aliases []string) bool {
	words := strings.Fields(nonWordRe.ReplaceAllString(lower, " "))
	if len(words) == 0 {
		return false
	}
	word := func(i int) string {
		if i < len(words) {
			return words[i]
		}
		return ""
	}
	for _, key := range append([]string{primaryKey}, aliases...) {
		key = strings.Join(strings.Fields(nonWordRe.ReplaceAllString(key, " ")), " ")
		switch strings.Count(key, " ") {
		case 0:
			// "Gazi Antep" / "H 4": Whisper sometimes splits a one-word name.
			if word(0) == key || word(0)+word(1) == key {
				return true
			}
		case 1:
			if word(0)+" "+word(1) == key {
				return true
			}
		}
	}

	budget := misspellBudget(len(primaryKey))
	if budget == 0 {
		return false
	}
	if next := word(1); next == "tower" || next == "traffic" {
		budget++ // the address word confirms what the first word is meant to be
	}
	return levenshtein(word(0), primaryKey) <= budget
}

// misspellBudget is how many single-letter edits a field name of length n may
// take and still count as that field. Short names get none: "H4" vs "H5" is a
// different field, not a typo.
func misspellBudget(n int) int {
	switch {
	case n < 5:
		return 0
	case n < 7:
		return 1
	default:
		return 2
	}
}

// levenshtein is the edit distance between two ASCII strings.
func levenshtein(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
