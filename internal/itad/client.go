package itad

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	BaseURL   = "https://api.isthereanydeal.com"
	SteamShop = 61
	UserAgent = "ShouldIPlay/1.0 (+https://shouldiplay.co; ITAD historical low backfill)"
)

type Client struct {
	HTTP    *http.Client
	APIKey  string
	Country string
}

type RateError struct {
	Status     int
	RetryAfter time.Duration
}

func (e *RateError) Error() string {
	return fmt.Sprintf("itad http %d", e.Status)
}

type HTTPError struct {
	Status int
	Body   string
}

func (e *HTTPError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("itad http %d: %s", e.Status, e.Body)
	}
	return fmt.Sprintf("itad http %d", e.Status)
}

type StoreLow struct {
	GID       string
	Cents     int
	Currency  string
	Shop      string
	ShopID    int
	Timestamp time.Time
}

func New(apiKey, country string) *Client {
	if country == "" {
		country = "US"
	}
	return &Client{
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		APIKey:  strings.TrimSpace(apiKey),
		Country: strings.ToUpper(country),
	}
}

func ShopAppID(appid int) string {
	return "app/" + strconv.Itoa(appid)
}

// LookupSteam maps Steam appids to ITAD game UUIDs. Missing games are omitted (null responses).
func (c *Client) LookupSteam(appids []int) (map[int]string, error) {
	out := map[int]string{}
	if len(appids) == 0 {
		return out, nil
	}
	body := make([]string, 0, len(appids))
	for _, id := range appids {
		if id > 0 {
			body = append(body, ShopAppID(id))
		}
	}
	if len(body) == 0 {
		return out, nil
	}
	raw, err := c.postJSON("/lookup/id/shop/"+strconv.Itoa(SteamShop)+"/v1", nil, body)
	if err != nil {
		return nil, err
	}
	var parsed map[string]*string
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	for _, id := range appids {
		key := ShopAppID(id)
		if gid, ok := parsed[key]; ok && gid != nil && *gid != "" {
			out[id] = *gid
		}
	}
	return out, nil
}

// SteamStoreLows returns historical Steam-store lows for ITAD game UUIDs.
func (c *Client) SteamStoreLows(gids []string) ([]StoreLow, error) {
	if len(gids) == 0 {
		return nil, nil
	}
	q := url.Values{}
	q.Set("country", c.Country)
	q.Set("shops", strconv.Itoa(SteamShop))
	raw, err := c.postJSON("/games/storelow/v2", q, gids)
	if err != nil {
		return nil, err
	}
	var parsed []struct {
		ID   string `json:"id"`
		Lows []struct {
			Shop struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"shop"`
			Price struct {
				AmountInt int    `json:"amountInt"`
				Currency  string `json:"currency"`
			} `json:"price"`
			Timestamp string `json:"timestamp"`
		} `json:"lows"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	var out []StoreLow
	for _, row := range parsed {
		for _, low := range row.Lows {
			if low.Shop.ID != 0 && low.Shop.ID != SteamShop {
				continue
			}
			if low.Price.AmountInt <= 0 {
				continue
			}
			ts, _ := time.Parse(time.RFC3339, low.Timestamp)
			out = append(out, StoreLow{
				GID:       row.ID,
				Cents:     low.Price.AmountInt,
				Currency:  low.Price.Currency,
				Shop:      low.Shop.Name,
				ShopID:    low.Shop.ID,
				Timestamp: ts,
			})
			break
		}
	}
	return out, nil
}

func (c *Client) postJSON(path string, query url.Values, payload any) ([]byte, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("ITAD_API_KEY is required")
	}
	if query == nil {
		query = url.Values{}
	}
	query.Set("key", c.APIKey)
	u := BaseURL + path + "?" + query.Encode()
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("ITAD-API-Key", c.APIKey)
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusTooManyRequests {
		retry := 60 * time.Second
		if ra := res.Header.Get("Retry-After"); ra != "" {
			if sec, err := strconv.Atoi(ra); err == nil && sec > 0 {
				retry = time.Duration(sec) * time.Second
			}
		}
		return nil, &RateError{Status: res.StatusCode, RetryAfter: retry}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return nil, &HTTPError{Status: res.StatusCode, Body: msg}
	}
	return raw, nil
}
