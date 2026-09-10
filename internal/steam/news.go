package steam

import (
	"encoding/json"
	"net/url"
	"strconv"

	"steam-suggestions/internal/store"
)

func (c *Client) NewsForApp(appid, count int) ([]store.NewsItem, error) {
	if appid <= 0 {
		return nil, Status(400, "invalid appid")
	}
	if count <= 0 {
		count = 5
	}
	u, _ := url.Parse("https://api.steampowered.com/ISteamNews/GetNewsForApp/v2/")
	q := u.Query()
	q.Set("appid", strconv.Itoa(appid))
	q.Set("count", strconv.Itoa(count))
	q.Set("maxlength", "300")
	q.Set("format", "json")
	u.RawQuery = q.Encode()
	body, err := c.get(u.String())
	if err != nil {
		return nil, err
	}
	var parsed struct {
		AppNews struct {
			NewsItems []struct {
				GID       string `json:"gid"`
				Title     string `json:"title"`
				URL       string `json:"url"`
				Author    string `json:"author"`
				Contents  string `json:"contents"`
				FeedLabel string `json:"feedlabel"`
				Date      int64  `json:"date"`
			} `json:"newsitems"`
		} `json:"appnews"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	out := make([]store.NewsItem, 0, len(parsed.AppNews.NewsItems))
	for _, item := range parsed.AppNews.NewsItems {
		out = append(out, store.NewsItem{
			AppID:     appid,
			GID:       item.GID,
			Title:     item.Title,
			URL:       item.URL,
			Author:    item.Author,
			FeedLabel: item.FeedLabel,
			Date:      item.Date,
			Contents:  item.Contents,
		})
	}
	return out, nil
}
