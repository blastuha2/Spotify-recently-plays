package partner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultEndpoint  = "https://api-partner.spotify.com/pathfinder/v2/query"
	defaultQueryHash = "698be5892a3cc95331deebeff463d05dfdd5febf5254bea30b895b5a93dfb584"
	pageLimit        = 100
	maxResponseSize  = 20 << 20
	max5xxRetries    = 3
)

type Client struct {
	HTTPClient  *http.Client
	AccessToken string
	ClientToken string
	AppVersion  string
	QueryHash   string
	Endpoint    string
}

type Progress struct {
	Request    int
	Offset     int
	Received   int
	Tracks     int
	TotalItems int
	Oldest     time.Time
}

type Result struct {
	Tracks        []Track
	APIRequests   int
	TotalItems    int
	SkippedItems  int
	StopReason    string
	ReachedCutoff bool
}

type queryRequest struct {
	Variables struct {
		URIs   []string `json:"uris"`
		Offset int      `json:"offset"`
		Limit  int      `json:"limit"`
	} `json:"variables"`
	OperationName string `json:"operationName"`
	Extensions    struct {
		PersistedQuery struct {
			Version    int    `json:"version"`
			SHA256Hash string `json:"sha256Hash"`
		} `json:"persistedQuery"`
	} `json:"extensions"`
}

func NewClient(accessToken, clientToken string) *Client {
	return &Client{
		HTTPClient:  &http.Client{Timeout: 30 * time.Second},
		AccessToken: accessToken,
		ClientToken: clientToken,
		QueryHash:   defaultQueryHash,
		Endpoint:    defaultEndpoint,
	}
}

// FetchRecents walks the internal Web Player recents list from newest to oldest.
func (c *Client) FetchRecents(ctx context.Context, from time.Time, report func(Progress)) (Result, error) {
	if strings.TrimSpace(c.AccessToken) == "" {
		return Result{}, errors.New("SPOTIFY_PARTNER_TOKEN is not set")
	}
	if strings.TrimSpace(c.ClientToken) == "" {
		return Result{}, errors.New("SPOTIFY_CLIENT_TOKEN is not set")
	}
	if c.HTTPClient == nil {
		return Result{}, errors.New("Spotify partner HTTP client is nil")
	}

	result := Result{Tracks: make([]Track, 0, 1024)}
	offset := 0
	seenOffsets := make(map[int]struct{})
	for {
		if _, exists := seenOffsets[offset]; exists {
			result.StopReason = "Spotify returned a repeated offset"
			break
		}
		seenOffsets[offset] = struct{}{}

		page, err := c.getPage(ctx, offset)
		if err != nil {
			return result, err
		}
		result.APIRequests++
		result.TotalItems = page.TotalCount
		if len(page.Items) == 0 {
			result.StopReason = "Spotify returned empty items"
			break
		}

		oldest := time.Time{}
		cutoffReached := false
		for index, item := range page.Items {
			date, dateErr := item.AddedAt.time()
			if dateErr != nil {
				return result, dateErr
			}
			if oldest.IsZero() || date.Before(oldest) {
				oldest = date
			}
			if date.Before(from) {
				cutoffReached = true
				continue
			}
			track, isTrack, trackErr := item.track(offset + index)
			if trackErr != nil {
				return result, trackErr
			}
			if !isTrack {
				result.SkippedItems++
				continue
			}
			result.Tracks = append(result.Tracks, track)
		}

		if report != nil {
			report(Progress{
				Request:    result.APIRequests,
				Offset:     offset,
				Received:   len(page.Items),
				Tracks:     len(result.Tracks),
				TotalItems: page.TotalCount,
				Oldest:     oldest,
			})
		}
		if cutoffReached {
			result.ReachedCutoff = true
			result.StopReason = "requested start date reached"
			break
		}

		nextOffset := page.PagingInfo.NextOffset
		if nextOffset <= offset {
			nextOffset = offset + len(page.Items)
		}
		if page.TotalCount > 0 && nextOffset >= page.TotalCount {
			result.StopReason = "all available recents fetched"
			break
		}
		if nextOffset <= offset {
			result.StopReason = "Spotify returned no next offset"
			break
		}
		offset = nextOffset
	}
	return result, nil
}

