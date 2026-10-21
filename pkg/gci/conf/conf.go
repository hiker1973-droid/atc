// Package conf holds the few SkyEye configuration constants the vendored GCI
// packages read. It replaces SkyEye's internal/conf, which also pulls in
// whisper.cpp (cgo) and its own SRS client. Ours, not vendored.
package conf

import "github.com/martinlindhe/unit"

// DefaultPictureRadius is how far out a PICTURE looks.
var DefaultPictureRadius = 300 * unit.NauticalMile

// DefaultMarginRadius: contacts closer than this are ignored by nearest-group
// searches, so a flight does not get itself as a bogey.
const DefaultMarginRadius = 3 * unit.NauticalMile

// DefaultPlaybackSpeed is SkyEye's speech speed (unused: our TTS sets its own).
var DefaultPlaybackSpeed = 1.0
