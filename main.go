package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	authURL       = "https://www.strava.com/oauth/token"
	activitiesURL = "https://www.strava.com/api/v3/athlete/activities"
	perPage       = 200
	httpTimeout   = 30 * time.Second
	maxRetries    = 3
)

// OAuth response shape
type authResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
}

// Activity fields we need
type activity struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Distance float64 `json:"distance"` // meters
	// StartDate string  `json:"start_date"` // RFC3339 if you need it
}

// Env config
type envVars struct {
	StravaClientId     string `mapstructure:"STRAVA_CLIENT_ID"`
	StravaClientSecret string `mapstructure:"STRAVA_CLIENT_SECRET"`
	StravaRefreshToken string `mapstructure:"STRAVA_REFRESH_TOKEN"`
	StravaAccessToken  string `mapstructure:"STRAVA_ACCESS_TOKEN"`
}

func main() {
	logger := log.New(os.Stdout, "[strava] ", log.LstdFlags|log.Lmsgprefix)

	// Load env/secrets (unchanged)
	viper.SetConfigName("strava")
	viper.AddConfigPath(".")
	viper.SetConfigType("env")
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		logger.Fatalf("read config: %v", err)
	}

	var cfg envVars
	if err := viper.Unmarshal(&cfg); err != nil {
		logger.Fatalf("unmarshal config: %v", err)
	}

	// HTTP client
	client := &http.Client{Timeout: httpTimeout}

	// Get a working access token (refresh if needed)
	accessToken, refreshToken, err := ensureAccessToken(context.Background(), client, cfg, logger)
	if err != nil {
		logger.Fatalf("ensure access token: %v", err)
	}

	// Optional: quick probe so logs clearly show if the token works at all
	if err := probeToken(context.Background(), client, accessToken, logger); err != nil {
		logger.Printf("token probe warning: %v", err)
	}

	logger.Println("Authenticated. Fetching activities (pages of 200)…")

	// Fetch all pages
	var all []activity
	page := 1
	for {
		pageActs, status, rl := fetchActivitiesPage(context.Background(), client, accessToken, page, logger)
		logger.Printf("page=%d status=%d rateLimitUsage=%q", page, status, rl)

		if status == http.StatusUnauthorized {
			// Attempt one refresh and retry this page
			logger.Printf("401 unauthorized on page %d; attempting token refresh…", page)
			at, rt, rerr := refreshAccessToken(context.Background(), client, cfg, logger)
			if rerr != nil {
				logger.Fatalf("refresh after 401 failed: %v", rerr)
			}
			accessToken, refreshToken = at, rt

			pageActs, status, rl = fetchActivitiesPage(context.Background(), client, accessToken, page, logger)
			logger.Printf("retry page=%d status=%d rateLimitUsage=%q", page, status, rl)
			if status != http.StatusOK {
				logger.Fatalf("after refresh, fetch failed status=%d", status)
			}
		} else if status == http.StatusTooManyRequests {
			// Basic backoff for 429
			logger.Println("429 rate limited; backing off for 60s before retry…")
			time.Sleep(60 * time.Second)
			continue
		} else if status != http.StatusOK {
			logger.Fatalf("unexpected status code: %d", status)
		}

		logger.Printf("Page %d retrieved with %d activities", page, len(pageActs))
		all = append(all, pageActs...)

		if len(pageActs) < perPage {
			break // last page
		}
		page++
	}

	logger.Printf("Total activities fetched: %d", len(all))

	// Sum distances for activities named exactly "Desk Treadmill" (case-insensitive)
	var deskCount int
	var totalMeters float64
	for _, a := range all {
		if strings.EqualFold(strings.TrimSpace(a.Name), "Desk Treadmill") {
			deskCount++
			totalMeters += a.Distance
		}
	}

	miles := totalMeters * 0.000621371
	logger.Printf("Desk Treadmill Activities: %d", deskCount)
	logger.Printf("Total Distance: %.2f miles", miles)

	// Notify if Strava rotated the refresh token
	if refreshToken != "" && refreshToken != cfg.StravaRefreshToken {
		logger.Println("NOTICE: Strava issued a new refresh token during auth.")
		logger.Println("Update STRAVA_REFRESH_TOKEN in your secrets to avoid future auth failures.")
	}
}

// Try existing access token; refresh if missing/invalid
func ensureAccessToken(ctx context.Context, client *http.Client, cfg envVars, logger *log.Logger) (string, string, error) {
	if strings.TrimSpace(cfg.StravaAccessToken) == "" {
		logger.Println("No STRAVA_ACCESS_TOKEN provided; refreshing with STRAVA_REFRESH_TOKEN…")
		return refreshAccessToken(ctx, client, cfg, logger)
	}

	// Probe with minimal request
	req, _ := http.NewRequestWithContext(ctx, "GET", activitiesURL+"?per_page=1&page=1", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.StravaAccessToken)
	req.Header.Set("Accept", "application/json")
	res, err := client.Do(req)
	if err != nil {
		logger.Printf("token probe failed (will refresh): %v", err)
		return refreshAccessToken(ctx, client, cfg, logger)
	}
	defer res.Body.Close()
	io.Copy(io.Discard, res.Body)

	if res.StatusCode == http.StatusUnauthorized {
		logger.Println("Configured access token rejected; refreshing…")
		return refreshAccessToken(ctx, client, cfg, logger)
	}

	return cfg.StravaAccessToken, cfg.StravaRefreshToken, nil
}

