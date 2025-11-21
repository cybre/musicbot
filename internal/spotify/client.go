// Package spotify provides a client for interacting with the Spotify Web API.
// It handles OAuth authentication, token persistence, and playback control.
package spotify

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/cybre/discordbotv3/internal/config"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

const (
	// tokenFile is the filename where OAuth tokens are persisted.
	tokenFile = "spotify_token.json"

	// stateLength is the number of random bytes used for OAuth state generation.
	stateLength = 32
)

// RepeatState represents the repeat mode for playback.
type RepeatState string

const (
	RepeatOff     RepeatState = "off"
	RepeatContext RepeatState = "context"
	RepeatTrack   RepeatState = "track"
)

// Client wraps the Spotify API client.
type Client struct {
	client *spotify.Client
}

// persistentTokenSource wraps an oauth2.TokenSource to persist token updates to disk.
type persistentTokenSource struct {
	src       oauth2.TokenSource
	lastToken *oauth2.Token
}

// New creates a new Spotify client using Authorization Code Flow.
func New(ctx context.Context, cfg *config.Config) (*Client, error) {
	oauthConf := &oauth2.Config{
		ClientID:     cfg.SpotifyClientID,
		ClientSecret: cfg.SpotifyClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.spotify.com/authorize",
			TokenURL: "https://accounts.spotify.com/api/token",
		},
		RedirectURL: cfg.SpotifyRedirectURL,
		Scopes: []string{
			spotifyauth.ScopeUserReadPlaybackState,
			spotifyauth.ScopeUserModifyPlaybackState,
			spotifyauth.ScopeUserReadRecentlyPlayed,
			spotifyauth.ScopeStreaming,
		},
	}

	token, err := loadToken()
	if err != nil {
		state, err := generateRandomState()
		if err != nil {
			return nil, fmt.Errorf("failed to generate state string: %w", err)
		}

		codeChan := make(chan string)

		// Start a local HTTP server to handle the callback
		server := &http.Server{Addr: ":" + cfg.SpotifyCallbackPort}
		http.HandleFunc("/spotify/callback", func(w http.ResponseWriter, r *http.Request) {
			code := r.URL.Query().Get("code")
			if code == "" {
				http.Error(w, "No code found", http.StatusBadRequest)
				return
			}
			if r.URL.Query().Get("state") != state {
				http.Error(w, "State mismatch", http.StatusBadRequest)
				return
			}
			codeChan <- code
			if _, err := fmt.Fprintf(w, "Login successful! You can close this window."); err != nil {
				slog.Error("Failed to write HTTP response", "error", err)
			}
		})

		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("HTTP server error", "error", err)
			}
		}()

		url := oauthConf.AuthCodeURL(state)
		fmt.Printf("Please log in to Spotify by visiting the following page in your browser: \n%s\n", url)

		// Wait for the code
		code := <-codeChan

		if err := server.Shutdown(ctx); err != nil {
			slog.Error("Error shutting down server", "error", err)
		}

		token, err = oauthConf.Exchange(ctx, code)
		if err != nil {
			return nil, fmt.Errorf("error exchanging code for token: %w", err)
		}
		saveToken(token)
	}

	// Wrap the token source to persist updates
	tokenSource := oauthConf.TokenSource(ctx, token)
	reuseTokenSource := &persistentTokenSource{
		src:       tokenSource,
		lastToken: token,
	}

	httpClient := oauth2.NewClient(ctx, reuseTokenSource)
	client := spotify.New(httpClient)

	return &Client{
		client: client,
	}, nil
}

func (c *Client) TransferPlayback(ctx context.Context, deviceName string) error {
	devices, err := c.client.PlayerDevices(ctx)
	if err != nil {
		return fmt.Errorf("error getting devices: %w", err)
	}

	for _, device := range devices {
		slog.Info("Found device", "name", device.Name, "id", device.ID)
		if device.Name == deviceName {
			slog.Info("Transferring playback to device", "name", device.Name, "id", device.ID)
			if err := c.client.TransferPlayback(ctx, device.ID, false); err != nil {
				return err
			}
			if err := c.client.Pause(ctx); err != nil {
				slog.Error("failed to pause playback after transfering device", "error", err)
			}
		}
	}

	return fmt.Errorf("device %s not found", deviceName)
}

// Search searches for tracks on Spotify.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]spotify.FullTrack, error) {
	results, err := c.client.Search(ctx, query, spotify.SearchTypeTrack, spotify.Limit(limit))
	if err != nil {
		return nil, fmt.Errorf("error searching Spotify: %w", err)
	}

	if results.Tracks == nil {
		return []spotify.FullTrack{}, nil
	}

	return results.Tracks.Tracks, nil
}

// GetRecentlyPlayed returns the user's recently played tracks.
func (c *Client) GetRecentlyPlayed(ctx context.Context) ([]spotify.RecentlyPlayedItem, error) {
	return c.client.PlayerRecentlyPlayed(ctx)
}

