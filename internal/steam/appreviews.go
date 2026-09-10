package steam

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"steam-suggestions/internal/store"
)

const reviewSnippetFreshSec = 12 * 60 * 60

// RecentReviewSnippets returns a few recent English store reviews for a game page.
func (c *Client) RecentReviewSnippets(appid, limit int) ([]store.ReviewSnippet, error) {
	if appid <= 0 {
		return nil, fmt.Errorf("invalid appid")
	}
	if limit <= 0 {
		limit = 5
	}
	// Ask Steam for a few extras so we can drop empty / tiny posts.
	want := limit + 4
	if want > 20 {
		want = 20
	}
	u, _ := url.Parse(fmt.Sprintf("https://store.steampowered.com/appreviews/%d", appid))
	q := u.Query()
	q.Set("json", "1")
	q.Set("language", "english")
	q.Set("filter", "recent")
	q.Set("purchase_type", "all")
	q.Set("review_type", "all")
	q.Set("num_per_page", strconv.Itoa(want))
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500 {
		return nil, &RateError{Status: res.StatusCode}
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("appreviews http %d", res.StatusCode)
	}
	var parsed struct {
		Reviews []struct {
			RecommendationID string `json:"recommendationid"`
			VotedUp          bool   `json:"voted_up"`
			TimestampCreated int64  `json:"timestamp_created"`
			Review           string `json:"review"`
			Author           struct {
				SteamID          string `json:"steamid"`
				PlaytimeForever  int    `json:"playtime_forever"`
			} `json:"author"`
		} `json:"reviews"`
	}
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	out := make([]store.ReviewSnippet, 0, limit)
	for _, row := range parsed.Reviews {
		text := cleanReviewText(row.Review)
		if utf8.RuneCountInString(text) < 28 {
			continue
		}
		hours := row.Author.PlaytimeForever / 60
		out = append(out, store.ReviewSnippet{
			AppID:     appid,
			ReviewID:  row.RecommendationID,
			SteamID:   row.Author.SteamID,
			VotedUp:   row.VotedUp,
			Hours:     hours,
			CreatedAt: row.TimestampCreated,
			Text:      text,
			URL:       reviewURL(row.Author.SteamID, appid),
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// EnsureReviewSnippets returns cached snippets, refreshing from Steam when stale.
func (c *Client) EnsureReviewSnippets(db *store.DB, appid, limit int) []store.ReviewSnippet {
	if appid <= 0 || db == nil {
		return nil
	}
	if limit <= 0 {
		limit = 5
	}
	if items, fetchedAt, ok := db.GetReviewSnippets(appid, limit); ok {
		if time.Now().Unix()-fetchedAt < reviewSnippetFreshSec {
			return items
		}
	}
	items, err := c.RecentReviewSnippets(appid, limit)
	if err != nil {
		if items, _, ok := db.GetReviewSnippets(appid, limit); ok {
			return items
		}
		return nil
	}
	_ = db.SetReviewSnippets(appid, items)
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

func cleanReviewText(raw string) string {
	text := strings.ReplaceAll(raw, "\r\n", "\n")
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	// Keep short paragraphs readable on the page.
	runes := []rune(text)
	if len(runes) > 320 {
		text = strings.TrimSpace(string(runes[:320])) + "…"
	}
	return text
}

func reviewURL(steamid string, appid int) string {
	if store.ValidSteamID(steamid) {
		return "https://steamcommunity.com/profiles/" + steamid + "/recommended/" + strconv.Itoa(appid) + "/"
	}
	return StoreURL(appid) + "#app_reviews_hash"
}
