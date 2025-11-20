package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/cybre/discordbotv3/internal/config"
	"github.com/cybre/discordbotv3/internal/player"
	"github.com/cybre/discordbotv3/internal/router"
	"github.com/cybre/discordbotv3/internal/spotify"
)

// NextCommand returns the next command definition.
func NextCommand(spotifyClient *spotify.Client, playerService *player.Service, cfg *config.Config) router.Command {
	return router.Command{
		Name:        "next",
		Description: "Skip to the next track",
		Handler: func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
			if err := spotifyClient.Next(ctx); err != nil {
				return fmt.Errorf("failed to skip track: %w", err)
			}

			playerService.ScheduleWidgetUpdate(1 * time.Second)

			return router.Respond(s, i, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Skipped to next track.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			}, cfg.ResponseDeleteTimeout)
		},
	}
}
