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

// ResumeCommand returns the resume command definition.
func ResumeCommand(spotifyClient *spotify.Client, playerService *player.Service, cfg *config.Config) router.Command {
	return router.Command{
		Name:        "resume",
		Description: "Resume Spotify playback",
		Handler: func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
			if err := spotifyClient.Resume(ctx); err != nil {
				return fmt.Errorf("failed to resume playback: %w", err)
			}

			playerService.ScheduleWidgetUpdate(1 * time.Second)

			return router.Respond(s, i, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Resumed playback.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			}, cfg.ResponseDeleteTimeout)
		},
	}
}
