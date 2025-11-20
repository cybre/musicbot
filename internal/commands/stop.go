package commands

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/cybre/discordbotv3/internal/config"
	"github.com/cybre/discordbotv3/internal/router"
	"github.com/cybre/discordbotv3/internal/voice"
)

// StopCommand returns the stop command definition.
func StopCommand(voiceManager *voice.Manager, cfg *config.Config) router.Command {
	return router.Command{
		Name:        "stop",
		Description: "Stop Spotify playback and leave voice channel",
		Handler: func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
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
