package steam

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	profileRE = regexp.MustCompile(`(?i)steamcommunity\.com/profiles/(\d{17})`)
	vanityRE  = regexp.MustCompile(`(?i)steamcommunity\.com/id/([^/?#]+)`)
)

type SteamInput struct {
	SteamID string
	Vanity  string
}

type OwnedGame struct {
	AppID                int    `json:"appid"`
	Name                 string `json:"name"`
	ImgIconURL           string `json:"img_icon_url"`
	PlaytimeForever      int    `json:"playtime_forever"`
	PlaytimeWindows      int    `json:"playtime_windows_forever"`
	PlaytimeMac          int    `json:"playtime_mac_forever"`
	PlaytimeLinux        int    `json:"playtime_linux_forever"`
	PlaytimeDeck         int    `json:"playtime_deck_forever"`
	PlaytimeDisconnected int    `json:"playtime_disconnected"`
	Playtime2Weeks       int    `json:"playtime_2weeks"`
	RtimeLastPlayed      int64  `json:"rtime_last_played"`
}

func (g OwnedGame) PlaytimeMinutes() int {
	byOS := g.PlaytimeWindows + g.PlaytimeMac + g.PlaytimeLinux + g.PlaytimeDeck + g.PlaytimeDisconnected
	if byOS > g.PlaytimeForever {
		return byOS
	}
	return g.PlaytimeForever
}

const recentWindowSec = 14 * 24 * 60 * 60

func (g OwnedGame) PlayedRecently(now int64) bool {
	if g.Playtime2Weeks > 0 {
		return true
	}
	if g.RtimeLastPlayed > 0 && now-g.RtimeLastPlayed <= recentWindowSec {
		return true
	}
	return false
}

type PlayerSummary struct {
	SteamID      string `json:"steamid"`
	PersonaName  string `json:"personaname"`
	AvatarFull   string `json:"avatarfull"`
	AvatarMedium string `json:"avatarmedium"`
	ProfileURL   string `json:"profileurl"`
}

func ParseSteamInput(raw string) (SteamInput, error) {
	input := strings.TrimSpace(raw)
	if input == "" {
		return SteamInput{}, Status(http.StatusBadRequest, "Enter a Steam profile URL, vanity name, or SteamID64.")
	}
	if m := profileRE.FindStringSubmatch(input); len(m) == 2 {
		return SteamInput{SteamID: m[1]}, nil
	}
	if m := vanityRE.FindStringSubmatch(input); len(m) == 2 {
		v, _ := url.QueryUnescape(m[1])
		return SteamInput{Vanity: v}, nil
	}
	if regexp.MustCompile(`^\d{17}$`).MatchString(input) {
		return SteamInput{SteamID: input}, nil
	}
	return SteamInput{Vanity: strings.TrimPrefix(input, "@")}, nil
}

func (c *Client) ResolveSteamID(parsed SteamInput) (string, error) {
	if parsed.SteamID != "" {
		return parsed.SteamID, nil
	}
	u, _ := url.Parse("https://api.steampowered.com/ISteamUser/ResolveVanityURL/v1/")
	q := u.Query()
	q.Set("key", c.APIKey)
	q.Set("vanityurl", parsed.Vanity)
	u.RawQuery = q.Encode()
	body, err := c.get(u.String())
	if err != nil {
		return "", err
	}
	var parsedJSON struct {
		Response struct {
			SteamID string `json:"steamid"`
			Success int    `json:"success"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &parsedJSON); err != nil {
		return "", err
	}
	if parsedJSON.Response.Success != 1 || parsedJSON.Response.SteamID == "" {
		return "", Status(http.StatusNotFound, fmt.Sprintf("Could not resolve Steam vanity URL %q.", parsed.Vanity))
	}
	return parsedJSON.Response.SteamID, nil
}

func (c *Client) OwnedGames(steamid string) ([]OwnedGame, error) {
	u, _ := url.Parse("https://api.steampowered.com/IPlayerService/GetOwnedGames/v1/")
	q := u.Query()
	q.Set("key", c.APIKey)
	q.Set("steamid", steamid)
	q.Set("include_appinfo", "true")
	q.Set("include_played_free_games", "1")
	q.Set("include_free_sub", "1")
	q.Set("format", "json")
	u.RawQuery = q.Encode()
	body, err := c.get(u.String())
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Response struct {
			GameCount int         `json:"game_count"`
			Games     []OwnedGame `json:"games"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Response.Games) == 0 && parsed.Response.GameCount == 0 {
		return nil, Status(http.StatusNotFound, "No games returned. The profile must have Game details set to Public.")
	}
	return parsed.Response.Games, nil
}

func (c *Client) RecentlyPlayed(steamid string) ([]OwnedGame, error) {
	u, _ := url.Parse("https://api.steampowered.com/IPlayerService/GetRecentlyPlayedGames/v1/")
	q := u.Query()
	q.Set("key", c.APIKey)
	q.Set("steamid", steamid)
	u.RawQuery = q.Encode()
	body, err := c.get(u.String())
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Response struct {
			Games []OwnedGame `json:"games"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	return parsed.Response.Games, nil
}

func (c *Client) PlayerSummary(steamid string) (*PlayerSummary, error) {
	u, _ := url.Parse("https://api.steampowered.com/ISteamUser/GetPlayerSummaries/v2/")
	q := u.Query()
	q.Set("key", c.APIKey)
	q.Set("steamids", steamid)
	u.RawQuery = q.Encode()
	body, err := c.get(u.String())
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Response struct {
			Players []PlayerSummary `json:"players"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Response.Players) == 0 {
		return nil, nil
	}
	p := parsed.Response.Players[0]
	return &p, nil
}

func MinutesToHours(minutes int) float64 {
	return float64(int((float64(minutes)/60)*10+0.5)) / 10
}

func HeaderURL(appid int) string {
	return fmt.Sprintf("https://cdn.akamai.steamstatic.com/steam/apps/%d/header.jpg", appid)
}

func IconURL(appid int, hash string) string {
	if hash == "" {
		return ""
	}
	return fmt.Sprintf("https://media.steampowered.com/steamcommunity/public/images/apps/%d/%s.jpg", appid, hash)
}

func StoreURL(appid int) string {
	return "https://store.steampowered.com/app/" + strconv.Itoa(appid) + "/"
}

func AppURL(appid int) string {
	if appid <= 0 {
		return "/"
	}
	return "/app/" + strconv.Itoa(appid)
}

// PlayerURL is the canonical shareable Should I Play profile path for a steamid64.
func PlayerURL(steamid string) string {
	if steamid == "" {
		return "/"
	}
	return "/u/" + steamid
}
