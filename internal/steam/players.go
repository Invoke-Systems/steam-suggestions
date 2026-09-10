package steam

import (
	"encoding/json"
	"net/url"
	"strconv"
)

func (c *Client) CurrentPlayers(appid int) (int, error) {
	if appid <= 0 {
		return 0, Status(400, "invalid appid")
	}
	u, _ := url.Parse("https://api.steampowered.com/ISteamUserStats/GetNumberOfCurrentPlayers/v1/")
	q := u.Query()
	q.Set("appid", strconv.Itoa(appid))
	u.RawQuery = q.Encode()
	body, err := c.get(u.String())
	if err != nil {
		return 0, err
	}
	var parsed struct {
		Response struct {
			PlayerCount int `json:"player_count"`
			Result      int `json:"result"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, err
	}
	if parsed.Response.Result != 1 {
		return 0, nil
	}
	if parsed.Response.PlayerCount < 0 {
		return 0, nil
	}
	return parsed.Response.PlayerCount, nil
}
