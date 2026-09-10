package steam

import (
	"encoding/json"
	"errors"
	"net/url"
	"strconv"

	"steam-suggestions/internal/store"
)

func (c *Client) AchievementPercentages(appid int) ([]store.AchievementPct, error) {
	if appid <= 0 {
		return nil, Status(400, "invalid appid")
	}
	u, _ := url.Parse("https://api.steampowered.com/ISteamUserStats/GetGlobalAchievementPercentagesForApp/v2/")
	q := u.Query()
	q.Set("gameid", strconv.Itoa(appid))
	u.RawQuery = q.Encode()
	body, err := c.get(u.String())
	if err != nil {
		var httpErr *HTTPError
		if errors.As(err, &httpErr) {
			// Many apps have no stats schema.
			return []store.AchievementPct{}, nil
		}
		return nil, err
	}
	var parsed struct {
		AchievementPercentages struct {
			Achievements []struct {
				Name    string  `json:"name"`
				Percent float64 `json:"percent"`
			} `json:"achievements"`
		} `json:"achievementpercentages"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return []store.AchievementPct{}, nil
	}
	out := make([]store.AchievementPct, 0, len(parsed.AchievementPercentages.Achievements))
	for _, item := range parsed.AchievementPercentages.Achievements {
		out = append(out, store.AchievementPct{Name: item.Name, Percent: item.Percent})
	}
	return out, nil
}
