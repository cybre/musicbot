// Package bot provides the core Discord bot implementation.
// It orchestrates the Discord session, command routing, Spotify integration,
// and voice connection management.
package bot

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/cybre/discordbotv3/internal/commands"
	"github.com/cybre/discordbotv3/internal/config"
	"github.com/cybre/discordbotv3/internal/player"
	"github.com/cybre/discordbotv3/internal/router"
	"github.com/cybre/discordbotv3/internal/spotify"
	"github.com/cybre/discordbotv3/internal/voice"
)

const (
	// shutdownTimeout is the maximum time to wait for graceful shutdown operations.
	shutdownTimeout = 3 * time.Second
)

// Bot represents the Discord bot.
type Bot struct {
	Session           *discordgo.Session
	Router            *router.Router
	spotifyClient     *spotify.Client
	inactivityMonitor *voice.Monitor
	voiceManager      *voice.Manager
	playerService     *player.Service
}

// New creates a new instance of the Bot.
func New(cfg *config.Config) (*Bot, error) {
	dg, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("error creating Discord session: %w", err)
	}

	voiceManager := voice.New(dg)

	spotifyClient, err := spotify.New(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating Spotify client: %w", err)
	}

	if err := spotifyClient.TransferPlayback(context.Background(), cfg.SpotifyDeviceName); err != nil {
		return nil, fmt.Errorf("error transferring playback: %w", err)
	}

	voiceManager.OnLeave(func() {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := spotifyClient.Pause(ctx); err != nil {
			slog.Error("Failed to pause Spotify on leave", "error", err)
		}
	})

	playerService := player.New(spotifyClient, dg)
	voiceManager.OnJoin(playerService.Start)
	voiceManager.OnLeave(playerService.Stop)

	inactivityMonitor := voice.NewMonitor(spotifyClient, voiceManager, cfg.InactivityTimeout)
	voiceManager.OnJoin(inactivityMonitor.Start)
	voiceManager.OnLeave(inactivityMonitor.Stop)

	r := router.New()
	r.Register(commands.PingCommand(cfg))
	r.Register(commands.PlayCommand(spotifyClient, voiceManager, cfg, playerService))
	r.Register(commands.PauseCommand(spotifyClient, playerService, cfg))
	r.Register(commands.StopCommand(voiceManager, cfg))
	r.Register(commands.ResumeCommand(spotifyClient, playerService, cfg))
	r.Register(commands.SeekCommand(spotifyClient, playerService, cfg))
	r.Register(commands.NextCommand(spotifyClient, playerService, cfg))
	r.Register(commands.PreviousCommand(spotifyClient, playerService, cfg))
	r.Register(commands.NowPlayingCommand(playerService, cfg))
	r.Register(commands.QueueCommand(spotifyClient, cfg, r))
	r.Register(commands.VolumeCommand(spotifyClient, cfg))
	playerService.RegisterHandlers(r, voiceManager)

	return &Bot{
		Session:           dg,
		Router:            r,
		spotifyClient:     spotifyClient,
		inactivityMonitor: inactivityMonitor,
		voiceManager:      voiceManager,
		playerService:     playerService,
	}, nil
}

// Run starts the bot and waits for a termination signal.
func (b *Bot) Run() error {
	b.Session.AddHandler(b.Router.Handle)
	b.Session.AddHandler(b.ready)

	b.Session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsGuilds | discordgo.IntentsGuildVoiceStates
	b.Session.StateEnabled = true

	if err := b.Session.Open(); err != nil {
		return fmt.Errorf("error opening connection: %w", err)
	}
	defer func() {
		if err := b.Session.Close(); err != nil {
			slog.Error("Failed to close Discord session", "error", err)
		}
	}()

	if err := b.Router.Sync(b.Session, ""); err != nil {
		slog.Error("Error syncing commands", "error", err)
	}

	slog.Info("Bot is now running. Press CTRL-C to exit.")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	if err := b.voiceManager.Leave(); err != nil {
		slog.Error("Failed to leave voice channel", "error", err)
	}

	return nil
}

func (b *Bot) ready(s *discordgo.Session, event *discordgo.Ready) {
	slog.Info("Logged in", "user", s.State.User.Username, "discriminator", s.State.User.Discriminator)
}