func (c *Client) getPage(ctx context.Context, offset int) (page, error) {
	payload := queryRequest{OperationName: "recents"}
	payload.Variables.URIs = []string{"spotify:list:recents:page"}
	payload.Variables.Offset = offset
	payload.Variables.Limit = pageLimit
	payload.Extensions.PersistedQuery.Version = 1
	payload.Extensions.PersistedQuery.SHA256Hash = c.QueryHash
	if payload.Extensions.PersistedQuery.SHA256Hash == "" {
		payload.Extensions.PersistedQuery.SHA256Hash = defaultQueryHash
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return page{}, fmt.Errorf("encode partner request: %w", err)
	}

	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	serverFailures := 0
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return page{}, fmt.Errorf("create Spotify partner request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Accept-Language", "ru")
		req.Header.Set("Authorization", "Bearer "+c.AccessToken)
		req.Header.Set("Client-Token", c.ClientToken)
		req.Header.Set("Content-Type", "application/json;charset=UTF-8")
		req.Header.Set("App-Platform", "WebPlayer")
		req.Header.Set("DNT", "1")
		req.Header.Set("Origin", "https://open.spotify.com")
		req.Header.Set("Priority", "u=1, i")
		req.Header.Set("Referer", "https://open.spotify.com/")
		req.Header.Set("Sec-CH-UA", `"Not=A?Brand";v="99", "Google Chrome";v="151", "Chromium";v="151"`)
		req.Header.Set("Sec-CH-UA-Mobile", "?0")
		req.Header.Set("Sec-CH-UA-Platform", `"Windows"`)
		req.Header.Set("Sec-Fetch-Dest", "empty")
		req.Header.Set("Sec-Fetch-Mode", "cors")
		req.Header.Set("Sec-Fetch-Site", "same-site")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36")
		if c.AppVersion != "" {
			req.Header.Set("Spotify-App-Version", c.AppVersion)
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return page{}, fmt.Errorf("Spotify partner request failed: %w", err)
		}
		responseBody, readErr := readResponse(resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil {
			return page{}, fmt.Errorf("read Spotify partner response: %w", readErr)
		}
		if closeErr != nil {
			return page{}, fmt.Errorf("close Spotify partner response: %w", closeErr)
		}

		switch {
		case resp.StatusCode == http.StatusOK:
			var response queryResponse
			if err := json.Unmarshal(responseBody, &response); err != nil {
				return page{}, fmt.Errorf("decode Spotify partner response: %w", err)
			}
			if len(response.Errors) > 0 {
				return page{}, fmt.Errorf("Spotify partner GraphQL error: %s", response.Errors[0].Message)
			}
			if len(response.Data.Lists) == 0 {
				return page{}, errors.New("Spotify partner response has no recents list")
			}
			return response.Data.Lists[0].Items, nil

		case resp.StatusCode == http.StatusUnauthorized:
			return page{}, errors.New("Spotify partner API returned HTTP 401 Unauthorized: authorization token is invalid or expired")

		case resp.StatusCode == http.StatusForbidden:
			return page{}, errors.New("Spotify partner API returned HTTP 403 Forbidden: client-token, app version, request hash, or Web Player session was rejected")

		case resp.StatusCode == http.StatusTooManyRequests:
			if err := sleepContext(ctx, retryAfter(resp.Header.Get("Retry-After"), time.Now())); err != nil {
				return page{}, err
			}
			continue

		case resp.StatusCode >= 500 && resp.StatusCode <= 599:
			if serverFailures >= max5xxRetries {
				return page{}, fmt.Errorf("Spotify partner server error %s after %d retries", resp.Status, max5xxRetries)
			}
			delay := time.Second << serverFailures
			serverFailures++
			if err := sleepContext(ctx, delay); err != nil {
				return page{}, err
			}
			continue

		default:
			return page{}, fmt.Errorf("Spotify partner API error: %s", resp.Status)
		}
	}
}

func readResponse(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxResponseSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResponseSize {
		return nil, fmt.Errorf("response exceeds %d bytes", maxResponseSize)
	}
	return data, nil
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
