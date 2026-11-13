package main

import "testing"

func TestSplitNineLine(t *testing.T) {
	pre, slow, post := splitNineLine("Raider 1, target 3, armour. line 1, IP Alpha. line 2, heading zero nine zero. line 3, distance two zero miles. Read back.")
	if pre != "Raider 1, target 3, armour." || slow == "" || post != "" || slow[:6] != "line 1" {
		t.Fatalf("bad split: %q | %q | %q", pre, slow, post)
	}
	if _, s, _ := splitNineLine("copy, cleared hot."); s != "" {
		t.Fatalf("no 9-line expected, got %q", s)
	}
	base := voiceProfile{speed: 1.2, eleven: elevenVoice{ID: "x", Speed: 1.08}, lineSlow: 0.88}
	v := slowedVoice(base, 0.88)
	if v.speed >= base.speed || v.eleven.Speed >= 1.08 || v.lineSlow != 0 {
		t.Fatalf("slowedVoice wrong: %+v", v)
	}
}
