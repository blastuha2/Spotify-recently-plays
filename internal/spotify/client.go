package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.spotify.com/v1/me/player/recently-played"
	maxBodySize    = 10 << 20
	max5xxRetries  = 3
)

type Client struct {
	HTTPClient *http.Client
	Token      string
	BaseURL    string
}

type Progress struct {
	Request  int
	Received int
	Total    int
	Oldest   time.Time
}

type Result struct {
	Events        []Event
	APIRequests   int
	StopReason    string
	ReachedCutoff bool
}

func NewClient(token string) *Client {
	return &Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Token:      token,
		BaseURL:    defaultBaseURL,
	}
}

// FetchHistory follows Spotify-provided next links until the cutoff or the end.
func (c *Client) FetchHistory(ctx context.Context, from time.Time, report func(Progress)) (Result, error) {
	if strings.TrimSpace(c.Token) == "" {
		return Result{}, errors.New("SPOTIFY_TOKEN is not set")
	}
	if c.HTTPClient == nil {
		return Result{}, errors.New("Spotify HTTP client is nil")
	}

	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	first, err := url.Parse(baseURL)
	if err != nil {
		return Result{}, fmt.Errorf("invalid Spotify API URL: %w", err)
	}
	query := first.Query()
	query.Set("limit", "50")
	first.RawQuery = query.Encode()
	nextURL := first.String()

	result := Result{Events: make([]Event, 0, 256)}
	seenEvents := make(map[string]struct{})
	seenPages := make(map[string]struct{})

	for {
		if _, exists := seenPages[nextURL]; exists {
			result.StopReason = "Spotify returned a repeated next page"
			break
		}
		seenPages[nextURL] = struct{}{}

		p, err := c.getPage(ctx, nextURL)
		if err != nil {
			return result, err
		}
		result.APIRequests++
		if len(p.Items) == 0 {
			result.StopReason = "Spotify returned empty items"
			break
		}

		oldest := p.Items[0].PlayedAt
		cutoffReached := false
		for _, item := range p.Items {
			if item.PlayedAt.Before(oldest) {
				oldest = item.PlayedAt
			}
			if item.PlayedAt.Before(from) {
				cutoffReached = true
				continue
			}
			if item.PlayedAt.Equal(from) {
				cutoffReached = true
			}
			event := item.event()
			key := eventKey(event)
			if _, exists := seenEvents[key]; exists {
				continue
			}
			seenEvents[key] = struct{}{}
			result.Events = append(result.Events, event)
		}

		if report != nil {
			report(Progress{
				Request:  result.APIRequests,
				Received: len(p.Items),
				Total:    len(result.Events),
				Oldest:   oldest,
			})
		}
		if cutoffReached {
			result.ReachedCutoff = true
			result.StopReason = "requested start date reached"
			break
		}
		if p.Next == nil || strings.TrimSpace(*p.Next) == "" {
			result.StopReason = "Spotify returned no next page"
			break
		}
		nextURL, err = nextPageURL(first, *p.Next)
		if err != nil {
			return result, err
		}
	}

	return result, nil
}

func nextPageURL(apiURL *url.URL, value string) (string, error) {
	next, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid Spotify next page URL: %w", err)
	}
	next = apiURL.ResolveReference(next)
	if !strings.EqualFold(next.Scheme, apiURL.Scheme) || !strings.EqualFold(next.Host, apiURL.Host) {
		return "", errors.New("Spotify next page points to an unexpected host")
	}
	query := next.Query()
	query.Set("limit", "50")
	next.RawQuery = query.Encode()
	return next.String(), nil
}

func (c *Client) getPage(ctx context.Context, pageURL string) (page, error) {
	serverFailures := 0
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
		if err != nil {
			return page{}, fmt.Errorf("create Spotify request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.Token)
		req.Header.Set("Accept", "application/json")

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return page{}, fmt.Errorf("Spotify request failed: %w", err)
		}
		body, readErr := readBody(resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil {
			return page{}, fmt.Errorf("read Spotify response: %w", readErr)
		}
		if closeErr != nil {
			return page{}, fmt.Errorf("close Spotify response: %w", closeErr)
		}

		switch {
		case resp.StatusCode == http.StatusOK:
			var p page
			if err := json.Unmarshal(body, &p); err != nil {
				return page{}, fmt.Errorf("decode Spotify response: %w", err)
			}
			return p, nil

		case resp.StatusCode == http.StatusUnauthorized:
			return page{}, errors.New("Spotify access token is invalid or expired; get a new token and set SPOTIFY_TOKEN")

		case resp.StatusCode == http.StatusTooManyRequests:
			delay := retryAfter(resp.Header.Get("Retry-After"), time.Now())
			if err := sleepContext(ctx, delay); err != nil {
				return page{}, err
			}
			continue

		case resp.StatusCode >= 500 && resp.StatusCode <= 599:
			if serverFailures >= max5xxRetries {
				return page{}, fmt.Errorf("Spotify server error %s after %d retries", resp.Status, max5xxRetries)
			}
			delay := time.Second << serverFailures
			serverFailures++
			if err := sleepContext(ctx, delay); err != nil {
				return page{}, err
			}
			continue

		default:
			message := spotifyErrorMessage(body)
			if message == "" {
				message = resp.Status
			}
			return page{}, fmt.Errorf("Spotify API error %d: %s", resp.StatusCode, message)
		}
	}
}

func readBody(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxBodySize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBodySize {
		return nil, fmt.Errorf("response exceeds %d bytes", maxBodySize)
	}
	return data, nil
}

func spotifyErrorMessage(body []byte) string {
	var response struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &response) == nil {
		return response.Error.Message
	}
	return ""
}

func retryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil && when.After(now) {
		return when.Sub(now)
	}
	return 2 * time.Second
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
