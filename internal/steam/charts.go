package steam

import (
	"encoding/json"
	"net/url"
)

// ChartRank is one row from ISteamChartsService concurrent charts.
type ChartRank struct {
	Rank             int
	AppID            int
	ConcurrentInGame int
	PeakInGame       int
}

// GamesByConcurrentPlayers returns Steam's live top chart (~100 games).
func (c *Client) GamesByConcurrentPlayers() ([]ChartRank, error) {
	u, _ := url.Parse("https://api.steampowered.com/ISteamChartsService/GetGamesByConcurrentPlayers/v1/")
	q := u.Query()
	if c.APIKey != "" {
		q.Set("key", c.APIKey)
	}
	u.RawQuery = q.Encode()
	body, err := c.get(u.String())
	if err != nil {
		return nil, err
	}
	return parseGamesByConcurrentPlayers(body)
}

func parseGamesByConcurrentPlayers(body []byte) ([]ChartRank, error) {
	var parsed struct {
		Response struct {
			Ranks []struct {
				Rank             int `json:"rank"`
				AppID            int `json:"appid"`
				ConcurrentInGame int `json:"concurrent_in_game"`
				PeakInGame       int `json:"peak_in_game"`
			} `json:"ranks"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	out := make([]ChartRank, 0, len(parsed.Response.Ranks))
	for _, row := range parsed.Response.Ranks {
		if row.AppID <= 0 {
			continue
		}
		out = append(out, ChartRank{
			Rank:             row.Rank,
			AppID:            row.AppID,
			ConcurrentInGame: row.ConcurrentInGame,
			PeakInGame:       row.PeakInGame,
		})
	}
	return out, nil
}
