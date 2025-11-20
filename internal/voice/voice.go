// Package voice manages Discord voice connections and audio streaming.
// It handles joining voice channels, streaming audio from input devices,
// and encoding audio data using Opus codec.
package voice

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/gordonklaus/portaudio"
	"github.com/hraban/opus"
)

const (
	// sampleRate is the audio sample rate in Hz (Discord requires 48kHz).
	sampleRate = 48000

	// channels is the number of audio channels (stereo).
	channels = 2

	// frameSize is the number of samples per frame (20ms at 48kHz).
	frameSize = 960

	// maxOpusPacketSize is the maximum size of an Opus encoded packet.
	maxOpusPacketSize = 1000
)

// Manager handles voice connections and audio streaming.
type Manager struct {
	session   *discordgo.Session
	vc        *discordgo.VoiceConnection
	mu        sync.Mutex
	streaming bool
	cancelCtx context.CancelFunc
	onLeave   []func()
}

// New creates a new Voice Manager.
func New(s *discordgo.Session) *Manager {
	return &Manager{
		session: s,
	}
}

// Join joins a voice channel.
func (m *Manager) Join(guildID, channelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.vc != nil && m.vc.ChannelID == channelID {
		return nil
	}

	vc, err := m.session.ChannelVoiceJoin(guildID, channelID, false, true)
	if err != nil {
		return fmt.Errorf("failed to join voice channel: %w", err)
	}

	m.vc = vc
	return nil
}

// IsConnected returns whether the bot is currently in a voice channel.
func (m *Manager) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.vc != nil
}

// GetChannelUserCount returns the number of users in the current voice channel.
// It excludes bots from the count.
func (m *Manager) GetChannelUserCount() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.vc == nil {
		return 0, nil
	}

	guildID := m.vc.GuildID
	channelID := m.vc.ChannelID

	guild, err := m.session.State.Guild(guildID)
	if err != nil {
		return 0, fmt.Errorf("failed to get guild: %w", err)
	}

	count := 0
	selfID := m.session.State.User.ID
	for _, vs := range guild.VoiceStates {
		if vs.ChannelID == channelID {
			if vs.UserID != selfID {
				count++
			}
		}
	}

	return count, nil
}

// StartStreaming starts streaming audio from the specified device.
func (m *Manager) StartStreaming(deviceName string) error {
	m.mu.Lock()
	if m.streaming {
		m.mu.Unlock()
		return nil
	}
	m.streaming = true
	m.mu.Unlock()

	if err := portaudio.Initialize(); err != nil {
		m.mu.Lock()
		m.streaming = false
		m.mu.Unlock()
		return fmt.Errorf("failed to initialize portaudio: %w", err)
	}

	devices, err := portaudio.Devices()
	if err != nil {
		if err := portaudio.Terminate(); err != nil {
			slog.Error("Failed to terminate portaudio", "error", err)
		}
		m.mu.Lock()
		m.streaming = false
		m.mu.Unlock()
		return fmt.Errorf("failed to list devices: %w", err)
	}

	var inputDevice *portaudio.DeviceInfo
	for _, device := range devices {
		if device.Name == deviceName {
			inputDevice = device
			break
		}
	}

	if inputDevice == nil {
		if err := portaudio.Terminate(); err != nil {
			slog.Error("Failed to terminate portaudio", "error", err)
		}
		m.mu.Lock()
		m.streaming = false
		m.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrDeviceNotFound, deviceName)
	}

	encoder, err := opus.NewEncoder(sampleRate, channels, opus.AppAudio)
	if err != nil {
		if err := portaudio.Terminate(); err != nil {
			slog.Error("Failed to terminate portaudio", "error", err)
		}
		m.mu.Lock()
		m.streaming = false
		m.mu.Unlock()
		return fmt.Errorf("failed to create opus encoder: %w", err)
	}

	in := make([]float32, frameSize*channels)
	stream, err := portaudio.OpenStream(portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   inputDevice,
			Channels: channels,
			Latency:  inputDevice.DefaultLowInputLatency,
		},
		SampleRate:      sampleRate,
		FramesPerBuffer: frameSize,
	}, in)
	if err != nil {
		if err := portaudio.Terminate(); err != nil {
			slog.Error("Failed to terminate portaudio", "error", err)
		}
		m.mu.Lock()
		m.streaming = false
		m.mu.Unlock()
		return fmt.Errorf("failed to open stream: %w", err)
	}

	if err := stream.Start(); err != nil {
		if err := portaudio.Terminate(); err != nil {
			slog.Error("Failed to terminate portaudio", "error", err)
		}
		m.mu.Lock()
		m.streaming = false
		m.mu.Unlock()
		return fmt.Errorf("failed to start stream: %w", err)
	}

	slog.Info("Started streaming audio", "device", deviceName)

	// Create a cancellable context for the streaming goroutine
	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.cancelCtx = cancel
	m.mu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			m.streaming = false
			m.cancelCtx = nil
			m.mu.Unlock()

			if err := stream.Close(); err != nil {
				slog.Error("Failed to close audio stream", "error", err)
			}

			if err := portaudio.Terminate(); err != nil {
				slog.Error("Failed to terminate portaudio", "error", err)
			}
		}()

		pcm := make([]int16, frameSize*channels)
		opusBuffer := make([]byte, maxOpusPacketSize)

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if err := stream.Read(); err != nil {
				slog.Error("Error reading from stream", "error", err)
				continue
			}

			// Convert float32 to int16
			for i, v := range in {
				pcm[i] = int16(v * 32767)
			}

			n, err := encoder.Encode(pcm, opusBuffer)
			if err != nil {
				slog.Error("Error encoding opus", "error", err)
				continue
			}

			if m.vc == nil || !m.vc.Ready {
				continue
			}

			m.vc.OpusSend <- opusBuffer[:n]
		}
	}()

	return nil
}

// StopStreaming stops the audio stream.
func (m *Manager) StopStreaming() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancelCtx != nil {
		m.cancelCtx()
	}
	m.streaming = false
}

// Leave leaves the voice channel.
func (m *Manager) Leave() error {
	m.StopStreaming()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.vc == nil {
		return nil
	}

	if err := m.vc.Disconnect(); err != nil {
		return fmt.Errorf("failed to disconnect: %w", err)
	}

	m.vc = nil

	for _, fn := range m.onLeave {
		fn()
	}

	return nil
}

func (m *Manager) OnLeave(fn func()) {
	m.onLeave = append(m.onLeave, fn)
}
