package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/cybre/discordbotv3/internal/config"
	"github.com/cybre/discordbotv3/internal/player"
	"github.com/cybre/discordbotv3/internal/router"
	"github.com/cybre/discordbotv3/internal/spotify"
	"github.com/cybre/discordbotv3/internal/voice"
)

const optionQuery = "query"

// PlayCommand returns the play command definition.
func PlayCommand(spotifyClient *spotify.Client, voiceManager *voice.Manager, cfg *config.Config, playerService *player.Service) router.Command {
	return router.Command{
		Name:        "play",
		Description: "Play a track on Spotify",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         optionQuery,
				Description:  "The track to search for",
				Required:     true,
				Autocomplete: true,
			},
		},
		Handler: func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
			playerService.SetChannel(i.ChannelID)

			data := i.ApplicationCommandData()
			trackID, ok := getStringOption(data, optionQuery)
			if !ok {
				return fmt.Errorf("missing required query parameter")
			}

			// Find user's voice channel
			channelID, err := getUserVoiceChannel(s, i)
			if err != nil {
				return router.Respond(s, i, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: err.Error(),
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				}, cfg.ResponseDeleteTimeout)
			}

			wasConnected := voiceManager.IsConnected()

			// Join voice channel
			if err := voiceManager.Join(i.GuildID, channelID); err != nil {
				return fmt.Errorf("failed to join voice channel: %w", err)
			}

			// Start streaming
			if err := voiceManager.StartStreaming(cfg.AudioInputDevice); err != nil {
				slog.Error("Failed to start streaming", "error", err)
				// Continue anyway to try playing on Spotify
			}

			// Get track details
			track, err := spotifyClient.GetTrack(ctx, trackID)
			trackName := "track"
			if err != nil {
				slog.Error("Failed to get track details", "error", err)
			} else {
				trackName = formatTrackName(track.Name, track.Artists[0].Name)
			}

			// Check if something is already playing
			state, err := spotifyClient.GetPlayerState(ctx)
			if err != nil {
				slog.Error("Failed to get player state", "error", err)
				// Fallback to just playing if we can't get state
			}

			if state != nil && state.Playing && wasConnected {
				if err := spotifyClient.AddToQueue(ctx, trackID); err != nil {
					return fmt.Errorf("failed to add to queue: %w", err)
				}
				return router.Respond(s, i, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("Added %s to queue.", trackName),
					},
				}, cfg.ResponseDeleteTimeout)
			}

			if err := spotifyClient.Play(ctx, trackID); err != nil {
				if errors.Is(err, spotify.ErrNoActiveDevice) {
					return router.Respond(s, i, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "No active Spotify device found. Please open Spotify on a device and try again.",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					}, cfg.ResponseDeleteTimeout)
				}
				return fmt.Errorf("failed to play track: %w", err)
			}

			playerService.ScheduleWidgetUpdate(1 * time.Second)

			return router.Respond(s, i, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Playing %s...", trackName),
				},
			}, cfg.ResponseDeleteTimeout)
		},
		AutocompleteHandler: func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
			data := i.ApplicationCommandData()
			query, ok := getStringOption(data, optionQuery)
			if !ok || query == "" {
				return
			}

			tracks, err := spotifyClient.Search(ctx, query)
			if err != nil {
				slog.Error("Error searching Spotify", "error", err)
				return
			}

			choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(tracks))
			for _, track := range tracks {
				name := fmt.Sprintf("%s - %s", track.Name, track.Artists[0].Name)
				if len(name) > 100 {
					name = name[:100]
				}
				choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
					Name:  name,
					Value: track.ID.String(),
				})
			}

			if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionApplicationCommandAutocompleteResult,
				Data: &discordgo.InteractionResponseData{
					Choices: choices,
				},
			}); err != nil {
				slog.Error("Error sending autocomplete response", "error", err)
			}
		},
	}
}
