package main

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// Command tasking: a pilot asks for a target, Command sends them to the IP on
// that target's 9-line kneeboard page, and clears them hot on the readback.
// Unlike the other Command intents this is a two-step exchange, so it keeps
// state per callsign. Kept apart from commandResponse, which is pure text.

// taskingTarget is one 9-line page: the IP the flight holds at and the target
// it runs in on.
type taskingTarget struct {
	IP     string
	Number int
	Name   string
}

// caucasusTaskingTargets mirrors the 9-line pages of the vSFG-7 Caucasus
// Training kneeboard (V1.22, pages 19-23). The IP names are what the pilot
// reads back, so they must stay in step with the kneeboard.
var caucasusTaskingTargets = []taskingTarget{
	{"Anvil", 1, "Enguri Bridge"},
	{"Brick", 2, "Rioni Rail Bridge"},
	{"Cobra", 3, "Fuel Depot"},
	{"Dagger", 4, "Command Post"},
	{"Eagle", 5, "Artillery Park"},
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
}

// taskingBoard tracks which flight holds which target.
type taskingBoard struct {
	mu      sync.Mutex
	targets []taskingTarget
	flights map[string]*taskingAssignment
	now     func() time.Time
	intn    func(int) int
}

func newTaskingBoard(targets []taskingTarget) *taskingBoard {
	return &taskingBoard{targets: targets, flights: make(map[string]*taskingAssignment), now: time.Now, intn: rand.Intn}
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
		"say tasking", "any tasking")
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
