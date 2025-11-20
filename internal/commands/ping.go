package commands

import (
	"context"

	"github.com/bwmarrin/discordgo"
	"github.com/cybre/discordbotv3/internal/config"
	"github.com/cybre/discordbotv3/internal/router"
)

// PingCommand returns the ping command definition.
func PingCommand(cfg *config.Config) router.Command {
	return router.Command{
		Name:        "ping",
		Description: "Responds with Pong!",
		Handler: func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
			return router.Respond(s, i, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Pong!",
				},
			}, cfg.ResponseDeleteTimeout)
		},
	}
}
