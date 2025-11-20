// Package player manages the Discord player widget and Spotify playback state.
// It provides a real-time player interface with controls for playback, navigation,
// shuffle, and repeat modes.
package player

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/cybre/discordbotv3/internal/router"
	"github.com/cybre/discordbotv3/internal/spotify"
	"github.com/cybre/discordbotv3/internal/voice"
	spotifyLib "github.com/zmb3/spotify/v2"
)

const (
	// pollInterval is how often the player widget is updated automatically.
	pollInterval = 5 * time.Second

	// widgetUpdateDelay is how long to wait after a player action before updating the widget.
	widgetUpdateDelay = 200 * time.Millisecond

	// Component custom IDs for player controls.
	componentResume   = "player_resume"
	componentPause    = "player_pause"
	componentPrevious = "player_previous"
	componentNext     = "player_next"
	componentStop     = "player_stop"
	componentShuffle  = "player_shuffle"
	componentRepeat   = "player_repeat"
)

// Service manages the player widget and state polling.
type Service struct {
	spotifyClient   *spotify.Client
	discordSession  *discordgo.Session
	widgetMessageID string
	widgetChannelID string
	mu              sync.Mutex
}

// New creates a new player service.
func New(spotifyClient *spotify.Client, discordSession *discordgo.Session) *Service {
	return &Service{
		spotifyClient:  spotifyClient,
		discordSession: discordSession,
	}
}

// RegisterHandlers registers the player component handlers with the router.
func (s *Service) RegisterHandlers(r *router.Router, voiceManager *voice.Manager) {
	r.RegisterComponent(componentPrevious, func(ctx context.Context, sess *discordgo.Session, i *discordgo.InteractionCreate) error {
		if err := s.spotifyClient.Previous(ctx); err != nil {
			slog.Error("Failed to skip to previous track", "error", err)
		}
		time.Sleep(widgetUpdateDelay)

		embed, components, err := s.GetNowPlayingEmbed(ctx)
		if err != nil {
			return sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
		}

		return sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Embeds:     []*discordgo.MessageEmbed{embed},
				Components: components,
			},
		})
	})

	r.RegisterComponent(componentNext, func(ctx context.Context, sess *discordgo.Session, i *discordgo.InteractionCreate) error {
		if err := s.spotifyClient.Next(ctx); err != nil {
			slog.Error("Failed to skip track", "error", err)
		}
		time.Sleep(widgetUpdateDelay)

		embed, components, err := s.GetNowPlayingEmbed(ctx)
		if err != nil {
			return sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
		}

		return sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Embeds:     []*discordgo.MessageEmbed{embed},
				Components: components,
			},
		})
	})

	r.RegisterComponent(componentResume, func(ctx context.Context, sess *discordgo.Session, i *discordgo.InteractionCreate) error {
		if err := s.spotifyClient.Resume(ctx); err != nil {
			slog.Error("Failed to resume playback", "error", err)
		}
		time.Sleep(widgetUpdateDelay)

		embed, components, err := s.GetNowPlayingEmbed(ctx)
		if err != nil {
			return sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
		}

		return sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Embeds:     []*discordgo.MessageEmbed{embed},
				Components: components,
			},
		})
	})

	r.RegisterComponent(componentPause, func(ctx context.Context, sess *discordgo.Session, i *discordgo.InteractionCreate) error {
		if err := s.spotifyClient.Pause(ctx); err != nil {
			slog.Error("Failed to pause playback", "error", err)
		}
		time.Sleep(widgetUpdateDelay)

		embed, components, err := s.GetNowPlayingEmbed(ctx)
		if err != nil {
			return sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
		}

		return sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Embeds:     []*discordgo.MessageEmbed{embed},
				Components: components,
			},
		})
	})

	r.RegisterComponent(componentStop, func(ctx context.Context, sess *discordgo.Session, i *discordgo.InteractionCreate) error {
		if err := s.spotifyClient.Pause(ctx); err != nil {
			slog.Error("Failed to pause playback (stop)", "error", err)
		}
		if err := voiceManager.Leave(); err != nil {
			slog.Error("Failed to leave voice channel", "error", err)
		}
		return sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
	})

	r.RegisterComponent(componentShuffle, func(ctx context.Context, sess *discordgo.Session, i *discordgo.InteractionCreate) error {
		state, err := s.spotifyClient.GetPlayerState(ctx)
		if err != nil {
			slog.Error("Failed to get player state for shuffle", "error", err)
			return sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
		}

		newShuffleState := !state.ShuffleState
		if err := s.spotifyClient.Shuffle(ctx, newShuffleState); err != nil {
			slog.Error("Failed to toggle shuffle", "error", err)
		}
		time.Sleep(widgetUpdateDelay)

		embed, components, err := s.GetNowPlayingEmbed(ctx)
		if err != nil {
			return sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
		}

		return sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Embeds:     []*discordgo.MessageEmbed{embed},
				Components: components,
			},
		})
	})

	r.RegisterComponent(componentRepeat, func(ctx context.Context, sess *discordgo.Session, i *discordgo.InteractionCreate) error {
		state, err := s.spotifyClient.GetPlayerState(ctx)
		if err != nil {
			slog.Error("Failed to get player state for repeat", "error", err)
			return sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
		}

		var newRepeatState spotify.RepeatState
		switch state.RepeatState {
		case "off":
			newRepeatState = spotify.RepeatContext
		case "context":
			newRepeatState = spotify.RepeatTrack
		default:
			newRepeatState = spotify.RepeatOff
		}

		if err := s.spotifyClient.Repeat(ctx, newRepeatState); err != nil {
			slog.Error("Failed to set repeat mode", "error", err)
		}
		time.Sleep(widgetUpdateDelay)

		embed, components, err := s.GetNowPlayingEmbed(ctx)
		if err != nil {
			return sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
		}

		return sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Embeds:     []*discordgo.MessageEmbed{embed},
				Components: components,
			},
		})
	})
}

