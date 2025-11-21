package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/cybre/discordbotv3/internal/config"
	"github.com/cybre/discordbotv3/internal/player"
	"github.com/cybre/discordbotv3/internal/router"
	"github.com/cybre/discordbotv3/internal/spotify"
	"github.com/cybre/discordbotv3/internal/voice"
)

const (
	optionQuery        = "query"
	defaultSearchLimit = 10
)

// PlayCommand returns the play command definition.
func PlayCommand(spotifyClient *spotify.Client, voiceManager *voice.Manager, cfg *config.Config, playerService *player.Service) router.Command {
	return router.Command{
		Name:        "play",
		Description: "Play a track on Spotify",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         optionQuery,
				Description:  "The track to search for",
				Required:     true,
				Autocomplete: true,
			},
		},
		Handler: func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
			playerService.SetChannel(i.ChannelID)

			data := i.ApplicationCommandData()
			trackID, ok := getStringOption(data, optionQuery)
			if !ok {
				return fmt.Errorf("missing required query parameter")
			}

			// Find user's voice channel
			channelID, err := getUserVoiceChannel(s, i)
			if err != nil {
				return router.Respond(s, i, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: err.Error(),
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				}, cfg.ResponseDeleteTimeout)
			}

			wasConnected := voiceManager.IsConnected()

			// Join voice channel
			if err := voiceManager.Join(i.GuildID, channelID); err != nil {
				return fmt.Errorf("failed to join voice channel: %w", err)
			}

			// Start streaming
			if err := voiceManager.StartStreaming(cfg.AudioInputDevice); err != nil {
				slog.Error("Failed to start streaming", "error", err)
				// Continue anyway to try playing on Spotify
			}

			activeDevice, err := spotifyClient.GetActiveDevice(ctx)
			if err != nil {
				slog.Error("Failed to get active device", "error", err)
			}
			if activeDevice == nil || activeDevice.Name != cfg.SpotifyDeviceName {
				if err := spotifyClient.TransferPlayback(ctx, cfg.SpotifyDeviceName); err != nil {
					return fmt.Errorf("failed to transfer playback: %w", err)
				}
			}

			// Check if query is a URL or URI
			uriType, id := parseSpotifyID(trackID)

			if uriType == "album" || uriType == "playlist" || uriType == "artist" {
				return playContext(ctx, s, i, spotifyClient, playerService, cfg, uriType, id, trackID)
			}

			// If it's a track from a URL/URI, use the extracted ID
			if uriType == "track" {
				trackID = id
			}

			// Get track details
			track, err := spotifyClient.GetTrack(ctx, trackID)
			trackName := "track"
			if err != nil {
				slog.Error("Failed to get track details", "error", err)
			} else {
				trackName = formatTrackName(track.Name, track.Artists[0].Name)
			}

			// Check if something is already playing
			state, err := spotifyClient.GetPlayerState(ctx)
			if err != nil {
				slog.Error("Failed to get player state", "error", err)
			}

			if state != nil && state.Playing && wasConnected {
				if err := spotifyClient.AddToQueue(ctx, trackID); err != nil {
					return fmt.Errorf("failed to add to queue: %w", err)
				}
				return router.Respond(s, i, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("Added %s to queue.", trackName),
					},
				}, cfg.ResponseDeleteTimeout)
			}

			if err := spotifyClient.Play(ctx, trackID); err != nil {
				if errors.Is(err, spotify.ErrNoActiveDevice) {
					return router.Respond(s, i, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "No active Spotify device found. Please open Spotify on a device and try again.",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					}, cfg.ResponseDeleteTimeout)
				}
				return fmt.Errorf("failed to play track: %w", err)
			}

			playerService.UpdateWidget()

			return router.Respond(s, i, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Playing %s...", trackName),
				},
			}, cfg.ResponseDeleteTimeout)
		},
		AutocompleteHandler: func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
			data := i.ApplicationCommandData()
			query, ok := getStringOption(data, optionQuery)
			var choices []*discordgo.ApplicationCommandOptionChoice
			if !ok || query == "" {
				tracks, err := spotifyClient.GetRecentlyPlayed(ctx)
				if err != nil {
					slog.Error("Error searching Spotify", "error", err)
					return
				}
				if len(tracks) > defaultSearchLimit {
					tracks = tracks[:defaultSearchLimit]
				}
				for _, track := range tracks {
					choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
						Name:  formatTrackNameAutocomplete(track.Track.Name, track.Track.Artists[0].Name),
						Value: track.Track.ID.String(),
					})
				}
			} else {
				tracks, err := spotifyClient.Search(ctx, query, defaultSearchLimit)
				if err != nil {
					slog.Error("Error searching Spotify", "error", err)
					return
				}
				for _, track := range tracks {
					choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
						Name:  formatTrackNameAutocomplete(track.Name, track.Artists[0].Name),
						Value: track.ID.String(),
					})
				}
			}

			if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionApplicationCommandAutocompleteResult,
				Data: &discordgo.InteractionResponseData{
					Choices: choices,
				},
			}); err != nil {
				slog.Error("Error sending autocomplete response", "error", err)
			}
		},
	}
}

func playContext(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, spotifyClient *spotify.Client, playerService *player.Service, cfg *config.Config, uriType, id, originalInput string) error {
	var name string
	switch uriType {
	case "album":
		album, err := spotifyClient.GetAlbum(ctx, id)
		if err == nil {
			name = fmt.Sprintf("album **%s**", album.Name)
		} else {
			name = "album"
		}
	case "artist":
		artist, err := spotifyClient.GetArtist(ctx, id)
		if err == nil {
			name = fmt.Sprintf("artist **%s**", artist.Name)
		} else {
			name = "artist"
		}
	case "playlist":
		playlist, err := spotifyClient.GetPlaylist(ctx, id)
		if err == nil {
			name = fmt.Sprintf("playlist **%s**", playlist.Name)
		} else {
			name = "playlist"
		}
	default:
		slog.Error("Invalid uri type", "uriType", uriType)
		name = "unknown"
	}

	if err := spotifyClient.PlayContext(ctx, originalInput); err != nil {
		if errors.Is(err, spotify.ErrNoActiveDevice) {
			return router.Respond(s, i, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "No active Spotify device found. Please open Spotify on a device and try again.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			}, cfg.ResponseDeleteTimeout)
		}
		return fmt.Errorf("failed to play %s: %w", uriType, err)
	}

	playerService.UpdateWidget()

	return router.Respond(s, i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Playing %s...", name),
		},
	}, cfg.ResponseDeleteTimeout)
}
