package lyrics

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var neteaseHTTPClient = &http.Client{
	Timeout: 7 * time.Second,
}

const (
	neteaseSearchURL = "https://music.163.com/api/search/get/web"
	neteaseLyricURL  = "https://music.163.com/api/song/lyric"
	neteaseUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

type neteaseSearchResponse struct {
	Result struct {
		Songs []struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			Duration int64  `json:"duration"` // in milliseconds
			Artists  []struct {
				Name string `json:"name"`
			} `json:"artists"`
		} `json:"songs"`
		SongCount int `json:"songCount"`
	} `json:"result"`
	Code int `json:"code"`
}

type neteaseLyricResponse struct {
	Lrc struct {
		Lyric   string `json:"lyric"`
		Version int    `json:"version"`
	} `json:"lrc"`
	Code int `json:"code"`
}

// fetchNetEase attempts to search and retrieve synced lyrics from NetEase Cloud Music.
func fetchNetEase(ctx context.Context, req Request) (*Result, error) {
	if req.Title == "" || req.Artist == "" {
		return nil, fmt.Errorf("netease: title and artist are required")
	}

	cleanTitle := sanitizeTitle(req.Title)
	cleanArtist := sanitizeArtist(req.Artist)
	query := fmt.Sprintf("%s %s", cleanArtist, cleanTitle)

	q := url.Values{}
	q.Set("csrf_token", "")
	q.Set("type", "1") // 1 = song
	q.Set("s", query)
	q.Set("limit", "5")

	reqURL := fmt.Sprintf("%s?%s", neteaseSearchURL, q.Encode())
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", neteaseUserAgent)
	httpReq.Header.Set("Referer", "https://music.163.com/")

	resp, err := neteaseHTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("netease: search returned status %d", resp.StatusCode)
	}

	var searchResp neteaseSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}

	if len(searchResp.Result.Songs) == 0 {
		return nil, fmt.Errorf("netease: no songs found for %s", query)
	}

	// Find best matching song by duration
	var matchedID int64
	bestDiff := math.MaxFloat64
	const maxAllowedDurationDiff = 5.0

	for _, song := range searchResp.Result.Songs {
		if req.DurationSec > 0 && song.Duration > 0 {
			songDurationSec := float64(song.Duration) / 1000.0
			diff := math.Abs(songDurationSec - float64(req.DurationSec))
			if diff <= maxAllowedDurationDiff && diff < bestDiff {
				bestDiff = diff
				matchedID = song.ID
			}
		} else if matchedID == 0 {
			matchedID = song.ID
		}
	}

	if matchedID == 0 {
		return nil, fmt.Errorf("netease: no duration-matched track for %s", query)
	}

	return fetchNetEaseLyricByID(ctx, matchedID)
}

func fetchNetEaseLyricByID(ctx context.Context, songID int64) (*Result, error) {
	q := url.Values{}
	q.Set("id", strconv.FormatInt(songID, 10))
	q.Set("lv", "1")
	q.Set("kv", "1")
	q.Set("tv", "-1")

	reqURL := fmt.Sprintf("%s?%s", neteaseLyricURL, q.Encode())
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", neteaseUserAgent)
	httpReq.Header.Set("Referer", "https://music.163.com/")

	resp, err := neteaseHTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("netease: lyric returned status %d", resp.StatusCode)
	}

	var lyricResp neteaseLyricResponse
	if err := json.NewDecoder(resp.Body).Decode(&lyricResp); err != nil {
		return nil, err
	}

	lyric := strings.TrimSpace(lyricResp.Lrc.Lyric)
	if lyric == "" {
		return nil, fmt.Errorf("netease: empty lyrics for id %d", songID)
	}

	syncType := "none"
	if strings.Contains(lyric, "[") && strings.Contains(lyric, "]") {
		syncType = "line"
	}

	return &Result{
		LRC:      lyric,
		SyncType: syncType,
		Provider: "netease",
	}, nil
}

