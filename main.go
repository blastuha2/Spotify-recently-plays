package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"spotify-history/internal/config"
	"spotify-history/internal/export"
	"spotify-history/internal/partner"
	"spotify-history/internal/spotify"
	"spotify-history/internal/stats"
)

const dateLayout = "2006-01-02"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run() error {
	if err := config.LoadEnvFile(".env"); err != nil {
		return err
	}
	defaultFrom := time.Now().UTC().AddDate(0, -4, 0).Format(dateLayout)
	modeValue := flag.String("mode", "official", "export mode: official or partner")
	fromValue := flag.String("from", defaultFrom, "start date, inclusive (YYYY-MM-DD)")
	csvValue := flag.String("csv", "", "CSV output path (mode-specific default if empty)")
	jsonValue := flag.String("json", "", "JSON output path (mode-specific default if empty)")
	flag.Parse()

	from, err := parseFrom(*fromValue)
	if err != nil {
		return err
	}
	mode := strings.ToLower(strings.TrimSpace(*modeValue))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch mode {
	case "official":
		csvPath := defaultPath(*csvValue, "spotify_history.csv")
		jsonPath := defaultPath(*jsonValue, "spotify_history.json")
		return runOfficial(ctx, from, csvPath, jsonPath)
	case "partner":
		csvPath := defaultPath(*csvValue, "spotify_partner_history.csv")
		jsonPath := defaultPath(*jsonValue, "spotify_partner_history.json")
		return runPartner(ctx, from, csvPath, jsonPath)
	default:
		return fmt.Errorf("invalid -mode %q: expected official or partner", *modeValue)
	}
}

func runOfficial(ctx context.Context, from time.Time, csvPath, jsonPath string) error {
	token := bearerValue(os.Getenv("SPOTIFY_TOKEN"))
	if token == "" {
		return errors.New("SPOTIFY_TOKEN is not set; add it to .env or set it in PowerShell")
	}

	fmt.Printf("Spotify history export\nMode: official Web API\nFrom: %s\n\n", from.Format(dateLayout))
	client := spotify.NewClient(token)
	result, err := client.FetchHistory(ctx, from, func(progress spotify.Progress) {
		fmt.Printf("Request: %d | Received: %d | Total: %d | Oldest: %s\n",
			progress.Request,
			progress.Received,
			progress.Total,
			progress.Oldest.UTC().Format("2006-01-02 15:04"),
		)
	})
	if err != nil {
		return err
	}

	sort.SliceStable(result.Events, func(i, j int) bool {
		return result.Events[i].PlayedAt.Before(result.Events[j].PlayedAt)
	})
	if err := export.CSV(csvPath, result.Events); err != nil {
		return err
	}
	if err := export.JSON(jsonPath, result.Events); err != nil {
		return err
	}
	printOfficialSummary(result, from, csvPath, jsonPath)
	return nil
}

func runPartner(ctx context.Context, from time.Time, csvPath, jsonPath string) error {
	accessToken := bearerValue(os.Getenv("SPOTIFY_PARTNER_TOKEN"))
	clientToken := strings.TrimSpace(os.Getenv("SPOTIFY_CLIENT_TOKEN"))
	appVersion := strings.TrimSpace(os.Getenv("SPOTIFY_APP_VERSION"))
	if accessToken == "" {
		return errors.New("SPOTIFY_PARTNER_TOKEN is not set; add the value after Bearer to .env")
	}
	if clientToken == "" {
		return errors.New("SPOTIFY_CLIENT_TOKEN is not set; add client-token to .env")
	}
	if appVersion == "" {
		return errors.New("SPOTIFY_APP_VERSION is not set; add spotify-app-version to .env")
	}

	fmt.Printf("Spotify history export\nMode: partner Web Player recents\nFrom: %s\n\n", from.Format(dateLayout))
	client := partner.NewClient(accessToken, clientToken)
	client.AppVersion = appVersion
	if hash := strings.TrimSpace(os.Getenv("SPOTIFY_RECENTS_HASH")); hash != "" {
		client.QueryHash = hash
	}
	result, err := client.FetchRecents(ctx, from, func(progress partner.Progress) {
		oldest := "unknown"
		if !progress.Oldest.IsZero() {
			oldest = progress.Oldest.Format(dateLayout)
		}
		fmt.Printf("Request: %d | Offset: %d | Received: %d | Tracks: %d | Available: %d | Oldest: %s\n",
			progress.Request,
			progress.Offset,
			progress.Received,
			progress.Tracks,
			progress.TotalItems,
			oldest,
		)
	})
	if err != nil {
		return err
	}

	// The source is newest-first. For equal calendar dates, a larger source
	// position is older and therefore comes first in the export.
	sort.SliceStable(result.Tracks, func(i, j int) bool {
		if result.Tracks[i].Date.Equal(result.Tracks[j].Date) {
			return result.Tracks[i].SourcePosition > result.Tracks[j].SourcePosition
		}
		return result.Tracks[i].Date.Before(result.Tracks[j].Date)
	})
	if err := export.PartnerCSV(csvPath, result.Tracks); err != nil {
		return err
	}
	if err := export.PartnerJSON(jsonPath, result.Tracks); err != nil {
		return err
	}
	printPartnerSummary(result, from, csvPath, jsonPath)
	return nil
}

