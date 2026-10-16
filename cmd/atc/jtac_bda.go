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
// damage cannot be assessed; the Valley Road Bridge checkpoint and the Kuban
// Rail Bridge's no-strike road bridge (impacts only) can. With no ground objects seen at a target, the
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
	// bdaNoStrikeM: an impact within this of a no-strike object hit it.
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

// kubanRoadBridge is range_north.ROAD_BRIDGE_NO_STRIKE (TGT 3).
var kubanRoadBridge = orb.Point{41.971038, 44.077495}

// caucasusBDA is keyed by target number. Positions are every static range_north.placed()
// puts at a target (layout offsets and the TGT 1 CHECKPOINT), converted x/y -> lat/lon by
// DCS's own terrain module, so no convergence approximation.
var caucasusBDA = map[int]bdaTarget{
	1: {Primary: "checkpoint", Spoken: "checkpoint", Bridge: true, Objects: []bdaObject{
		{"checkpoint vehicle", orb.Point{41.886978, 43.897335}},
		{"checkpoint vehicle", orb.Point{41.886826, 43.897266}},
		{"checkpoint vehicle", orb.Point{41.886672, 43.897210}},
		{"checkpoint outpost", orb.Point{41.887099, 43.897236}},
	}},
	2: {Primary: "gun", Spoken: "gun", Objects: []bdaObject{
		{"gun", orb.Point{41.890250, 43.951988}},
		{"gun", orb.Point{41.890619, 43.951959}},
		{"gun", orb.Point{41.890988, 43.951930}},
		{"gun", orb.Point{41.891358, 43.951901}},
		{"gun", orb.Point{41.891727, 43.951872}},
		{"gun", orb.Point{41.892096, 43.951843}},
		{"rocket launcher", orb.Point{41.890860, 43.951490}},
		{"rocket launcher", orb.Point{41.891352, 43.951451}},
		{"ammunition truck", orb.Point{41.890745, 43.951139}},
		{"ammunition truck", orb.Point{41.891053, 43.951115}},
		{"ammunition truck", orb.Point{41.891360, 43.951090}},
		{"ammo dump", orb.Point{41.890623, 43.950743}},
		{"ammo dump", orb.Point{41.891362, 43.950685}},
		{"tent", orb.Point{41.892016, 43.951309}},
		{"command vehicle", orb.Point{41.890169, 43.951454}},
	}},
	3: {Bridge: true, NoStrike: &kubanRoadBridge},
	4: {Primary: "large tank", Spoken: "large tank", Objects: []bdaObject{
		{"large tank", orb.Point{42.114338, 44.191097}},
		{"large tank", orb.Point{42.114770, 44.191062}},
		{"large tank", orb.Point{42.115190, 44.191028}},
		{"large tank", orb.Point{42.115623, 44.190993}},
		{"small tank", orb.Point{42.114547, 44.190675}},
		{"small tank", orb.Point{42.115289, 44.190615}},
		{"fuel bladder", orb.Point{42.114561, 44.190359}},
		{"fuel bladder", orb.Point{42.115178, 44.190309}},
		{"fuel truck", orb.Point{42.114567, 44.189998}},
		{"fuel truck", orb.Point{42.114814, 44.189978}},
		{"fuel truck", orb.Point{42.115061, 44.189959}},
		{"power box", orb.Point{42.115817, 44.190393}},
		{"generator", orb.Point{42.113964, 44.190542}},
		{"workshop", orb.Point{42.115084, 44.191712}},
	}},
	5: {Primary: "command building", Spoken: "command building", Objects: []bdaObject{
		{"command building", orb.Point{41.980892, 44.336361}},
		{"radio mast", orb.Point{41.981442, 44.336677}},
		{"bunker", orb.Point{41.980459, 44.335990}},
		{"bunker", orb.Point{41.981202, 44.335931}},
		{"tent", orb.Point{41.980376, 44.336672}},
		{"tent", orb.Point{41.980128, 44.336691}},
		{"tent", orb.Point{41.979880, 44.336711}},
		{"generator", orb.Point{41.981684, 44.336208}},
		{"command vehicle", orb.Point{41.980857, 44.336949}},
		{"command vehicle", orb.Point{41.981105, 44.336929}},
		{"jeep", orb.Point{41.980789, 44.335694}},
		{"jeep", orb.Point{41.980938, 44.335682}},
		{"command vehicle", orb.Point{41.981552, 44.336984}},
		{"outpost", orb.Point{41.980728, 44.335294}},
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
		return strings.Join(append(parts, bdaDestroyed), " ")
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
// bdaDestroyed ends the kill text when every object on the target is down.
const bdaDestroyed = "Target destroyed."

func bdaFor(callsign, name string, t taskingTarget, ground []tacviewGround, impacts []tacviewImpact) string {
	bt := caucasusBDA[t.Number]
	text := fmt.Sprintf("%s, %s, BDA. %s", callsign, name, bdaImpactText(t, bt, impacts))
	if k := bdaKillText(bt, ground); k != "" {
		// Operator 2026-10-15: "target destroyed Raider XX thanks for the help".
		if strings.HasSuffix(k, bdaDestroyed) {
			k = strings.TrimSuffix(k, bdaDestroyed) + fmt.Sprintf("Target destroyed, %s. Thanks for the help.", callsign)
		}
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
