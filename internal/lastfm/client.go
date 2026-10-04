package lastfm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	baseURL      = "https://ws.audioscrobbler.com/2.0/"
	defaultLimit = 200

	maxRetries     = 3                      // retry 3x jika gagal
	retryBaseDelay = 500 * time.Millisecond // delay percobaan pertama
)

type Client struct {
	apiKey     string
	username   string
	httpClient *http.Client
}

func NewClient(apiKey, username string) *Client {
	return &Client{
		apiKey:   apiKey,
		username: username,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// GetRecentTracks mengambil satu halaman scrobble history milik user.
func (c *Client) GetRecentTracks(ctx context.Context, from, to time.Time, page int) (*RecentTracksResponse, error) {
	params := url.Values{}
	params.Set("method", "user.getrecenttracks")
	params.Set("user", c.username)
	params.Set("limit", strconv.Itoa(defaultLimit))
	params.Set("page", strconv.Itoa(page))

	if !from.IsZero() {
		params.Set("from", strconv.FormatInt(from.Unix(), 10))
	}
	if !to.IsZero() {
		params.Set("to", strconv.FormatInt(to.Unix(), 10))
	}

	var resp RecentTracksResponse
	if err := c.getWithRetry(ctx, params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetTrackInfo mengambil metadata lagu dari endpoint track.getInfo.
func (c *Client) GetTrackInfo(ctx context.Context, artist, track string) (*TrackInfoResponse, error) {
	params := url.Values{}
	params.Set("method", "track.getInfo")
	params.Set("artist", artist)
	params.Set("track", track)
	params.Set("autocorrect", "1")

	var resp TrackInfoResponse
	if err := c.getWithRetry(ctx, params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetArtistInfo mengambil metadata artis dari endpoint artist.getInfo.
func (c *Client) GetArtistInfo(ctx context.Context, artist string) (*ArtistInfoResponse, error) {
	params := url.Values{}
	params.Set("method", "artist.getInfo")
	params.Set("artist", artist)
	params.Set("autocorrect", "1")

	var resp ArtistInfoResponse
	if err := c.getWithRetry(ctx, params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetAlbumInfo mengambil metadata album dari endpoint album.getInfo.
func (c *Client) GetAlbumInfo(ctx context.Context, artist, album string) (*AlbumInfoResponse, error) {
	params := url.Values{}
	params.Set("method", "album.getInfo")
	params.Set("artist", artist)
	params.Set("album", album)
	params.Set("autocorrect", "1")

	var resp AlbumInfoResponse
	if err := c.getWithRetry(ctx, params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// getWithRetry membungkus eksekusi HTTP GET dengan exponential backoff.
func (c *Client) getWithRetry(ctx context.Context, params url.Values, target any) error {
	params.Set("api_key", c.apiKey)
	params.Set("format", "json")

	reqURL := baseURL + "?" + params.Encode()

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			if err := sleepWithBackoff(ctx, attempt-1); err != nil {
				return fmt.Errorf("retry dibatalkan: %w", err)
			}
		}

		retryable, err := c.doGet(ctx, reqURL, target)
		if err == nil {
			return nil
		}

		lastErr = err
		if !retryable {
			return err
		}
	}

	return fmt.Errorf("gagal setelah %d percobaan: %w", maxRetries+1, lastErr)
}

func (c *Client) doGet(ctx context.Context, reqURL string, target any) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return false, fmt.Errorf("gagal membuat request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return true, fmt.Errorf("gagal memanggil Last.fm API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return true, fmt.Errorf("gagal membaca response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		retryable := isRetryableStatus(resp.StatusCode)

		var apiErr errorResponse
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Message != "" {
			return retryable, fmt.Errorf("last.fm API error (%d): %s", apiErr.Error, apiErr.Message)
		}
		return retryable, fmt.Errorf("last.fm API mengembalikan status %d", resp.StatusCode)
	}

	if err := json.Unmarshal(body, target); err != nil {
		return false, fmt.Errorf("gagal parse response JSON: %w", err)
	}

	return false, nil
}

func isRetryableStatus(statusCode int) bool {
	if statusCode == http.StatusTooManyRequests {
		return true
	}
	return statusCode >= 500 && statusCode < 600
}

func sleepWithBackoff(ctx context.Context, attempt int) error {
	delay := retryBaseDelay * time.Duration(1<<attempt)

	select {
	case <-time.After(delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