// Play plays a track on the user's active device.
func (c *Client) Play(ctx context.Context, trackID string) error {
	err := c.client.PlayOpt(ctx, &spotify.PlayOptions{
		URIs: []spotify.URI{spotify.URI("spotify:track:" + trackID)},
	})
	if err != nil && strings.Contains(err.Error(), "No active device") {
		return ErrNoActiveDevice
	}
	return err
}

// PlayContext plays a context (album, artist, playlist) on the user's active device.
func (c *Client) PlayContext(ctx context.Context, uri string) error {
	u := spotify.URI(uri)
	err := c.client.PlayOpt(ctx, &spotify.PlayOptions{
		PlaybackContext: &u,
	})
	if err != nil && strings.Contains(err.Error(), "No active device") {
		return ErrNoActiveDevice
	}
	return err
}

// Pause pauses playback on the user's active device.
func (c *Client) Pause(ctx context.Context) error {
	return c.client.Pause(ctx)
}

// Resume resumes playback on the user's active device.
func (c *Client) Resume(ctx context.Context) error {
	return c.client.Play(ctx)
}

// Seek seeks to the given position in the track.
func (c *Client) Seek(ctx context.Context, position int) error {
	return c.client.Seek(ctx, position)
}

// GetPlayerState gets the current player state.
func (c *Client) GetPlayerState(ctx context.Context) (*spotify.PlayerState, error) {
	return c.client.PlayerState(ctx)
}

// AddToQueue adds a track to the queue.
func (c *Client) AddToQueue(ctx context.Context, trackID string) error {
	return c.client.QueueSong(ctx, spotify.ID(trackID))
}

// Next skips to the next track.
func (c *Client) Next(ctx context.Context) error {
	return c.client.Next(ctx)
}

// Previous skips to the previous track.
func (c *Client) Previous(ctx context.Context) error {
	return c.client.Previous(ctx)
}

// GetTrack gets a track by ID.
func (c *Client) GetTrack(ctx context.Context, trackID string) (*spotify.FullTrack, error) {
	return c.client.GetTrack(ctx, spotify.ID(trackID))
}

// GetArtist gets an artist by ID.
func (c *Client) GetArtist(ctx context.Context, artistID string) (*spotify.FullArtist, error) {
	return c.client.GetArtist(ctx, spotify.ID(artistID))
}

// GetAlbum gets an album by ID.
func (c *Client) GetAlbum(ctx context.Context, albumID string) (*spotify.FullAlbum, error) {
	return c.client.GetAlbum(ctx, spotify.ID(albumID))
}

// GetPlaylist gets a playlist by ID.
func (c *Client) GetPlaylist(ctx context.Context, playlistID string) (*spotify.FullPlaylist, error) {
	return c.client.GetPlaylist(ctx, spotify.ID(playlistID))
}

// GetQueue gets the user's current playback queue.
func (c *Client) GetQueue(ctx context.Context) (*spotify.Queue, error) {
	return c.client.GetQueue(ctx)
}

// Shuffle toggles shuffle mode.
func (c *Client) Shuffle(ctx context.Context, state bool) error {
	return c.client.Shuffle(ctx, state)
}

// Repeat sets the repeat mode.
func (c *Client) Repeat(ctx context.Context, state RepeatState) error {
	return c.client.Repeat(ctx, string(state))
}

// SetVolume sets the volume for the user's active device.
func (c *Client) SetVolume(ctx context.Context, volume int) error {
	return c.client.Volume(ctx, volume)
}

// GetVolume gets the current volume for the user's active device.
func (c *Client) GetVolume(ctx context.Context) (int, error) {
	state, err := c.client.PlayerState(ctx)
	if err != nil {
		return 0, err
	}
	if state == nil || state.Device.ID == "" {
		return 0, ErrNoActiveDevice
	}
	return int(state.Device.Volume), nil
}

// Token implements oauth2.TokenSource and persists token updates.
func (s *persistentTokenSource) Token() (*oauth2.Token, error) {
	t, err := s.src.Token()
	if err != nil {
		return nil, err
	}
	if s.lastToken == nil || t.AccessToken != s.lastToken.AccessToken {
		saveToken(t)
		s.lastToken = t
	}
	return t, nil
}

func saveToken(t *oauth2.Token) {
	f, err := os.Create(tokenFile)
	if err != nil {
		slog.Error("failed to create token file", "error", err)
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			slog.Error("Failed to close token file", "error", err)
		}
	}()
	if err := json.NewEncoder(f).Encode(t); err != nil {
		slog.Error("failed to encode token", "error", err)
	}
}

func loadToken() (*oauth2.Token, error) {
	f, err := os.Open(tokenFile)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := f.Close(); err != nil {
			slog.Error("Failed to close token file", "error", err)
		}
	}()
	var t oauth2.Token
	if err := json.NewDecoder(f).Decode(&t); err != nil {
		return nil, err
	}
	return &t, nil
}

func generateRandomState() (string, error) {
	b := make([]byte, stateLength)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
