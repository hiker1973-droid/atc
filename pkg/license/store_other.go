//go:build !windows

package license

import "time"

// RegistryStore is Windows-only; elsewhere licence.json alone keeps the trial.
type RegistryStore struct{}

func (RegistryStore) Load() (start, lastSeen time.Time)    { return }
func (RegistryStore) Save(start, lastSeen time.Time) error { return nil }