// Start starts the polling loop.
func (s *Service) Start() {
	ticker := time.NewTicker(pollInterval)
	go func() {
		for range ticker.C {
			s.UpdateWidget()
		}
	}()
}

// SetChannel sets the channel for the widget.
func (s *Service) SetChannel(channelID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.widgetChannelID != channelID {
		s.widgetChannelID = channelID
		s.widgetMessageID = ""
	}
}

// SetMessage sets the widget channel and message ID manually.
func (s *Service) SetMessage(channelID, messageID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.widgetChannelID = channelID
	s.widgetMessageID = messageID
}

// DeleteMessage deletes the current widget message if it exists.
func (s *Service) DeleteMessage() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.widgetChannelID != "" && s.widgetMessageID != "" {
		if err := s.discordSession.ChannelMessageDelete(s.widgetChannelID, s.widgetMessageID); err != nil {
			slog.Error("Failed to delete widget message", "error", err)
		}
		s.widgetChannelID = ""
		s.widgetMessageID = ""
	}
}

// ScheduleWidgetUpdate schedules a widget update after a delay.
// This is used to update the widget after Spotify API calls that may take time to propagate.
func (s *Service) ScheduleWidgetUpdate(delay time.Duration) {
	time.AfterFunc(delay, func() {
		s.UpdateWidget()
	})
}

// GetNowPlayingEmbed fetches the current state and returns the widget embed and components.
func (s *Service) GetNowPlayingEmbed(ctx context.Context) (*discordgo.MessageEmbed, []discordgo.MessageComponent, error) {
	state, err := s.spotifyClient.GetPlayerState(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get player state: %w", err)
	}

	if state == nil || state.Item == nil {
		return nil, nil, fmt.Errorf("nothing is currently playing")
	}

	return buildEmbed(state), buildComponents(state), nil
}

