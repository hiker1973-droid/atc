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
	Sector  string // range_north sector; with two JTACs each runs one (--jtac-sector, --jtac2-sector)
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
// Training kneeboard (V1.25, Hornet 081-085 / Viper 091-095). The IP names are
// what the pilot reads back, so they must stay in step with the kneeboard.
// V1.25 (operator 2026-10-15): the range moved past the Main Caucasus ridge,
// 100-127 nm north of Senaki, in two sectors (Kuban Valley 1-3, Cherkessk 4-5).
// GENERATED from range_north.SITES / build_ag.nine_line() (Saved Games,
// Missions/Caucasus/Training/AG Range/_BUILD/range_north.py), positions
// converted by DCS's terrain module. Regenerate, do not hand-edit.
var caucasusTaskingTargets = []taskingTarget{
	{IP: "Falcon", Number: 1, Name: "Valley Road Bridge", Control: "type 2", Sector: "Kuban Valley",
		IPPos: orb.Point{41.898726, 43.753326}, TgtPos: orb.Point{41.887660, 43.897790}, MagVar: 7.64,
		RunIn: 349, FAHLow: 329, FAHHigh: 9, ElevFt: 2615, Egress: "angels 15"},
	{IP: "Granite", Number: 2, Name: "Artillery Park", Control: "type 1", Sector: "Kuban Valley",
		IPPos: orb.Point{41.757882, 44.061294}, TgtPos: orb.Point{41.891173, 43.951915}, MagVar: 7.66,
		RunIn: 131, FAHLow: 111, FAHHigh: 151, ElevFt: 2375, Egress: "angels 14"},
	{IP: "Hammer", Number: 3, Name: "Kuban Rail Bridge", Control: "type 1", Sector: "Kuban Valley",
		IPPos: orb.Point{42.181234, 44.054785}, TgtPos: orb.Point{41.975988, 44.073047}, MagVar: 7.70,
		RunIn: 269, FAHLow: 249, FAHHigh: 289, ElevFt: 2080, Egress: "angels 13"},
	{IP: "Indigo", Number: 4, Name: "Fuel Depot", Control: "type 2", Sector: "Cherkessk",
		IPPos: orb.Point{42.277286, 44.109959}, TgtPos: orb.Point{42.114918, 44.190645}, MagVar: 7.75,
		RunIn: 297, FAHLow: 277, FAHHigh: 317, ElevFt: 2110, Egress: "angels 14"},
	{IP: "Javelin", Number: 5, Name: "Command Post", Control: "type 2", Sector: "Cherkessk",
		IPPos: orb.Point{42.020807, 44.472706}, TgtPos: orb.Point{41.980892, 44.336361}, MagVar: 7.79,
		RunIn: 184, FAHLow: 164, FAHHigh: 204, ElevFt: 1510, Egress: "angels 13"},
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
	effectsAsked time.Time // the BDA could not assess and asked the pilot for theirs
}

// taskingBoard tracks which flight holds which target.
type taskingBoard struct {
	mu      sync.Mutex
	targets []taskingTarget
	flights map[string]*taskingAssignment
	now     func() time.Time
	intn    func(int) int

	// jtacs, when set, are the JTACs Command hands the flight to. Command then
	// only sends the flight to the JTAC; the JTAC passes the 9-line and clears
	// it hot (jtac.go). Unset, Command clears hot on the readback itself.
	jtacs    []*jtacNet
	handoffs map[string]jtacHandoff // flights Command sent to a JTAC, for the readback
}

// jtacNet is the JTAC as Command names it on the radio.
type jtacNet struct {
	Spoken  string  // "Hitman one one"
	FreqMHz float64 // 61.000
	// board is the JTAC's own board, holding its sector's targets, when there
	// is more than one JTAC (operator 2026-10-16: Hitman 2-1 on 62.000, split
	// by sector). nil = the JTAC works Command's board, the whole range.
	board *taskingBoard
}

// workBoard is the board the JTAC works: its own, or Command's.
func (n *jtacNet) workBoard(command *taskingBoard) *taskingBoard {
	if n.board != nil {
		return n.board
	}
	return command
}

type jtacHandoff struct {
	at  time.Time
	net *jtacNet
}

