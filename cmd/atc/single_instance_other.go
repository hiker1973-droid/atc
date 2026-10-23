//go:build !windows

package main

// claimInstance has no cross-process lock off Windows, where the roles never
// run; it always lets the process start.
func claimInstance(key string) (bool, error) { return true, nil }
