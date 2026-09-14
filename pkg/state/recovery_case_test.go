package state

import "testing"

func TestRecoveryCaseOverride(t *testing.T) {
	s := &AirfieldState{}
	s.UpdateFlightConditions(25000, 10, false)
	if got := s.GetRecoveryCase(); got != CaseOne {
		t.Fatalf("clear day: got case %d, want 1", got)
	}

	s.SetRecoveryCaseOverride(CaseThree)
	if got := s.GetRecoveryCase(); got != CaseThree {
		t.Fatalf("after override: got case %d, want 3", got)
	}
	// Neither a weather push nor the periodic refresh may undo a pinned case.
	s.UpdateFlightConditions(25000, 10, false)
	if got := s.GetRecoveryCase(); got != CaseThree {
		t.Fatalf("after weather update: got case %d, want 3", got)
	}
	if _, cur := s.RefreshRecoveryCase(false); cur != CaseThree {
		t.Fatalf("after refresh: got case %d, want 3", cur)
	}

	// Pinning Case 1 holds even at night.
	s.SetRecoveryCaseOverride(CaseOne)
	if _, cur := s.RefreshRecoveryCase(true); cur != CaseOne {
		t.Fatalf("night with Case 1 pinned: got case %d, want 1", cur)
	}

	// Auto hands the case back to the weather and night flag.
	s.SetRecoveryCaseOverride(0)
	if got := s.GetRecoveryCase(); got != CaseThree {
		t.Fatalf("auto at night: got case %d, want 3", got)
	}
	if got := s.GetRecoveryCaseOverride(); got != 0 {
		t.Fatalf("override after auto: got %d, want 0", got)
	}
}
