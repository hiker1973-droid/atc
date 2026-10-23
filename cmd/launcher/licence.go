package main

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/vsfg7/atc/pkg/license"
)

// Trial and licence (pkg/license): the launcher shows the status, takes a
// key on the Settings page, and refuses to start roles once the trial has
// ended without one. atc.exe checks too, so a start file run by hand stops
// as well. A git checkout (the vSFG-7 rigs) is never checked.

var (
	licMu             sync.Mutex
	licCached         license.Status
	licChecked        time.Time
	newLicenceChecker = func() *license.Checker { return license.NewChecker(rootDir) }
)

// licenceStatus is the current status, worked out at most once a minute.
func licenceStatus() license.Status {
	licMu.Lock()
	defer licMu.Unlock()
	if time.Since(licChecked) > time.Minute {
		licCached, licChecked = newLicenceChecker().Check(), time.Now()
	}
	return licCached
}

// licenceBlocks answers a start request when the trial is over; true means
// it did and the caller stops.
func licenceBlocks(w http.ResponseWriter) bool {
	st := licenceStatus()
	if st.Allowed() {
		return false
	}
	http.Error(w, st.Message, http.StatusPaymentRequired)
	return true
}

// requireLicence wraps the start handlers.
func requireLicence(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !licenceBlocks(w) {
			h(w, r)
		}
	}
}

// handleLicence: GET the status (viewer); POST (control) {"key": "SKYEYE1-..."}.
func handleLicence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, licenceStatus())
		return
	}
	if code, err := controlAllowed(r); err != nil {
		http.Error(w, err.Error(), code)
		return
	}
	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&req); err != nil {
		http.Error(w, "bad JSON", http.StatusBadRequest)
		return
	}
	st, err := newLicenceChecker().Activate(req.Key)
	licMu.Lock()
	licCached, licChecked = st, time.Now()
	licMu.Unlock()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, st)
}
