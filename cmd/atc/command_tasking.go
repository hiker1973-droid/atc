package main

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/paulmach/orb"
)

// Command tasking: a pilot asks for a target, Command sends them to the IP on
// that target's 9-line kneeboard page, and clears them hot on the readback.
// Unlike the other Command intents this is a two-step exchange, so it keeps
// state per callsign. Kept apart from commandResponse, which is pure text.

// taskingTarget is one 9-line page: the IP the flight holds at, the target it
// runs in on, and the geometry the JTAC checks the run against.
type taskingTarget struct {
	IP      string
	Number  int
	Name    string
	Control string // "type 1" / "type 2", as the card prints it
	IPPos   orb.Point // lon, lat
	TgtPos  orb.Point // lon, lat
	MagVar  float64   // east, degrees, at the target
	RunIn   int       // line 2, magnetic
	FAHLow  int       // final attack heading window, magnetic, clockwise low to high
	FAHHigh int
	ElevFt  int    // line 4
	Egress  string // line 9 altitude, e.g. "angels 15"
}

// caucasusTaskingTargets mirrors the 9-line pages of the vSFG-7 Caucasus
// Training kneeboard (V1.22, pages 19-23). The IP names are what the pilot
// reads back, so they must stay in step with the kneeboard. Positions, run-in,
// final attack headings, elevation and egress are build_ag.SITES /
// nine_line() in the AG Range builder (Saved Games, Missions/Caucasus/
// Training/AG Range/_BUILD/build_ag.py), which draws the cards.
var caucasusTaskingTargets = []taskingTarget{
	{IP: "Anvil", Number: 1, Name: "Enguri Bridge", Control: "type 2",
		IPPos: orb.Point{41.942121, 42.431496}, TgtPos: orb.Point{41.851168, 42.582912}, MagVar: 7.21,
		RunIn: 329, FAHLow: 309, FAHHigh: 349, ElevFt: 445, Egress: "angels 15"},
	{IP: "Brick", Number: 2, Name: "Rioni Rail Bridge", Control: "type 1",
		IPPos: orb.Point{42.314535, 42.284035}, TgtPos: orb.Point{42.328935, 42.118794}, MagVar: 7.08,
		RunIn: 169, FAHLow: 149, FAHHigh: 189, ElevFt: 135, Egress: "angels 12"},
	{IP: "Cobra", Number: 3, Name: "Fuel Depot", Control: "type 2",
		IPPos: orb.Point{42.020965, 42.318974}, TgtPos: orb.Point{41.806303, 42.364730}, MagVar: 7.14,
		RunIn: 279, FAHLow: 259, FAHHigh: 299, ElevFt: 25, Egress: "angels 14"},
	{IP: "Dagger", Number: 4, Name: "Command Post", Control: "type 2",
		IPPos: orb.Point{41.939278, 42.130233}, TgtPos: orb.Point{42.155376, 42.170030}, MagVar: 7.09,
		RunIn: 69, FAHLow: 49, FAHHigh: 89, ElevFt: 50, Egress: "angels 13"},
	{IP: "Eagle", Number: 5, Name: "Artillery Park", Control: "type 1",
		IPPos: orb.Point{41.889289, 42.172673}, TgtPos: orb.Point{41.729363, 42.057419}, MagVar: 7.05,
		RunIn: 219, FAHLow: 199, FAHHigh: 239, ElevFt: 15, Egress: "angels 12"},
}

// taskingTargetsForMap returns the tasking set for a theatre, nil where the
// mission has no 9-line targets.
func taskingTargetsForMap(mapName string) []taskingTarget {
	switch strings.ToLower(strings.TrimSpace(mapName)) {
	case "ca", "caucasus", "blacksea", "black-sea", "black sea":
		return caucasusTaskingTargets
	}
	return nil
}

const (
	// taskingAckWindow is how long a bare "copy" still counts as the readback
	// of a tasking. Past it the pilot has to ask again.
	taskingAckWindow = 5 * time.Minute
	// taskingHoldTime is how long a target stays off the board for other
	// flights once it has been handed out.
	taskingHoldTime = 45 * time.Minute
)

type taskingAssignment struct {
	target   int // index into the board's targets
	issued   time.Time
	awaiting bool // tasking passed, readback not yet heard

	// JTAC side (jtac.go). Only used when the board has a JTAC.
	stage     jtacStage
	redraw    bool      // asked Command for more: draw a new target at check-in
	clearedAt time.Time // when "cleared hot" went out; the watch runs from here
	offAxis   int       // consecutive watch samples outside the attack window
	bdaDone   bool      // this pass's BDA has gone out
	offAcked  bool      // this pass's "off" has been answered
}

