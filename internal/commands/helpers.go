package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// getStringOption extracts a string option value from command data.
// Returns the value and true if found, empty string and false otherwise.
func getStringOption(data discordgo.ApplicationCommandInteractionData, name string) (string, bool) {
	for _, opt := range data.Options {
		if opt.Name == name {
			return opt.StringValue(), true
		}
	}
	return "", false
}

// getUserVoiceChannel gets the voice channel ID for the user who triggered the interaction.
// Returns an error if the interaction is not in a guild or the user is not in a voice channel.
func getUserVoiceChannel(s *discordgo.Session, i *discordgo.InteractionCreate) (string, error) {
	if i.GuildID == "" {
		return "", fmt.Errorf("command can only be used in a server")
	}

	vs, err := s.State.VoiceState(i.GuildID, i.Member.User.ID)
	if err != nil {
		return "", fmt.Errorf("you must be in a voice channel to use this command")
	}

	return vs.ChannelID, nil
}

// formatTrackName formats a track name with artist for display.
func formatTrackName(trackName, artistName string) string {
	return fmt.Sprintf("**%s** by **%s**", trackName, artistName)
}
