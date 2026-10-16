package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vsfg7/atc/pkg/controller"
)

// Comms report, read by the launcher dashboard (operator 2026-10-15: "Should
// we have a tower or command report near miss or inproper comms?" -> build
// it). The towers file every near miss, departure or landing without a
// clearance, call with no callsign, callsign that changed mid-sortie and call
// not understood as one JSON line in <SkyeyeATC>\comms_report\YYYY-MM-DD.jsonl,
// the same way as the ElevenLabs usage log (eleven_usage.go).

type commsReportLine struct {
	Time     time.Time `json:"t"`
	Airfield string    `json:"airfield"`
	Tower    string    `json:"tower"`
	controller.CommsEvent
}

// commsReportDir is comms_report beside atc.exe; "" (tests) records nothing.
var commsReportDir = func() string {
	exe, err := os.Executable()
	if err != nil || strings.Contains(strings.ToLower(filepath.Base(exe)), ".test") {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "comms_report")
}()

func recordCommsEvent(tower string, ev controller.CommsEvent) {
	if commsReportDir == "" {
		return
	}
	l := commsReportLine{Time: time.Now().UTC(), Airfield: flagAirfield, Tower: tower, CommsEvent: ev}
	line, _ := json.Marshal(l)
	if err := os.MkdirAll(commsReportDir, 0o755); err != nil {
		log.Debug().Err(err).Msg("comms report: no directory")
		return
	}
	f, err := os.OpenFile(filepath.Join(commsReportDir, l.Time.Format("2006-01-02")+".jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Debug().Err(err).Msg("comms report: open failed")
		return
	}
	defer f.Close()
	f.Write(append(line, '\n'))
}