func newTaskingBoard(targets []taskingTarget) *taskingBoard {
	return &taskingBoard{targets: targets, flights: make(map[string]*taskingAssignment), handoffs: make(map[string]jtacHandoff), now: time.Now, intn: rand.Intn}
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
		// The answer to a good BDA's "report ready for further tasking".
		"ready for further tasking", "request further tasking", "ready for more tasking",
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
		if len(b.jtacs) > 0 {
			n := b.chooseJTAC(key, now)
			if jb := n.workBoard(b); jb == b {
				if cur != nil {
					cur.redraw = true
				}
			} else {
				jb.markRedraw(key)
			}
			// Moving sectors frees the target the flight held on the other net.
			for _, o := range b.jtacs {
				if o != n && o.board != nil {
					o.board.release(key)
				}
			}
			b.handoffs[key] = jtacHandoff{at: now, net: n}
			return fmt.Sprintf("%s, %s, contact %s on %s for tasking.", callsign, channelName,
				n.Spoken, formatFreqMHz(n.FreqMHz))
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

	if len(b.jtacs) > 0 {
		if h, ok := b.handoffs[key]; ok && now.Sub(h.at) <= taskingAckWindow && isTaskingAck(lower) {
			delete(b.handoffs, key)
			return fmt.Sprintf("%s, %s, switch %s, good hunting.", callsign, channelName, h.net.Spoken)
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
	free, last, held := b.free(key, now)
	switch {
	case len(free) > 0:
		return free[b.intn(len(free))]
	case last >= 0 && !held[last]:
		return last
	}
	return b.intn(len(b.targets))
}

// free lists the targets no other flight holds, less the one this flight has
// just worked (last, -1 for none). Caller holds b.mu.
func (b *taskingBoard) free(key string, now time.Time) (free []int, last int, held map[int]bool) {
	held = make(map[int]bool)
	for k, a := range b.flights {
		if k != key && now.Sub(a.issued) < taskingHoldTime {
			held[a.target] = true
		}
	}
	last = -1
	if a := b.flights[key]; a != nil {
		last = a.target
	}
	for i := range b.targets {
		if !held[i] && i != last {
			free = append(free, i)
		}
	}
	return free, last, held
}

// chooseJTAC picks the JTAC Command sends a flight to. With one sector per
// JTAC: one with a free target the flight has not just been working, so a
// flight coming back for more changes sector; then any with a free target;
// then any. Caller holds b.mu; takes each JTAC board's lock in turn.
func (b *taskingBoard) chooseJTAC(key string, now time.Time) *jtacNet {
	if len(b.jtacs) == 1 {
		return b.jtacs[0]
	}
	var fresh, open []*jtacNet
	for _, n := range b.jtacs {
		jb := n.workBoard(b)
		if jb != b {
			jb.mu.Lock()
		}
		free, _, _ := jb.free(key, now)
		_, worked := jb.flights[key]
		if jb != b {
			jb.mu.Unlock()
		}
		if len(free) > 0 {
			open = append(open, n)
			if !worked {
				fresh = append(fresh, n)
			}
		}
	}
	for _, c := range [][]*jtacNet{fresh, open, b.jtacs} {
		if len(c) > 0 {
			return c[b.intn(len(c))]
		}
	}
	return b.jtacs[0]
}

// handOffTarget sends a flight that named a 9-line area to the JTAC running
// one of those targets, with that target already drawn so its check-in briefs
// it (command_areas.go). nil when no JTAC runs any of them.
func (b *taskingBoard) handOffTarget(key string, numbers []int) *jtacNet {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	for _, n := range b.jtacs {
		jb := n.workBoard(b)
		if jb != b {
			jb.mu.Lock()
		}
		// One the other flights do not hold, when the area has a choice.
		pick := -1
		_, _, held := jb.free(key, now)
		for i, t := range jb.targets {
			for _, num := range numbers {
				if t.Number == num && (pick < 0 || held[pick] && !held[i]) {
					pick = i
				}
			}
		}
		if pick >= 0 {
			jb.flights[key] = &taskingAssignment{target: pick, issued: now}
		}
		if jb != b {
			jb.mu.Unlock()
		}
		if pick < 0 {
			continue
		}
		for _, o := range b.jtacs {
			if o != n && o.board != nil {
				o.board.release(key)
			}
		}
		b.handoffs[key] = jtacHandoff{at: now, net: n}
		return n
	}
	return nil
}

// markRedraw: the flight asked Command for more, so its next check-in on this
// board draws a new target.
func (b *taskingBoard) markRedraw(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if cur := b.flights[key]; cur != nil {
		cur.redraw = true
	}
}

// release drops the flight from this board, freeing its target.
func (b *taskingBoard) release(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.flights, key)
}

// sectorTargets is the part of the range one JTAC runs.
func sectorTargets(all []taskingTarget, sector string) []taskingTarget {
	var out []taskingTarget
	for _, t := range all {
		if strings.EqualFold(t.Sector, sector) {
			out = append(out, t)
		}
	}
	return out
}