// Refresh OAuth token using refresh_token grant
func refreshAccessToken(ctx context.Context, client *http.Client, cfg envVars, logger *log.Logger) (string, string, error) {
	if strings.TrimSpace(cfg.StravaClientId) == "" ||
		strings.TrimSpace(cfg.StravaClientSecret) == "" ||
		strings.TrimSpace(cfg.StravaRefreshToken) == "" {
		return "", "", errors.New("missing STRAVA_CLIENT_ID/SECRET/REFRESH_TOKEN")
	}

	form := url.Values{}
	form.Set("client_id", cfg.StravaClientId)
	form.Set("client_secret", cfg.StravaClientSecret)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", cfg.StravaRefreshToken)

	req, err := http.NewRequestWithContext(ctx, "POST", authURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", fmt.Errorf("new refresh req: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("refresh req: %w", err)
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("refresh failed status=%d body=%s", res.StatusCode, string(body))
	}

	var auth authResponse
	if err := json.Unmarshal(body, &auth); err != nil {
		return "", "", fmt.Errorf("unmarshal refresh: %w", err)
	}

	logger.Printf("token refreshed; expires in ~%d seconds", auth.ExpiresIn)
	return auth.AccessToken, auth.RefreshToken, nil
}

// Rate-limit observability
type rateLimitUsage struct {
	Usage string // X-RateLimit-Usage, e.g., "10,100"
	Limit string // X-RateLimit-Limit, e.g., "100,1000"
}

func (r rateLimitUsage) String() string {
	if r.Usage == "" && r.Limit == "" {
		return ""
	}
	return fmt.Sprintf("usage=%s limit=%s", r.Usage, r.Limit)
}

// Fetch one page of activities with retries/backoff and rich 401 logging
func fetchActivitiesPage(ctx context.Context, client *http.Client, accessToken string, page int, logger *log.Logger) ([]activity, int, rateLimitUsage) {
	var lastStatus int
	var lastRL rateLimitUsage

	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "GET", activitiesURL, nil)
		if err != nil {
			logger.Printf("build request: %v", err)
			return nil, 0, lastRL
		}
		q := req.URL.Query()
		q.Set("per_page", strconv.Itoa(perPage))
		q.Set("page", strconv.Itoa(page))
		req.URL.RawQuery = q.Encode()
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Accept", "application/json")

		res, err := client.Do(req)
		if err != nil {
			logger.Printf("request error (attempt %d/%d): %v", attempt, maxRetries, err)
			time.Sleep(backoff(attempt))
			continue
		}

		lastStatus = res.StatusCode
		lastRL = rateLimitUsage{
			Usage: res.Header.Get("X-RateLimit-Usage"),
			Limit: res.Header.Get("X-RateLimit-Limit"),
		}

		body, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			logger.Printf("read body (attempt %d/%d): %v", attempt, maxRetries, readErr)
			time.Sleep(backoff(attempt))
			continue
		}

		switch res.StatusCode {
		case http.StatusOK:
			var items []activity
			if err := json.Unmarshal(body, &items); err != nil {
				logger.Printf("unmarshal activities: %v", err)
				return nil, res.StatusCode, lastRL
			}
			return items, res.StatusCode, lastRL

		case http.StatusTooManyRequests:
			sleepFor := backoff(attempt)
			logger.Printf("429 rate limited; retrying in %s (attempt %d/%d)…", sleepFor, attempt, maxRetries)
			time.Sleep(sleepFor)
			continue

		case http.StatusUnauthorized:
			// Log why (scopes are a common root cause).
			wa := res.Header.Get("WWW-Authenticate")
			logger.Printf("401 unauthorized. WWW-Authenticate=%q body=%s", wa, truncate(string(body), 300))
			return nil, res.StatusCode, lastRL

		default:
			logger.Printf("unexpected status=%d body=%s", res.StatusCode, truncate(string(body), 300))
			return nil, res.StatusCode, lastRL
		}
	}

	return nil, lastStatus, lastRL
}

func probeToken(ctx context.Context, client *http.Client, accessToken string, logger *log.Logger) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://www.strava.com/api/v3/athlete", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("probe /athlete request: %w", err)
	}
	defer res.Body.Close()

	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("probe /athlete status=%d body=%s", res.StatusCode, truncate(string(b), 200))
	}
	return nil
}

func backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := time.Duration(1<<uint(attempt-1)) * time.Second
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	return d
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
