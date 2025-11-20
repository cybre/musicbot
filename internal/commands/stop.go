package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/cybre/discordbotv3/internal/config"
	"github.com/cybre/discordbotv3/internal/player"
	"github.com/cybre/discordbotv3/internal/router"
	"github.com/cybre/discordbotv3/internal/spotify"
	"github.com/cybre/discordbotv3/internal/voice"
)

// StopCommand returns the stop command definition.
func StopCommand(spotifyClient *spotify.Client, voiceManager *voice.Manager, playerService *player.Service, cfg *config.Config) router.Command {
	return router.Command{
		Name:        "stop",
		Description: "Stop Spotify playback and leave voice channel",
		Handler: func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
			// Pause Spotify
			if err := spotifyClient.Pause(ctx); err != nil {
				// Log error but continue to leave voice
				slog.Error("Failed to pause Spotify", "error", err)
			}

			// Leave voice channel
			if err := voiceManager.Leave(); err != nil {
				return fmt.Errorf("failed to leave voice channel: %w", err)
			}

			return router.Respond(s, i, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Stopped playback and left voice channel.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			}, cfg.ResponseDeleteTimeout)
		},
	}
}
