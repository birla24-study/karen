package lyrics

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	lrclibBaseURL   = "https://lrclib.net"
	lrclibUserAgent = "karen-bot/3.0 (+https://github.com/spideyonmoon/karen)"
)

var lrclibHTTPClient = &http.Client{
	Timeout: 7 * time.Second,
}

var (
	featRe     = regexp.MustCompile(`(?i)\s*[\(\[](?:feat\.?|featuring|with|ft\.?).*?[\)\]]`)
	versionRe  = regexp.MustCompile(`(?i)\s*[\(\[](?:remaster(?:ed)?|deluxe|bonus|anniversary|live|explicit|clean|mono|stereo|version|edit).*?[\)\]]`)
	trailingRe = regexp.MustCompile(`(?i)\s*-\s*(?:\d{4}\s+)?remaster.*$`)
)

func sanitizeTitle(title string) string {
	s := featRe.ReplaceAllString(title, "")
	s = versionRe.ReplaceAllString(s, "")
	s = trailingRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func sanitizeArtist(artist string) string {
	s := featRe.ReplaceAllString(artist, "")
	return strings.TrimSpace(s)
}

type lrclibRecord struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

// fetchLRCLIB attempts to retrieve lyrics from LRCLIB using exact match,
// album-agnostic match, and sanitized fuzzy search.
func fetchLRCLIB(ctx context.Context, req Request) (*Result, error) {
	if req.Title == "" || req.Artist == "" {
		return nil, fmt.Errorf("lrclib: title and artist are required")
	}

	// 1. Exact match with album name (if available)
	if req.Album != "" {
		res, err := lrclibGet(ctx, req.Artist, req.Title, req.Album, req.DurationSec)
		if err == nil && res != nil {
			return res, nil
		}
	}

	// 2. Exact match without album name (handles single vs deluxe vs compilation differences)
	res, err := lrclibGet(ctx, req.Artist, req.Title, "", req.DurationSec)
	if err == nil && res != nil {
		return res, nil
	}

	// 3. Sanitized search with duration tolerance
	cleanTitle := sanitizeTitle(req.Title)
	cleanArtist := sanitizeArtist(req.Artist)
	if cleanTitle != "" && cleanArtist != "" {
		res, err := lrclibSearch(ctx, cleanArtist, cleanTitle, req.DurationSec)
		if err == nil && res != nil {
			return res, nil
		}
	}

	return nil, fmt.Errorf("lrclib: no lyrics found for %s - %s", req.Artist, req.Title)
}

func lrclibGet(ctx context.Context, artist, title, album string, durationSec int) (*Result, error) {
	q := url.Values{}
	q.Set("artist_name", artist)
	q.Set("track_name", title)
	if album != "" {
		q.Set("album_name", album)
	}
	if durationSec > 0 {
		q.Set("duration", strconv.Itoa(durationSec))
	}

	reqURL := fmt.Sprintf("%s/api/get?%s", lrclibBaseURL, q.Encode())
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", lrclibUserAgent)

	resp, err := lrclibHTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // not found, continue to next phase
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lrclib: unexpected status %d", resp.StatusCode)
	}

	var rec lrclibRecord
	if err := json.NewDecoder(resp.Body).Decode(&rec); err != nil {
		return nil, err
	}

	return recordToResult(&rec)
}

func lrclibSearch(ctx context.Context, artist, title string, durationSec int) (*Result, error) {
	q := url.Values{}
	q.Set("artist_name", artist)
	q.Set("track_name", title)

	reqURL := fmt.Sprintf("%s/api/search?%s", lrclibBaseURL, q.Encode())
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", lrclibUserAgent)

	resp, err := lrclibHTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lrclib: search returned status %d", resp.StatusCode)
	}

	var records []lrclibRecord
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		return nil, err
	}

	var best *lrclibRecord
	bestDiff := math.MaxFloat64
	const maxAllowedDurationDiff = 5.0

	for i := range records {
		rec := &records[i]
		if rec.Instrumental {
			if best == nil {
				best = rec
			}
			continue
		}
		if strings.TrimSpace(rec.SyncedLyrics) == "" && strings.TrimSpace(rec.PlainLyrics) == "" {
			continue
		}

		if durationSec > 0 && rec.Duration > 0 {
			diff := math.Abs(rec.Duration - float64(durationSec))
			if diff <= maxAllowedDurationDiff && diff < bestDiff {
				best = rec
				bestDiff = diff
			}
		} else if best == nil {
			best = rec
		}
	}

	if best != nil {
		return recordToResult(best)
	}

	return nil, nil
}

func recordToResult(rec *lrclibRecord) (*Result, error) {
	if rec == nil {
		return nil, nil
	}
	if rec.Instrumental {
		return &Result{
			SyncType:     "none",
			Provider:     "lrclib",
			Instrumental: true,
		}, nil
	}

	synced := strings.TrimSpace(rec.SyncedLyrics)
	if synced != "" {
		return &Result{
			LRC:      synced,
			SyncType: "line",
			Provider: "lrclib",
		}, nil
	}

	plain := strings.TrimSpace(rec.PlainLyrics)
	if plain != "" {
		return &Result{
			LRC:      plain,
			SyncType: "none",
			Provider: "lrclib",
		}, nil
	}

	return nil, nil
}

