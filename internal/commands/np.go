package commands

import (
	"context"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/cybre/discordbotv3/internal/config"
	"github.com/cybre/discordbotv3/internal/player"
	"github.com/cybre/discordbotv3/internal/router"
)

// NowPlayingCommand returns the np command definition.
func NowPlayingCommand(playerService *player.Service, cfg *config.Config) router.Command {
	return router.Command{
		Name:        "np",
		Description: "Resend the now playing widget",
		Handler: func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
			// Delete the previous widget message
			playerService.DeleteMessage()

			embed, components, err := playerService.GetNowPlayingEmbed(ctx)
			if err != nil {
				return router.Respond(s, i, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Could not get player state: " + err.Error(),
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				}, cfg.ResponseDeleteTimeout)
			}

			err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds:     []*discordgo.MessageEmbed{embed},
					Components: components,
				},
			})
			if err != nil {
				return err
			}

			// Get the message to store its ID
			msg, err := s.InteractionResponse(i.Interaction)
			if err != nil {
				slog.Error("Failed to get interaction response message", "error", err)
				return nil
			}

			playerService.SetMessage(i.ChannelID, msg.ID)
			return nil
		},
	}
}
