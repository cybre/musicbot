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

// PreviousCommand returns the previous command definition.
func PreviousCommand(spotifyClient *spotify.Client, playerService *player.Service, cfg *config.Config) router.Command {
	return router.Command{
		Name:        "previous",
		Description: "Skip to the previous track",
		Handler: func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
			if err := spotifyClient.Previous(ctx); err != nil {
				return fmt.Errorf("failed to skip to previous track: %w", err)
			}

			playerService.UpdateWidget()

			return router.Respond(s, i, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Skipped to previous track.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			}, cfg.ResponseDeleteTimeout)
		},
	}
}
