package steam

import (
	"encoding/json"
	"net/url"
)

func (c *Client) Wishlist(steamid string) []int {
	if c.APIKey == "" || steamid == "" {
		return nil
	}
	u, _ := url.Parse("https://api.steampowered.com/IWishlistService/GetWishlist/v1/")
	q := u.Query()
	q.Set("key", c.APIKey)
	q.Set("steamid", steamid)
	u.RawQuery = q.Encode()
	body, err := c.get(u.String())
	if err != nil {
		return nil
	}
	var parsed struct {
		Response struct {
			Items []struct {
				AppID int `json:"appid"`
			} `json:"items"`
		} `json:"response"`
	}
	if json.Unmarshal(body, &parsed) != nil {
		return nil
	}
	var ids []int
	for _, item := range parsed.Response.Items {
		if item.AppID > 0 {
			ids = append(ids, item.AppID)
		}
	}
	return ids
}
