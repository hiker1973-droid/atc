package main

import "testing"

func TestIsPromptEcho(t *testing.T) {
	prompt := "Command, Raider, Venom, Viper, request tasking, copy, IP Falcon, Granite, Hammer, Indigo, Javelin, " + rangeAreaWhisperWords
	// Day 2026-10-16 16:57, answered on 282 as "copy off target, Winchester, RTB".
	echo := "Command, Raider, Venom, Viper, request tasking, copy, IP Falcon, Granite, Hammer, Indigo, Javelin, permission to proceed to Alpha, Bravo, Charlie, Delta, Echo, Foxtrot, Golf, Hotel, India, Juliet, Kilo, Lima, Mike, November, Oscar, off target, Overlord Actual, Irene, Winchester, Bingo, Joker, RTB."
	if !isPromptEcho(echo, prompt) {
		t.Error("Command's own prompt was not caught")
	}
	for _, real := range []string{
		"Command, Raider 302, copy, off target Bravo, Winchester, Bingo, RTB.",
		"Command, Raider 302, permission to proceed to Alpha.",
		"Command, Raiders 302, fencing in air-to-ground for target Alpha, state 10.7.",
		"Command, Raider 302, request tasking.",
	} {
		if isPromptEcho(real, prompt) {
			t.Errorf("%q dropped as a prompt echo", real)
		}
	}
}
