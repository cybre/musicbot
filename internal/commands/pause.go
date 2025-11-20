package commands

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/cybre/discordbotv3/internal/config"
	"github.com/cybre/discordbotv3/internal/player"
	"github.com/cybre/discordbotv3/internal/router"
	"github.com/cybre/discordbotv3/internal/spotify"
)

// PauseCommand returns the pause command definition.
func PauseCommand(spotifyClient *spotify.Client, playerService *player.Service, cfg *config.Config) router.Command {
	return router.Command{
		Name:        "pause",
		Description: "Pause Spotify playback",
		Handler: func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
			if err := spotifyClient.Pause(ctx); err != nil {
				return fmt.Errorf("failed to pause playback: %w", err)
			}

			playerService.UpdateWidget()

			return router.Respond(s, i, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Paused playback.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			}, cfg.ResponseDeleteTimeout)
		},
	}
}
