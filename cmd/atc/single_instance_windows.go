//go:build windows

package main

import (
	"errors"

	"golang.org/x/sys/windows"
)

// instanceLock holds the role's mutex for the life of the process; Windows
// releases it when the process exits or is killed.
var instanceLock windows.Handle

// claimInstance takes the role's named mutex. It returns false when another
// process already holds it.
func claimInstance(key string) (bool, error) {
	name, err := windows.UTF16PtrFromString(`Local\` + key)
	if err != nil {
		return false, err
	}
	h, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	instanceLock = h
	return true, nil
}
