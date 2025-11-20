package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/cybre/discordbotv3/internal/config"
	"github.com/cybre/discordbotv3/internal/router"
	"github.com/cybre/discordbotv3/internal/spotify"
	spotifyLib "github.com/zmb3/spotify/v2"
)

const (
	itemsPerPage = 10
)

// PaginationAction represents a pagination action type.
type PaginationAction string

const (
	PaginationStart PaginationAction = "start"
	PaginationPrev  PaginationAction = "prev"
	PaginationNext  PaginationAction = "next"
	PaginationEnd   PaginationAction = "end"
)

// QueueCommand returns the queue command definition.
func QueueCommand(spotifyClient *spotify.Client, cfg *config.Config, r *router.Router) router.Command {
	// Register pagination handlers
	r.RegisterComponent("queue_start", func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
		return handleQueuePagination(ctx, s, i, spotifyClient, PaginationStart)
	})
	r.RegisterComponent("queue_prev", func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
		return handleQueuePagination(ctx, s, i, spotifyClient, PaginationPrev)
	})
	r.RegisterComponent("queue_next", func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
		return handleQueuePagination(ctx, s, i, spotifyClient, PaginationNext)
	})
	r.RegisterComponent("queue_end", func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
		return handleQueuePagination(ctx, s, i, spotifyClient, PaginationEnd)
	})
	r.RegisterComponent("queue_close", func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
		return s.ChannelMessageDelete(i.ChannelID, i.Message.ID)
	})

	return router.Command{
		Name:        "queue",
		Description: "Show the current playback queue",
		Handler: func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
			return handleShowQueue(ctx, s, i, spotifyClient, cfg)
		},
	}
}

func handleShowQueue(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, spotifyClient *spotify.Client, cfg *config.Config) error {
	queue, err := spotifyClient.GetQueue(ctx)
	if err != nil {
		return router.Respond(s, i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to get queue: " + err.Error(),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		}, cfg.ResponseDeleteTimeout)
	}

	if queue == nil || (len(queue.Items) == 0 && queue.CurrentlyPlaying.ID == "") {
		return router.Respond(s, i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "The queue is empty.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		}, cfg.ResponseDeleteTimeout)
	}

	embed := buildQueueEmbed(queue, 0)
	components := buildPaginationComponents(0, len(queue.Items))

	return router.Respond(s, i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
		},
	}, cfg.ResponseDeleteTimeout)
}

func handleQueuePagination(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, spotifyClient *spotify.Client, action PaginationAction) error {
	// Parse current page from embed footer
	if len(i.Message.Embeds) == 0 || i.Message.Embeds[0].Footer == nil {
		return nil
	}

	footerText := i.Message.Embeds[0].Footer.Text
	var currentPage, totalPages int
	_, err := fmt.Sscanf(footerText, "Page %d/%d", &currentPage, &totalPages)
	if err != nil {
		return nil
	}

	newPage := currentPage - 1 // 0-indexed

	switch action {
	case PaginationStart:
		newPage = 0
	case PaginationPrev:
		newPage--
	case PaginationNext:
		newPage++
	case PaginationEnd:
		newPage = totalPages - 1
	}

	if newPage < 0 {
		newPage = 0
	}
	if newPage >= totalPages {
		newPage = totalPages - 1
	}

	queue, err := spotifyClient.GetQueue(ctx)
	if err != nil {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to get queue: " + err.Error(),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	embed := buildQueueEmbed(queue, newPage)
	components := buildPaginationComponents(newPage, len(queue.Items))

	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
		},
	})
}

func buildQueueEmbed(queue *spotifyLib.Queue, page int) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title: "Playback Queue",
		Color: 0x1DB954,
	}

	var description strings.Builder
	if queue.CurrentlyPlaying.ID != "" {
		track := queue.CurrentlyPlaying
		artists := make([]string, len(track.Artists))
		for i, a := range track.Artists {
			artists[i] = a.Name
		}
		description.WriteString(fmt.Sprintf("**Currently Playing:**\n[%s](%s) - %s\n\n**Up Next:**\n", track.Name, track.ExternalURLs["spotify"], artists[0]))
	}

	start := page * itemsPerPage
	end := min(start+itemsPerPage, len(queue.Items))

	for i := start; i < end; i++ {
		track := queue.Items[i]
		artists := make([]string, len(track.Artists))
		for j, a := range track.Artists {
			artists[j] = a.Name
		}
		description.WriteString(fmt.Sprintf("`%d.` [%s](%s) - %s\n", i+1, track.Name, track.ExternalURLs["spotify"], artists[0]))
	}

	embed.Description = description.String()

	totalPages := (len(queue.Items) + itemsPerPage - 1) / itemsPerPage
	if totalPages == 0 {
		totalPages = 1
	}
	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: fmt.Sprintf("Page %d/%d", page+1, totalPages),
	}

	return embed
}

func buildPaginationComponents(page int, totalItems int) []discordgo.MessageComponent {
	totalPages := (totalItems + itemsPerPage - 1) / itemsPerPage
	if totalPages <= 1 {
		return nil
	}

	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "⏮️",
					},
					Style:    discordgo.SecondaryButton,
					CustomID: "queue_start",
					Disabled: page == 0,
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "◀️",
					},
					Style:    discordgo.PrimaryButton,
					CustomID: "queue_prev",
					Disabled: page == 0,
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "▶️",
					},
					Style:    discordgo.PrimaryButton,
					CustomID: "queue_next",
					Disabled: page >= totalPages-1,
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "⏭️",
					},
					Style:    discordgo.SecondaryButton,
					CustomID: "queue_end",
					Disabled: page >= totalPages-1,
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "✖",
					},
					Style:    discordgo.DangerButton,
					CustomID: "queue_close",
				},
			},
		},
	}
}
