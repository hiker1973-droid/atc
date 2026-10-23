package license

import (
	"time"

	"golang.org/x/sys/windows/registry"
)

// RegistryStore keeps the trial dates in HKCU\Software\Readback ATC, which an
// uninstall and reinstall do not touch.
type RegistryStore struct{}

const regPath = `Software\Readback ATC`

func (RegistryStore) Load() (start, lastSeen time.Time) {
	k, err := registry.OpenKey(registry.CURRENT_USER, regPath, registry.QUERY_VALUE)
	if err != nil {
		return
	}
	defer k.Close()
	read := func(name string) time.Time {
		v, _, err := k.GetStringValue(name)
		if err != nil {
			return time.Time{}
		}
		t, _ := time.Parse(time.RFC3339, v)
		return t
	}
	return read("TrialStart"), read("LastSeen")
}

func (RegistryStore) Save(start, lastSeen time.Time) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, regPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.SetStringValue("TrialStart", start.UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	return k.SetStringValue("LastSeen", lastSeen.UTC().Format(time.RFC3339))
}