// UpdateWidget fetches state and updates the widget message.
func (s *Service) UpdateWidget() {
	s.mu.Lock()
	channelID := s.widgetChannelID
	messageID := s.widgetMessageID
	s.mu.Unlock()

	if channelID == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	state, err := s.spotifyClient.GetPlayerState(ctx)
	if err != nil {
		slog.Error("Failed to get player state for widget", "error", err)
		return
	}

	if state == nil || state.Item == nil {
		return
	}

	embed := buildEmbed(state)
	components := buildComponents(state)

	data := &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: components,
	}

	if messageID == "" {
		msg, err := s.discordSession.ChannelMessageSendComplex(channelID, data)
		if err != nil {
			slog.Error("Failed to send widget message", "error", err)
			return
		}
		s.mu.Lock()
		s.widgetMessageID = msg.ID
		s.mu.Unlock()
	} else {
		edit := &discordgo.MessageEdit{
			Embeds:     &[]*discordgo.MessageEmbed{embed},
			Components: &components,
			ID:         messageID,
			Channel:    channelID,
		}
		_, err := s.discordSession.ChannelMessageEditComplex(edit)
		if err != nil {
			slog.Error("Failed to edit widget message", "error", err)
			if strings.Contains(err.Error(), "Unknown Message") {
				s.mu.Lock()
				s.widgetMessageID = ""
				s.mu.Unlock()
			}
		}
	}
}

func buildEmbed(state *spotifyLib.PlayerState) *discordgo.MessageEmbed {
	track := state.Item
	status := "Playing"
	if !state.Playing {
		status = "Paused"
	}

	artists := make([]string, len(track.Artists))
	for i, a := range track.Artists {
		artists[i] = a.Name
	}

	duration := track.Duration
	progress := state.Progress

	spotifyURL := track.ExternalURLs["spotify"]
	releaseDate := track.Album.ReleaseDate
	popularity := track.Popularity
	explicit := "No"
	if track.Explicit {
		explicit = "Yes"
	}

	footer := fmt.Sprintf("%s / %s", formatDuration(int(progress)), formatDuration(int(duration)))

	var thumbnail *discordgo.MessageEmbedThumbnail
	if len(track.Album.Images) > 0 {
		thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: track.Album.Images[0].URL,
		}
	}

	embed := &discordgo.MessageEmbed{
		Title:       track.Name,
		URL:         spotifyURL,
		Description: fmt.Sprintf("by **%s**\n\nAlbum: **%s**\nReleased: **%s**\nPopularity: **%d%%**\nExplicit: **%s**\n\nStatus: **%s**", strings.Join(artists, ", "), track.Album.Name, releaseDate, popularity, explicit, status),
		Color:       0x1DB954, // Spotify Green
		Thumbnail:   thumbnail,
		Footer: &discordgo.MessageEmbedFooter{
			Text: footer,
		},
	}

	return embed
}

func buildComponents(state *spotifyLib.PlayerState) []discordgo.MessageComponent {
	playPauseEmoji := "▶️"
	playPauseID := componentResume
	playPauseStyle := discordgo.SuccessButton
	if state.Playing {
		playPauseEmoji = "⏸️"
		playPauseID = componentPause
		playPauseStyle = discordgo.SecondaryButton
	}

	shuffleStyle := discordgo.SecondaryButton
	if state.ShuffleState {
		shuffleStyle = discordgo.SuccessButton
	}

	repeatEmoji := "🔁"
	repeatStyle := discordgo.SecondaryButton
	switch state.RepeatState {
	case "track":
		repeatEmoji = "🔂"
		repeatStyle = discordgo.SuccessButton
	case "context":
		repeatStyle = discordgo.SuccessButton
	}

	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "⏮️",
					},
					Style:    discordgo.SecondaryButton,
					CustomID: componentPrevious,
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: playPauseEmoji,
					},
					Style:    playPauseStyle,
					CustomID: playPauseID,
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "⏹️",
					},
					Style:    discordgo.DangerButton,
					CustomID: componentStop,
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "⏭️",
					},
					Style:    discordgo.SecondaryButton,
					CustomID: componentNext,
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "🔀",
					},
					Style:    shuffleStyle,
					CustomID: componentShuffle,
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: repeatEmoji,
					},
					Style:    repeatStyle,
					CustomID: componentRepeat,
				},
			},
		},
	}
}

func formatDuration(ms int) string {
	d := time.Duration(ms) * time.Millisecond
	return fmt.Sprintf("%d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}
