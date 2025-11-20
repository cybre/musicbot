package voice

import (
	"context"
	"log/slog"
	"sync"
	"time"

	spotifyLib "github.com/zmb3/spotify/v2"
)

const (
	// pollInterval is how often the monitor checks Spotify player state.
	pollInterval = 5 * time.Second
)

// SpotifyClient defines the interface for checking Spotify player state.
type SpotifyClient interface {
	GetPlayerState(ctx context.Context) (*spotifyLib.PlayerState, error)
	Pause(ctx context.Context) error
}

// Monitor tracks Spotify playback activity and auto-disconnects on inactivity.
type Monitor struct {
	spotifyClient     SpotifyClient
	voiceManager      *Manager
	inactivityTimeout time.Duration

	mu               sync.Mutex
	stopChan         chan struct{}
	inactivityTimer  *time.Timer
	lastPlayingState bool
	running          bool
}

// NewMonitor creates a new inactivity monitor.
func NewMonitor(spotifyClient SpotifyClient, voiceManager *Manager, inactivityTimeout time.Duration) *Monitor {
	return &Monitor{
		spotifyClient:     spotifyClient,
		voiceManager:      voiceManager,
		inactivityTimeout: inactivityTimeout,
		stopChan:          make(chan struct{}),
	}
}

// Start begins monitoring Spotify player state for inactivity.
func (m *Monitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	go m.monitorLoop()
	slog.Info("Started inactivity monitor", "timeout", m.inactivityTimeout)
}

// Stop gracefully stops the inactivity monitor.
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false
	close(m.stopChan)

	if m.inactivityTimer != nil {
		m.inactivityTimer.Stop()
		m.inactivityTimer = nil
	}

	slog.Info("Stopped inactivity monitor")
}

// monitorLoop is the main polling loop that checks player state.
func (m *Monitor) monitorLoop() {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.checkPlayerState()
		}
	}
}

// checkPlayerState checks the current Spotify player state and manages the inactivity timer.
func (m *Monitor) checkPlayerState() {
	if !m.voiceManager.IsConnected() {
		// Not connected to voice, no need to monitor
		slog.Debug("Not connected to voice, stopping inactivity timer")
		m.stopInactivityTimer()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	state, err := m.spotifyClient.GetPlayerState(ctx)
	if err != nil {
		// Error getting state, don't take action
		slog.Debug("Failed to get player state for inactivity check", "error", err)
		return
	}

	if state == nil {
		// No active session, start inactivity timer
		m.startInactivityTimer()
		return
	}

	isPlaying := state.Playing

	m.mu.Lock()
	wasPlaying := m.lastPlayingState
	m.lastPlayingState = isPlaying
	m.mu.Unlock()

	userCount, err := m.voiceManager.GetChannelUserCount()
	if err != nil {
		slog.Error("Failed to get channel user count", "error", err)
		// Assume users are present to avoid accidental disconnect
		userCount = 1
	}

	slog.Debug("Checking player state", "isPlaying", isPlaying, "wasPlaying", wasPlaying, "userCount", userCount)

	if isPlaying && userCount > 0 {
		// Playback is active and users are present, cancel any pending disconnect
		m.stopInactivityTimer()
		if !wasPlaying {
			slog.Debug("Playback resumed, inactivity timer cancelled")
		}
	} else {
		// Playback is paused/stopped OR channel is empty, start inactivity timer
		if wasPlaying {
			slog.Debug("Playback stopped or channel empty, starting inactivity timer")
		}
		m.startInactivityTimer()
	}
}

// startInactivityTimer starts or resets the inactivity timer.
func (m *Monitor) startInactivityTimer() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.inactivityTimer != nil {
		// Timer already running, let it count down
		slog.Debug("Inactivity timer already running")
		return
	}

	slog.Info("Starting inactivity timer", "timeout", m.inactivityTimeout)

	// Create new timer
	m.inactivityTimer = time.AfterFunc(m.inactivityTimeout, func() {
		slog.Info("Inactivity timeout reached, disconnecting from voice")

		// Pause playback
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.spotifyClient.Pause(ctx); err != nil {
			slog.Error("Failed to pause Spotify on inactivity", "error", err)
		}

		if err := m.voiceManager.Leave(); err != nil {
			slog.Error("Failed to disconnect due to inactivity", "error", err)
		}
	})
}

// stopInactivityTimer stops the inactivity timer if it's running.
func (m *Monitor) stopInactivityTimer() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.inactivityTimer != nil {
		slog.Info("Stopping inactivity timer")
		m.inactivityTimer.Stop()
		m.inactivityTimer = nil
	}
}