func parseFrom(value string) (time.Time, error) {
	from, err := time.ParseInLocation(dateLayout, value, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid -from %q: expected YYYY-MM-DD", value)
	}
	return from, nil
}

func defaultPath(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func bearerValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= len("Bearer ") && strings.EqualFold(value[:len("Bearer ")], "Bearer ") {
		return strings.TrimSpace(value[len("Bearer "):])
	}
	return value
}

func printOfficialSummary(result spotify.Result, from time.Time, csvPath, jsonPath string) {
	fmt.Printf("\nStop reason: %s\n", result.StopReason)
	if !result.ReachedCutoff {
		fmt.Printf("\nWARNING:\nSpotify API history ended before requested date.\n\nRequested from: %s\n", from.Format(dateLayout))
		if len(result.Events) > 0 {
			fmt.Printf("Oldest available event: %s\n", result.Events[0].PlayedAt.UTC().Format("2006-01-02 15:04"))
		} else {
			fmt.Println("Oldest available event: none")
		}
	}

	summary := stats.Calculate(result.Events)
	fmt.Printf("\nDone.\n\nAPI requests: %d\nListening events: %d\n", result.APIRequests, len(result.Events))
	if len(result.Events) > 0 {
		fmt.Printf("Newest event: %s\nOldest event: %s\n",
			summary.Last.Format("2006-01-02 15:04"),
			summary.First.Format("2006-01-02 15:04"),
		)
	}
	fmt.Printf("\nCSV: %s\nJSON: %s\n", csvPath, jsonPath)
	printStats(summary)
}

func printPartnerSummary(result partner.Result, from time.Time, csvPath, jsonPath string) {
	fmt.Printf("\nStop reason: %s\n", result.StopReason)
	if !result.ReachedCutoff {
		fmt.Printf("\nWARNING:\nSpotify partner history ended before requested date.\n\nRequested from: %s\n", from.Format(dateLayout))
		if len(result.Tracks) > 0 {
			fmt.Printf("Oldest available date: %s\n", result.Tracks[0].PlayedDate)
		} else {
			fmt.Println("Oldest available date: none")
		}
	}

	summary := stats.CalculatePartner(result.Tracks)
	fmt.Printf("\nDone.\n\nAPI requests: %d\nAvailable list items: %d\nExported played track occurrences: %d\nSkipped non-track/empty/non-played items: %d\n",
		result.APIRequests, result.TotalItems, len(result.Tracks), result.SkippedItems)
	if len(result.Tracks) > 0 {
		fmt.Printf("Newest date: %s\nOldest date: %s\n", summary.Last.Format(dateLayout), summary.First.Format(dateLayout))
	}
	fmt.Printf("\nCSV: %s\nJSON: %s\n", csvPath, jsonPath)
	fmt.Println("\nNote: partner recents provides calendar dates only; exact played_at times are unavailable.")
	printStats(summary)
}

func printStats(summary stats.Summary) {
	fmt.Printf("\nStatistics\nTotal plays: %d\nUnique tracks: %d\nUnique artists: %d\n",
		summary.TotalPlays, summary.UniqueTracks, summary.UniqueArtists)
	if !summary.First.IsZero() {
		fmt.Printf("First listening date: %s\nLast listening date: %s\n",
			summary.First.Format(dateLayout), summary.Last.Format(dateLayout))
	}
	if len(summary.PlaysPerMonth) > 0 {
		fmt.Println("\nPlays per month:")
		for _, month := range stats.SortedKeys(summary.PlaysPerMonth) {
			fmt.Printf("%s: %d\n", month, summary.PlaysPerMonth[month])
		}
	}
	if len(summary.PlaysPerDay) > 0 {
		fmt.Println("\nPlays per day:")
		for _, day := range stats.SortedKeys(summary.PlaysPerDay) {
			fmt.Printf("%s: %d\n", day, summary.PlaysPerDay[day])
		}
	}
}
