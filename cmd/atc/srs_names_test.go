package main

import "testing"

// Day 2026-10-19: one SRS client, "Venom 203 | Yoda", heard as Venom 023.
func TestCorrectCallsignText(t *testing.T) {
	for _, c := range []struct {
		name, in, want string
		ok             bool
	}{
		{"Venom 203 | Yoda", "Tower, Venom 023 on final runway 09.", "Tower, Venom 203 on final runway 09.", true},
		{"Venom 203 | Yoda", "Tower, Venom 023, 7 mile final.", "Tower, Venom 203, 7 mile final.", true},
		{"Raider 302| Jedi", "Texaco 3-1, Raider 3-0-3, comm check.", "Texaco 3-1, Raider 302, comm check.", true},
		{"Raider 302| Jedi", "Command, Raider 320 checking in.", "Command, Raider 302 checking in.", true},
		// Already right, a different word, a number too far off: untouched.
		{"Venom 203 | Yoda", "Tower, Venom 203 inbound.", "Tower, Venom 203 inbound.", false},
		{"Venom 203 | Yoda", "Tower, Raider 023 inbound.", "Tower, Raider 023 inbound.", false},
		{"Venom 203 | Yoda", "Tower, Venom 451 inbound.", "Tower, Venom 451 inbound.", false},
		// Not a callsign name, or no number in the call.
		{"GCI Darkstar [BOT]", "Tower, Venom 023 inbound.", "Tower, Venom 023 inbound.", false},
		{"Yoda", "Tower, Venom 023 inbound.", "Tower, Venom 023 inbound.", false},
		{"Venom 203 | Yoda", "Tower, Venom flight airborne.", "Tower, Venom flight airborne.", false},
		// Runway and distance numbers are not callsigns.
		{"Venom 203 | Yoda", "Tower, runway 27, 5 miles.", "Tower, runway 27, 5 miles.", false},
	} {
		got, ok := correctCallsignText(c.in, c.name)
		if got != c.want || ok != c.ok {
			t.Errorf("%q with %q:\n got %q %v\nwant %q %v", c.in, c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestSRSNameBook(t *testing.T) {
	b := &srsNameBook{names: map[string]string{}}
	b.note([]byte(`{"Clients":[{"ClientGuid":"3Cq8EGoSgE-AW6xYuBI4ng","Name":"Venom 203 | Yoda"},{"ClientGuid":"x","Name":""}],"MsgType":2}`))
	b.note([]byte(`{"Client":{"ClientGuid":"lBXheDkvPEOpbMXKBNiRDQ","Name":"Raider 302| Jedi"},"MsgType":0}`))
	b.note([]byte(`not json`))
	if got := b.name("3Cq8EGoSgE-AW6xYuBI4ng"); got != "Venom 203 | Yoda" {
		t.Errorf("sync client: %q", got)
	}
	if got := b.name("lBXheDkvPEOpbMXKBNiRDQ"); got != "Raider 302| Jedi" {
		t.Errorf("update client: %q", got)
	}
	if got := b.name("x"); got != "" {
		t.Errorf("blank name kept: %q", got)
	}
}
