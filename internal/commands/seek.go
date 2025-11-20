package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/cybre/discordbotv3/internal/config"
	"github.com/cybre/discordbotv3/internal/player"
	"github.com/cybre/discordbotv3/internal/router"
	"github.com/cybre/discordbotv3/internal/spotify"
)

const optionPosition = "position"

// SeekCommand returns the seek command definition.
func SeekCommand(spotifyClient *spotify.Client, playerService *player.Service, cfg *config.Config) router.Command {
	return router.Command{
		Name:        "seek",
		Description: "Seek to a position in the current track",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        optionPosition,
				Description: "Position to seek to (e.g. 1:30 or 90)",
				Required:    true,
			},
		},
		Handler: func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
			data := i.ApplicationCommandData()
			positionStr, ok := getStringOption(data, optionPosition)
			if !ok {
				return fmt.Errorf("missing required position parameter")
			}

			positionMs, err := parsePosition(positionStr)
			if err != nil {
				return router.Respond(s, i, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("Invalid position format: %v. Please use MM:SS or seconds.", err),
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				}, cfg.ResponseDeleteTimeout)
			}

			if err := spotifyClient.Seek(ctx, positionMs); err != nil {
				return fmt.Errorf("failed to seek: %w", err)
			}

			playerService.ScheduleWidgetUpdate(1 * time.Second)

			return router.Respond(s, i, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Seeked to %s.", positionStr),
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			}, cfg.ResponseDeleteTimeout)
		},
	}
}

func parsePosition(input string) (int, error) {
	if strings.Contains(input, ":") {
		parts := strings.Split(input, ":")
		if len(parts) != 2 {
			return 0, fmt.Errorf("invalid format")
		}
		min, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid minutes")
		}
		sec, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, fmt.Errorf("invalid seconds")
		}
		return (min*60 + sec) * 1000, nil
	}

	sec, err := strconv.Atoi(input)
	if err != nil {
		return 0, fmt.Errorf("invalid seconds")
	}
	return sec * 1000, nil
}
