package commands

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/cybre/discordbotv3/internal/config"
	"github.com/cybre/discordbotv3/internal/router"
	"github.com/cybre/discordbotv3/internal/spotify"
	"github.com/cybre/discordbotv3/internal/utils"
)

const optionLevel = "level"

// VolumeCommand returns the volume command definition.
func VolumeCommand(spotifyClient *spotify.Client, cfg *config.Config) router.Command {
	return router.Command{
		Name:        "volume",
		Description: "Get or set the playback volume",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        optionLevel,
				Description: "Volume level (0-100)",
				Required:    false,
				MinValue:    utils.Ptr(0.0),
				MaxValue:    100,
			},
		},
		Handler: func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
			data := i.ApplicationCommandData()

			if len(data.Options) > 0 {
				level := int(data.Options[0].IntValue())
				if err := spotifyClient.SetVolume(ctx, level); err != nil {
					return err
				}

				return router.Respond(s, i, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("Volume set to %d%%", level),
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				}, cfg.ResponseDeleteTimeout)
			}

			currentVol, err := spotifyClient.GetVolume(ctx)
			if err != nil {
				return err
			}

			return router.Respond(s, i, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Current volume is %d%%", currentVol),
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			}, cfg.ResponseDeleteTimeout)
		},
	}
}
