package spotify

import "errors"

// Sentinel errors for common Spotify API failures.
var (
	// ErrNoActiveDevice indicates no active Spotify device was found.
	ErrNoActiveDevice = errors.New("no active Spotify device found")
)