// taskingBoard tracks which flight holds which target.
type taskingBoard struct {
	mu      sync.Mutex
	targets []taskingTarget
	flights map[string]*taskingAssignment
	now     func() time.Time
	intn    func(int) int

	// jtac, when set, is the JTAC Command hands the flight to. Command then
	// only sends the flight to the IP; the JTAC passes the 9-line and clears
	// it hot (jtac.go). Unset, Command clears hot on the readback itself.
	jtac     *jtacNet
	handoffs map[string]time.Time // flights Command sent to the JTAC, for the readback
}

// jtacNet is the JTAC as Command names it on the radio.
type jtacNet struct {
	Spoken  string  // "Hitman one one"
	FreqMHz float64 // 61.000
}

func newTaskingBoard(targets []taskingTarget) *taskingBoard {
	return &taskingBoard{targets: targets, flights: make(map[string]*taskingAssignment), handoffs: make(map[string]time.Time), now: time.Now, intn: rand.Intn}
}

// commandTasking is the board for the running theatre, built on first use
// because flagMap is not parsed at package init.
var (
	commandTasking     *taskingBoard
	commandTaskingOnce sync.Once
)

func commandTaskingBoard() *taskingBoard {
	commandTaskingOnce.Do(func() {
		commandTasking = newTaskingBoard(taskingTargetsForMap(flagMap))
	})
	return commandTasking
}

func isTaskingRequest(lower string) bool {
	return containsAny(lower, "request tasking", "requesting tasking", "request task", "requesting task",
		"request a task", "ready for tasking", "request target", "requesting target", "request a target",
		"request new tasking", "request new target", "request next tasking", "request next target",
		"say tasking", "any tasking",
		// Whisper heard "request tasking" as "request taxiing" on Hitman's net
		// (DCS3 2026-10-14). Nobody taxis on Command or the JTAC net.
		"request taxiing", "requesting taxiing", "request taxing", "requesting taxing")
}

func isTaskingAck(lower string) bool {
	// "how copy" is a radio check, not a readback.
	if strings.Contains(lower, "how copy") {
		return false
	}
	return containsAny(lower, "copy", "copies", "wilco", "roger", "proceeding")
}

// Response answers a tasking request or the readback that follows one. Returns
// "" for any other call.
func (b *taskingBoard) Response(text, callsign, channelName string) string {
	lower := strings.ToLower(text)
	key := strings.ToLower(strings.TrimSpace(callsign))

	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	cur := b.flights[key]

	if isTaskingRequest(lower) {
		if len(b.targets) == 0 {
			return fmt.Sprintf("%s, %s, negative tasking at this time.", callsign, channelName)
		}
		// With a JTAC, Command only hands the flight over: the JTAC picks the
		// target and passes the 9-line, IP included (operator, 2026-10-14 -
		// the real flow). A flight coming back for more gets a fresh draw
		// at its next check-in.
		if b.jtac != nil {
			if cur != nil {
				cur.redraw = true
			}
			b.handoffs[key] = now
			return fmt.Sprintf("%s, %s, contact %s on %s for tasking.", callsign, channelName,
				b.jtac.Spoken, formatFreqMHz(b.jtac.FreqMHz))
		}
		// Asking again before the readback repeats the same tasking; asking
		// after it means the flight is done and wants the next target.
		if cur == nil || !cur.awaiting {
			cur = &taskingAssignment{target: b.pick(key, now)}
			b.flights[key] = cur
		}
		cur.issued, cur.awaiting = now, true
		t := b.targets[cur.target]
		return fmt.Sprintf("%s, %s, proceed to IP %s, target %d, %s.", callsign, channelName, t.IP, t.Number, t.Name)
	}

	if b.jtac != nil {
		if at, ok := b.handoffs[key]; ok && now.Sub(at) <= taskingAckWindow && isTaskingAck(lower) {
			delete(b.handoffs, key)
			return fmt.Sprintf("%s, %s, switch %s, good hunting.", callsign, channelName, b.jtac.Spoken)
		}
		return ""
	}
	if cur == nil || !cur.awaiting || now.Sub(cur.issued) > taskingAckWindow {
		return ""
	}
	if isTaskingAck(lower) || strings.Contains(lower, strings.ToLower(b.targets[cur.target].IP)) {
		cur.awaiting = false
		return fmt.Sprintf("%s, %s, you are cleared hot.", callsign, channelName)
	}
	return ""
}

// pick draws a target at random, passing over any another flight still holds
// and the one this flight has just worked. With nothing else free it falls
// back to the flight's own last target, then to any target at all.
func (b *taskingBoard) pick(key string, now time.Time) int {
	held := make(map[int]bool)
	for k, a := range b.flights {
		if k != key && now.Sub(a.issued) < taskingHoldTime {
			held[a.target] = true
		}
	}
	last := -1
	if a := b.flights[key]; a != nil {
		last = a.target
	}
	var free []int
	for i := range b.targets {
		if !held[i] && i != last {
			free = append(free, i)
		}
	}
	switch {
	case len(free) > 0:
		return free[b.intn(len(free))]
	case last >= 0 && !held[last]:
		return last
	}
	return b.intn(len(b.targets))
}
