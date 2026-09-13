package controller

import "testing"

// ParseIntent is a first-match keyword switch, so case order and substring
// collisions decide what a pilot actually hears back. Each block below pins
// one failure class; the last block guards the calls that already worked.
func TestParseIntentClassification(t *testing.T) {
	const tower = "Senaki Tower"
	cases := []struct {
		name string
		text string
		want RequestType
	}{
		// Emergencies win over any position or phase in the same call.
		{"mayday with distance", "Senaki Tower, Raider 11, mayday mayday mayday, 10 miles south, engine failure", RequestEmergency},
		{"mayday on base", "Senaki Tower, Raider 11, mayday, turning base", RequestEmergency},
		{"emergency just airborne", "Senaki Tower, Raider 11, declaring emergency, just airborne", RequestEmergency},
		{"pan pan with dme", "Senaki Tower, Raider 11, pan pan, 7 dme", RequestEmergency},

		// Live misses from Training 1 Senaki (.220:6013).
		{"request for taxi", "Senaki Tower, Raider 032, request for taxi.", RequestTaxiClear},
		{"this is, request for taxi", "Senaki Tower, this is 032, request for taxi.", RequestTaxiClear},
		{"rolling", "Senaki traffic, Raider 302, rolling runway 09.", RequestRolling},
		{"on the roll", "Senaki traffic, Raider 302, on the roll runway 09", RequestRolling},
		{"requesting for takeoff", "Senaki Tower, Raider 11, requesting for takeoff", RequestTakeoffClear},

		// Substring collisions.
		{"returning to base is inbound", "Senaki Tower, Raider 11, returning to base", RequestDistanceInitial},
		{"rtb is inbound", "Senaki Tower, Raider 11, RTB", RequestDistanceInitial},
		{"home base is not turning base", "Senaki Tower, Raider 11, inbound home base", RequestDistanceInitial},
		{"visual approach is straight in", "Senaki Tower, Raider 11, request visual approach", RequestStraightIn},
		{"details is not ils", "Senaki Tower, Raider 11, request taxi, details to follow", RequestTaxiClear},
		{"patrolling is not rolling", "Senaki Tower, Raider 11, patrolling, radio check", RequestRadioCheck},
		{"totally is not tally", "Senaki Tower, Raider 11, totally lost comms, radio check", RequestRadioCheck},

		// Round two: say again, wind check, hung ordnance, landing options.
		{"say again", "Senaki Tower, Raider 11, say again", RequestSayAgain},
		{"say again your last", "Senaki Tower, Raider 11, say again your last", RequestSayAgain},
		{"pilot repeating himself is not say again", "Senaki Tower, Raider 11, I say again, 10 mile initial", RequestDistanceInitial},
		{"wind check", "Senaki Tower, Raider 11, wind check", RequestWindCheck},
		{"say altimeter", "Senaki Tower, Raider 11, say altimeter", RequestWindCheck},
		{"request winds", "Senaki Tower, Raider 11, request winds", RequestWindCheck},
		{"altitude check stays altitude", "Senaki Tower, Raider 11, altitude check", RequestAltitude},
		{"hung ordnance with position", "Senaki Tower, Raider 11, hung ordnance, 10 miles south", RequestHungOrdnance},
		{"hung ordinance spelling", "Senaki Tower, Raider 11, inbound with hung ordinance", RequestHungOrdnance},
		{"hung store", "Senaki Tower, Raider 11, hung store, request straight in", RequestHungOrdnance},
		{"mayday beats hung ordnance", "Senaki Tower, Raider 11, mayday, hung ordnance", RequestEmergency},
		{"final touch and go", "Senaki Tower, Raider 11, on final, touch and go", RequestLandingClear},
		{"request touch and go", "Senaki Tower, Raider 11, request touch and go", RequestLandingClear},
		{"request low approach", "Senaki Tower, Raider 11, request low approach", RequestLandingClear},
		{"request the option", "Senaki Tower, Raider 11, request the option", RequestLandingClear},
		{"full stop", "Senaki Tower, Raider 11, full stop", RequestLandingClear},
		{"base with touch and go stays base", "Senaki Tower, Raider 11, turning base, touch and go", RequestBase},

		// Existing behaviour that must survive.
		{"turning base", "Senaki Tower, Raider 11, turning base", RequestBase},
		{"left base", "Senaki Tower, Raider 11, left base runway 09", RequestBase},
		{"traffic in sight", "Senaki Tower, Raider 11, traffic in sight", RequestTrafficInSight},
		{"tally", "Senaki Tower, Raider 11, tally", RequestTrafficInSight},
		{"visual on traffic", "Senaki Tower, Raider 11, visual", RequestTrafficInSight},
		{"ils", "Senaki Tower, Raider 11, request ILS runway 09", RequestStraightIn},
		{"straight in", "Senaki Tower, Raider 11, request straight in", RequestStraightIn},
		{"airborne", "Senaki Tower, Raider 11, airborne", RequestClearTraffic},
		{"departing", "Senaki traffic, Raider 11, departing runway 09", RequestClearTraffic},
		{"7 dme", "Senaki Tower, Raider 11, 7 dme", RequestDistanceCheck},
		{"10 mile initial", "Senaki Tower, Raider 11, 10 mile initial", RequestDistanceInitial},
		{"3 mile initial", "Senaki Tower, Raider 11, 3 mile initial", RequestOverhead},
		{"bare inbound", "Senaki Tower, Raider 11, inbound", RequestDistanceInitial},
		{"request taxi", "Senaki Tower, Raider 11, request taxi", RequestTaxiClear},
		{"holding short", "Senaki Tower, Raider 11, holding short runway 09", RequestHoldingShort},
		{"request takeoff", "Senaki Tower, Raider 11, request takeoff", RequestTakeoffClear},
		{"runway vacated", "Senaki Tower, Raider 11, runway vacated", RequestRunwayVacated},
		{"radio check", "Senaki Tower, Raider 11, radio check", RequestRadioCheck},
		{"on final", "Senaki Tower, Raider 11, on final, gear down", RequestLandingClear},
		{"going around", "Senaki Tower, Raider 11, going around", RequestGoAround},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := ParseIntent(tc.text, tower)
			if req == nil {
				t.Fatalf("ParseIntent(%q) = nil, want type %d", tc.text, tc.want)
			}
			if req.Type != tc.want {
				t.Errorf("ParseIntent(%q).Type = %d, want %d", tc.text, req.Type, tc.want)
			}
		})
	}
}

func TestParseIntentIgnoresOtherStations(t *testing.T) {
	if req := ParseIntent("Kutaisi Tower, Raider 11, mayday, 10 miles", "Senaki Tower"); req != nil {
		t.Errorf("call to another tower was accepted as type %d", req.Type)
	}
}

func TestContainsWord(t *testing.T) {
	cases := []struct {
		s     string
		words []string
		want  bool
	}{
		{"turning base.", []string{"base"}, true},
		{"database error", []string{"base"}, false},
		{"request ils, runway 09", []string{"ils"}, true},
		{"details to follow", []string{"ils"}, false},
		{"returning to base", []string{"to base"}, true},
		{"returning to  base", []string{"to base"}, true}, // collapsed whitespace
		{"on the roll", []string{"rolling", "on the roll"}, true},
		{"patrolling", []string{"rolling"}, false},
	}
	for _, tc := range cases {
		if got := containsWord(tc.s, tc.words...); got != tc.want {
			t.Errorf("containsWord(%q, %q) = %v, want %v", tc.s, tc.words, got, tc.want)
		}
	}
}
