// Package router provides command routing and handling for Discord interactions.
// It manages slash command registration, execution, and component interactions.
package router

import (
	"context"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	// defaultHandlerTimeout is the maximum time allowed for command and component handlers.
	defaultHandlerTimeout = 10 * time.Second
)

// Command represents a Discord slash command.
type Command struct {
	Name                string
	Description         string
	Options             []*discordgo.ApplicationCommandOption
	Handler             func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error
	AutocompleteHandler func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate)
}

// Router handles command registration and execution.
type Router struct {
	commands          map[string]Command
	componentHandlers map[string]func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error
}

// New creates a new Router.
func New() *Router {
	return &Router{
		commands:          make(map[string]Command),
		componentHandlers: make(map[string]func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error),
	}
}

// Register registers a command with the router.
func (r *Router) Register(cmd Command) {
	r.commands[cmd.Name] = cmd
}

// RegisterComponent registers a component handler.
func (r *Router) RegisterComponent(customID string, handler func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	r.componentHandlers[customID] = handler
}

// Handle routes the interaction to the appropriate command handler.
func (r *Router) Handle(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultHandlerTimeout)
	defer cancel()

	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		cmd, ok := r.commands[i.ApplicationCommandData().Name]
		if !ok {
			return
		}

		if err := cmd.Handler(ctx, s, i); err != nil {
			slog.Error("Command execution failed", "command", cmd.Name, "error", err)

			if respErr := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "An error occurred while executing the command.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			}); respErr != nil {
				slog.Error("Failed to send error response", "error", respErr)
			}
		}

	case discordgo.InteractionApplicationCommandAutocomplete:
		cmd, ok := r.commands[i.ApplicationCommandData().Name]
		if ok && cmd.AutocompleteHandler != nil {
			cmd.AutocompleteHandler(ctx, s, i)
		}

	case discordgo.InteractionMessageComponent:
		handler, ok := r.componentHandlers[i.MessageComponentData().CustomID]
		if !ok {
			return
		}

		if err := handler(ctx, s, i); err != nil {
			slog.Error("Component execution failed", "customID", i.MessageComponentData().CustomID, "error", err)
		}
	}
}

// Sync registers the commands with Discord.
func (r *Router) Sync(s *discordgo.Session, guildID string) error {
	for _, cmd := range r.commands {
		if _, err := s.ApplicationCommandCreate(s.State.User.ID, guildID, &discordgo.ApplicationCommand{
			Name:        cmd.Name,
			Description: cmd.Description,
			Options:     cmd.Options,
		}); err != nil {
			slog.Error("Cannot create command", "command", cmd.Name, "error", err)
			return err
		}
		slog.Info("Command registered", "command", cmd.Name)
	}
	return nil
}

// Respond sends an interaction response and automatically deletes it after the specified timeout.
// If the response is ephemeral, it is not deleted.
func Respond(s *discordgo.Session, i *discordgo.InteractionCreate, resp *discordgo.InteractionResponse, timeout time.Duration) error {
	if err := s.InteractionRespond(i.Interaction, resp); err != nil {
		return err
	}

	if timeout > 0 {
		time.AfterFunc(timeout, func() {
			if err := s.InteractionResponseDelete(i.Interaction); err != nil {
				slog.Error("Failed to delete interaction response", "error", err)
			}
		})
	}

	return nil
}
