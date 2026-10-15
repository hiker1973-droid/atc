package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// ElevenLabs usage log, read by the launcher dashboard (operator 2026-10-15:
// "put 2 in launcher dashboard" - a monthly credit count with a warning before
// the plan runs out, since running out silently drops every role to OpenAI).
//
// Every successful ElevenLabs call appends one JSON line to
// <SkyeyeATC>\elevenlabs_usage\YYYY-MM-DD.jsonl. All roles on a rig share the
// file; each line is a single O_APPEND write, which Windows keeps whole.
// Credits are the "character-cost" header ElevenLabs returns with the audio
// (Flash costs about half); without it, the text length.

type elevenUsage struct {
	Time     time.Time `json:"t"`
	Voice    string    `json:"voice"`    // elevenVoice.Name: Command, Hitman, Lisa, Departure ...
	Airfield string    `json:"airfield"` // the process's --airfield (towers); OMDM default elsewhere
	Model    string    `json:"model"`
	Chars    int       `json:"chars"`
	Credits  int       `json:"credits"`
}

// elevenUsageDir is where the usage files go: elevenlabs_usage beside atc.exe.
// "" (set by tests) records nothing.
var elevenUsageDir = func() string {
	exe, err := os.Executable()
	if err != nil || strings.Contains(strings.ToLower(filepath.Base(exe)), ".test") {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "elevenlabs_usage")
}()

func recordElevenUsage(ev elevenVoice, text, costHeader string) {
	if elevenUsageDir == "" {
		return
	}
	credits, err := strconv.Atoi(strings.TrimSpace(costHeader))
	if err != nil {
		credits = len(text)
	}
	u := elevenUsage{Time: time.Now().UTC(), Voice: ev.Name, Airfield: flagAirfield, Model: ev.model(),
		Chars: len(text), Credits: credits}
	line, _ := json.Marshal(u)
	if err := os.MkdirAll(elevenUsageDir, 0o755); err != nil {
		log.Debug().Err(err).Msg("ElevenLabs usage log: no directory")
		return
	}
	path := filepath.Join(elevenUsageDir, u.Time.Format("2006-01-02")+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Debug().Err(err).Msg("ElevenLabs usage log: open failed")
		return
	}
	defer f.Close()
	f.Write(append(line, '\n'))
}
