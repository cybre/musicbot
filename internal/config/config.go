// Package config provides application configuration management.
// Configuration is loaded from environment variables with support for .env files.
package config

import (
	"fmt"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// Config holds the application configuration.
type Config struct {
	// Token is the Discord bot authentication token.
	Token string `envconfig:"DISCORD_TOKEN" required:"true"`

	// SpotifyClientID is the Spotify application client ID for OAuth.
	SpotifyClientID string `envconfig:"SPOTIFY_CLIENT_ID" required:"true"`

	// SpotifyClientSecret is the Spotify application client secret for OAuth.
	SpotifyClientSecret string `envconfig:"SPOTIFY_CLIENT_SECRET" required:"true"`

	// SpotifyRedirectURL is the OAuth callback URL for Spotify authentication.
	SpotifyRedirectURL string `envconfig:"SPOTIFY_REDIRECT_URL" default:"https://localhost:6000/spotify/callback"`

	// SpotifyCallbackPort is the local port for the OAuth callback server.
	SpotifyCallbackPort string `envconfig:"SPOTIFY_CALLBACK_PORT" default:"6000"`

	// AudioInputDevice is the name of the audio input device for voice streaming.
	AudioInputDevice string `envconfig:"AUDIO_INPUT_DEVICE" required:"true"`

	// ResponseDeleteTimeout is the duration after which ephemeral bot responses are deleted.
	ResponseDeleteTimeout time.Duration `envconfig:"RESPONSE_DELETE_TIMEOUT" default:"2m"`
}

// Load loads the configuration from environment variables.
func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("error processing config: %w", err)
	}

	return &cfg, nil
}
