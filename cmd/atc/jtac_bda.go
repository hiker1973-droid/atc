package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/paulmach/orb"
)

// BDA: Hitman's battle damage assessment after a pass, from the Tacview
// picture (tacview_ground.go) against what the AG Range builder placed at each
// target. Two parts:
//
//   - impacts: where the flight's weapons ended, against the aim point
//   - kills:   the target's objects destroyed, counted against the builder's
//              list, with the target's priority objects first
//
// The two bridges are map objects, which Tacview does not carry, so their
// damage cannot be assessed; Enguri's checkpoint and Rioni's no-strike road
// bridge (impacts only) can. With no ground objects seen at a target, the
// report says it cannot assess and gives impacts only. Operator 2026-10-14:
// "write it to handle all cases".

const (
	// bdaSettle: wait this long after the last impact so the kills register.
	bdaSettle = 15 * time.Second
	// bdaWindow: a pass's BDA is due within this long of cleared hot.
	bdaWindow = 10 * time.Minute
	// bdaImpactRadiusM: an impact further than this from the aim point is not
	// on this target.
	bdaImpactRadiusM = 2000.0
	// bdaGoodHitM: within this of the aim point is a good hit.
	bdaGoodHitM = 20.0
	// bdaMatchM: a Tacview object within this of a placed object is that object.
	bdaMatchM = 40.0
	// bdaNoStrikeM: an impact within this of the Rioni road bridge hit it.
	bdaNoStrikeM = 50.0
)

// bdaObject is one object the builder placed at a target.
type bdaObject struct {
	Label string
	Pos   orb.Point // lon, lat
}

// bdaTarget is the damage picture of one 9-line target.
type bdaTarget struct {
	Primary  string // label prefix of the objects that matter first; "" = none
	Spoken   string // how Primary is said, plural handled by bdaCount
	Bridge   bool   // the aim point is a map bridge Tacview cannot see
	Objects  []bdaObject
	NoStrike *orb.Point // an impact near it is called out
}

// rioniRoadBridge is build_ag.ROAD_BRIDGE_NO_STRIKE.
var rioniRoadBridge = orb.Point{42.332265, 42.119356}

// caucasusBDA is keyed by target number. Positions are build_ag.SITES layout
// offsets (and ENGURI_CHECKPOINT) turned into lat/lon with each site's grid
// convergence (about 6 deg here) taken from its target-to-IP line.
var caucasusBDA = map[int]bdaTarget{
	1: {Primary: "checkpoint", Spoken: "checkpoint", Bridge: true, Objects: []bdaObject{
		{"checkpoint vehicle", orb.Point{41.848118, 42.584173}},
		{"checkpoint vehicle", orb.Point{41.847791, 42.584281}},
		{"checkpoint vehicle", orb.Point{41.847459, 42.584360}},
		{"checkpoint outpost", orb.Point{41.847893, 42.584049}},
	}},
	2: {Bridge: true, NoStrike: &rioniRoadBridge},
	3: {Primary: "large tank", Spoken: "large tank", Objects: []bdaObject{
		{"large tank", orb.Point{41.805736, 42.365177}},
		{"large tank", orb.Point{41.806156, 42.365144}},
		{"large tank", orb.Point{41.806564, 42.365113}},
		{"large tank", orb.Point{41.806983, 42.365080}},
		{"small tank", orb.Point{41.805943, 42.364758}},
		{"small tank", orb.Point{41.806663, 42.364702}},
		{"fuel bladder", orb.Point{41.805959, 42.364443}},
		{"fuel bladder", orb.Point{41.806558, 42.364397}},
		{"fuel truck", orb.Point{41.805969, 42.364084}},
		{"fuel truck", orb.Point{41.806208, 42.364066}},
		{"fuel truck", orb.Point{41.806448, 42.364047}},
		{"power box", orb.Point{41.807177, 42.364483}},
		{"generator", orb.Point{41.805379, 42.364623}},
		{"workshop", orb.Point{41.806455, 42.365793}},
	}},
	4: {Primary: "command building", Spoken: "command building", Objects: []bdaObject{
		{"command building", orb.Point{42.155376, 42.170030}},
		{"radio mast", orb.Point{42.155905, 42.170346}},
		{"bunker", orb.Point{42.154960, 42.169660}},
		{"bunker", orb.Point{42.155677, 42.169603}},
		{"tent", orb.Point{42.154877, 42.170338}},
		{"tent", orb.Point{42.154638, 42.170357}},
		{"tent", orb.Point{42.154399, 42.170376}},
		{"generator", orb.Point{42.156140, 42.169880}},
		{"command vehicle", orb.Point{42.155340, 42.170615}},
		{"command vehicle", orb.Point{42.155579, 42.170596}},
		{"jeep", orb.Point{42.155280, 42.169366}},
		{"jeep", orb.Point{42.155423, 42.169355}},
		{"command vehicle", orb.Point{42.156010, 42.170651}},
		{"outpost", orb.Point{42.155222, 42.168968}},
	}},
	5: {Primary: "gun", Spoken: "gun", Objects: []bdaObject{
		{"gun", orb.Point{41.728466, 42.057486}},
		{"gun", orb.Point{41.728825, 42.057459}},
		{"gun", orb.Point{41.729184, 42.057432}},
		{"gun", orb.Point{41.729542, 42.057406}},
		{"gun", orb.Point{41.729901, 42.057379}},
		{"gun", orb.Point{41.730260, 42.057352}},
		{"rocket launcher", orb.Point{41.729063, 42.056993}},
		{"rocket launcher", orb.Point{41.729542, 42.056957}},
		{"ammunition truck", orb.Point{41.728955, 42.056643}},
		{"ammunition truck", orb.Point{41.729254, 42.056620}},
		{"ammunition truck", orb.Point{41.729553, 42.056598}},
		{"ammo dump", orb.Point{41.728841, 42.056248}},
		{"ammo dump", orb.Point{41.729558, 42.056194}},
		{"tent", orb.Point{41.730187, 42.056819}},
		{"command vehicle", orb.Point{41.728394, 42.056954}},
	}},
}

