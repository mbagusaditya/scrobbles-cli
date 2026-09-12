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

	maxRetries     = 4                      // total percobaan = maxRetries + 1
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

// GetRecentTracks mengambil satu halaman scrobble history milik user
// dalam rentang [from, to]. Otomatis retry dengan exponential backoff
// kalau request gagal karena network error, rate limit (429), atau
// server error (5xx).
func (c *Client) GetRecentTracks(ctx context.Context, from, to time.Time, page int) (*RecentTracksResponse, error) {
	reqURL := c.buildRecentTracksURL(from, to, page)

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			if err := sleepWithBackoff(ctx, attempt-1); err != nil {
				return nil, fmt.Errorf("retry dibatalkan: %w", err)
			}
		}

		result, retryable, err := c.doRequest(ctx, reqURL)
		if err == nil {
			return result, nil
		}

		lastErr = err
		if !retryable {
			return nil, err
		}
	}

	return nil, fmt.Errorf("gagal setelah %d percobaan: %w", maxRetries+1, lastErr)
}

func (c *Client) buildRecentTracksURL(from, to time.Time, page int) string {
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
	if !to.IsZero() {
		params.Set("to", strconv.FormatInt(to.Unix(), 10))
	}

	return baseURL + "?" + params.Encode()
}

func (c *Client) doRequest(ctx context.Context, reqURL string) (*RecentTracksResponse, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, false, fmt.Errorf("gagal membuat request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, true, fmt.Errorf("gagal memanggil Last.fm API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, fmt.Errorf("gagal membaca response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		retryable := isRetryableStatus(resp.StatusCode)

		var apiErr errorResponse
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Message != "" {
			return nil, retryable, fmt.Errorf("last.fm API error (%d): %s", apiErr.Error, apiErr.Message)
		}
		return nil, retryable, fmt.Errorf("last.fm API mengembalikan status %d", resp.StatusCode)
	}

	var result RecentTracksResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, false, fmt.Errorf("gagal parse response JSON: %w", err)
	}

	return &result, false, nil
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
