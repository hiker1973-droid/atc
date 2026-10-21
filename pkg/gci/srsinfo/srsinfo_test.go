package srsinfo

import "testing"

func TestBook(t *testing.T) {
	b := NewBook("Darkstar", 264e6)
	b.Note([]byte(`{"MsgType":2,"Clients":[
		{"ClientGuid":"a","Name":"Venom 212 | Dabs","RadioInfo":{"radios":[{"freq":251e6,"modulation":0},{"freq":264e6,"modulation":0}]}},
		{"ClientGuid":"b","Name":"Raider 302| Jedi","RadioInfo":{"radios":[{"freq":282e6,"modulation":0}]}},
		{"ClientGuid":"c","Name":"Senaki Tower [BOT]","RadioInfo":{"radios":[{"freq":264e6,"modulation":0}]}},
		{"ClientGuid":"d","Name":"Darkstar","RadioInfo":{"radios":[{"freq":264e6,"modulation":0}]}},
		{"ClientGuid":"e","Name":"Knight 11","RadioInfo":{"radios":[{"freq":264e6,"modulation":1}]}}]}`))
	if !b.IsOnFrequency("Venom 212 | Dabs") || b.IsOnFrequency("Raider 302| Jedi") {
		t.Error("on-frequency by name wrong")
	}
	if got := b.HumansOnFrequency(); got != 1 {
		t.Errorf("humans = %d, want 1 (Dabs; not the bot, not our own Darkstar, not 264 FM)", got)
	}
	if got := b.ClientsOnFrequency(); got != 2 {
		t.Errorf("clients = %d, want 2 (Dabs + the bot)", got)
	}
	// An update retunes Raider to 264; a disconnect drops Dabs.
	b.Note([]byte(`{"MsgType":0,"Client":{"ClientGuid":"b","RadioInfo":{"radios":[{"freq":264.0004e6,"modulation":0}]}}}`))
	b.Note([]byte(`{"MsgType":5,"Client":{"ClientGuid":"a"}}`))
	if !b.IsOnFrequency("Raider 302| Jedi") || b.IsOnFrequency("Venom 212 | Dabs") || b.HumansOnFrequency() != 1 {
		t.Errorf("after update/disconnect: raider=%v dabs=%v humans=%d",
			b.IsOnFrequency("Raider 302| Jedi"), b.IsOnFrequency("Venom 212 | Dabs"), b.HumansOnFrequency())
	}
	if f := b.Frequencies(); len(f) != 1 || f[0].Frequency.Megahertz() != 264 {
		t.Errorf("frequencies %v", f)
	}
}