func metresBetween(a, b orb.Point) float64 { return haversineNm(a, b) * 1852 }

// bdaKills matches the picture's ground objects to the target's placed
// objects. seen = how many placed objects were found in the picture at all;
// dead[i] = placed object i is destroyed.
func bdaKills(bt bdaTarget, ground []tacviewGround) (seen int, dead []bool) {
	dead = make([]bool, len(bt.Objects))
	used := make(map[string]bool)
	for i, o := range bt.Objects {
		best, bestM := -1, bdaMatchM
		for j, g := range ground {
			if used[g.ID] {
				continue
			}
			if m := metresBetween(o.Pos, g.Pos); m <= bestM {
				best, bestM = j, m
			}
		}
		if best < 0 {
			continue
		}
		used[ground[best].ID] = true
		seen++
		dead[i] = !ground[best].Alive
	}
	return seen, dead
}

// compass8 is a true bearing as an eight-point direction.
func compass8(deg float64) string {
	names := []string{"north", "north-east", "east", "south-east", "south", "south-west", "west", "north-west"}
	return names[int(math.Mod(deg+22.5+360, 360)/45)%8]
}

func roundMetres(m float64) int {
	if m < 100 {
		return int(math.Round(m/10) * 10)
	}
	return int(math.Round(m/50) * 50)
}

// bdaImpactText describes the impacts against the aim point.
func bdaImpactText(t taskingTarget, bt bdaTarget, impacts []tacviewImpact) string {
	if len(impacts) == 0 {
		return "No impacts observed."
	}
	near := impacts[0]
	for _, i := range impacts[1:] {
		if metresBetween(i.Pos, t.TgtPos) < metresBetween(near.Pos, t.TgtPos) {
			near = i
		}
	}
	m := metresBetween(near.Pos, t.TgtPos)
	var b strings.Builder
	if len(impacts) > 1 {
		fmt.Fprintf(&b, "%d impacts, closest ", len(impacts))
	} else {
		b.WriteString("Impact ")
	}
	if m <= bdaGoodHitM {
		b.WriteString("on the aim point.")
	} else {
		brg, _ := bearingRangeFromRef(t.TgtPos, near.Pos[0], near.Pos[1])
		fmt.Fprintf(&b, "%d metres %s of the aim point.", roundMetres(m), compass8(brg))
	}
	if bt.NoStrike != nil {
		for _, i := range impacts {
			if metresBetween(i.Pos, *bt.NoStrike) <= bdaNoStrikeM {
				b.WriteString(" Impact on the no-strike road bridge.")
				break
			}
		}
	}
	return b.String()
}

// bdaCount is "Large tanks, 3 of 4 destroyed." / "Command building destroyed."
func bdaCount(spoken string, n, total int) string {
	title := strings.ToUpper(spoken[:1]) + spoken[1:]
	if total == 1 {
		if n == 1 {
			return title + " destroyed."
		}
		return title + " still standing."
	}
	if spoken != "checkpoint" {
		title += "s"
	}
	return fmt.Sprintf("%s, %d of %d destroyed.", title, n, total)
}

// bdaKillText describes the damage, "" when there is nothing to say.
func bdaKillText(bt bdaTarget, ground []tacviewGround) string {
	var parts []string
	if bt.Bridge {
		parts = append(parts, "Unable to assess the bridge.")
	}
	if len(bt.Objects) == 0 {
		return strings.Join(parts, " ")
	}
	seen, dead := bdaKills(bt, ground)
	if seen == 0 {
		return strings.Join(append(parts, "Unable to assess damage."), " ")
	}
	var pn, pt, n int
	for i, o := range bt.Objects {
		isP := bt.Primary != "" && strings.HasPrefix(o.Label, bt.Primary)
		if isP {
			pt++
		}
		if dead[i] {
			n++
			if isP {
				pn++
			}
		}
	}
	if n == len(bt.Objects) {
		return strings.Join(append(parts, "Target destroyed. Good work."), " ")
	}
	if pt > 0 {
		parts = append(parts, bdaCount(bt.Spoken, pn, pt))
	}
	if len(bt.Objects) > pt {
		parts = append(parts, fmt.Sprintf("%d of %d on target destroyed.", n, len(bt.Objects)))
	}
	return strings.Join(parts, " ")
}

// bdaFor is the full BDA call for one flight's pass.
func bdaFor(callsign, name string, t taskingTarget, ground []tacviewGround, impacts []tacviewImpact) string {
	bt := caucasusBDA[t.Number]
	text := fmt.Sprintf("%s, %s, BDA. %s", callsign, name, bdaImpactText(t, bt, impacts))
	if k := bdaKillText(bt, ground); k != "" {
		text += " " + k
	}
	return text
}

// flightImpacts are the impacts on t since since that belong to callsign: its
// own, or ones nobody could attribute.
func flightImpacts(all []tacviewImpact, callsign string, t taskingTarget) []tacviewImpact {
	var out []tacviewImpact
	for _, i := range all {
		if i.Shooter != "" && !strings.EqualFold(i.Shooter, callsign) {
			continue
		}
		if metresBetween(i.Pos, t.TgtPos) <= bdaImpactRadiusM {
			out = append(out, i)
		}
	}
	return out
}

func isBDARequest(lower string) bool {
	return containsAny(lower, "bda", "b.d.a", "b d a", "battle damage", "damage assessment", "say effects", "request effects")
}
