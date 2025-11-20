package voice

import "errors"

// Sentinel errors for voice connection and audio streaming failures.
var (
	// ErrDeviceNotFound indicates the specified audio input device was not found.
	ErrDeviceNotFound = errors.New("audio input device not found")
)
