// Package main is the entry point for the Discord bot application.
package main

import (
	"log/slog"
	"os"

	"github.com/cybre/discordbotv3/internal/bot"
	"github.com/cybre/discordbotv3/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Error loading configuration", "error", err)
		os.Exit(1)
	}

	b, err := bot.New(cfg)
	if err != nil {
		slog.Error("Error initializing bot", "error", err)
		os.Exit(1)
	}

	if err := b.Run(); err != nil {
		slog.Error("Error running bot", "error", err)
		os.Exit(1)
	}
}
