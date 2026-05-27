package airfield

import (
	"testing"

	"github.com/paulmach/orb"
)

func TestThresholdFor(t *testing.T) {
	af := &Airfield{
		RunwayPairs: []RunwayPair{
			{
				Primary:    Runway{Designator: "09", ThresholdLatLon: orb.Point{55.3550, 25.0333}},
				Reciprocal: Runway{Designator: "27", ThresholdLatLon: orb.Point{55.3833, 25.0333}},
			},
		},
	}

	got, ok := af.ThresholdFor("09")
	if !ok {
		t.Fatal("ThresholdFor(09) returned ok=false")
	}
	if got != (orb.Point{55.3550, 25.0333}) {
		t.Errorf("ThresholdFor(09) = %v, want primary threshold", got)
	}

	got, ok = af.ThresholdFor("27")
	if !ok {
		t.Fatal("ThresholdFor(27) returned ok=false")
	}
	if got != (orb.Point{55.3833, 25.0333}) {
		t.Errorf("ThresholdFor(27) = %v, want reciprocal threshold", got)
	}

	_, ok = af.ThresholdFor("Bogus")
	if ok {
		t.Error("ThresholdFor(Bogus) returned ok=true; want false for unknown designator")
	}
}
