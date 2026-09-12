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
	defaultLimit = 200 // maksimum limit per page yang diizinkan Last.fm API
)

// Client adalah HTTP client untuk berkomunikasi dengan Last.fm API.
type Client struct {
	apiKey     string
	username   string
	httpClient *http.Client
}

// NewClient membuat instance Client baru.
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
//
// from: kalau non-zero, hanya ambil scrobble SETELAH timestamp ini
// (dipakai untuk incremental sync). Kirim time.Time{} (zero value)
// untuk mengambil dari awal (full backfill).
//
// page: nomor halaman, dimulai dari 1.
func (c *Client) GetRecentTracks(ctx context.Context, from time.Time, page int) (*RecentTracksResponse, error) {
	params := url.Values{}
	params.Set("method", "user.getrecenttracks")
	params.Set("user", c.username)
	params.Set("api_key", c.apiKey)
	params.Set("format", "json")
	params.Set("limit", strconv.Itoa(defaultLimit))
	params.Set("page", strconv.Itoa(page))

	if !from.IsZero() {
		params.Set("from", strconv.FormatInt(from.Unix(), 10))
	}

	reqURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal memanggil Last.fm API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr errorResponse
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Message != "" {
			return nil, fmt.Errorf("last.fm API error (%d): %s", apiErr.Error, apiErr.Message)
		}
		return nil, fmt.Errorf("last.fm API mengembalikan status %d", resp.StatusCode)
	}

	var result RecentTracksResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("gagal parse response JSON: %w", err)
	}

	return &result, nil
}
