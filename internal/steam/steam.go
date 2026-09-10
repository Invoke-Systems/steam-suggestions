package steam

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"steam-suggestions/internal/store"
)

const (
	UserAgent  = "Mozilla/5.0 (compatible; SteamSuggestions/1.0; +https://localhost)"
	tagListURL = "https://api.steampowered.com/IStoreService/GetTagList/v1/?language=english"
	itemsURL   = "https://api.steampowered.com/IStoreBrowseService/GetItems/v1/"
	appListURL = "https://api.steampowered.com/IStoreService/GetAppList/v1/"
)

type Client struct {
	HTTP     *http.Client
	APIKey   string
	Country  string
	Currency string
}

type Item struct {
	AppID              int
	Name               string
	IsFree             bool
	Tags               []store.GameTag
	BestPurchaseOption *PurchaseOption
	ReviewCount        int
	PercentPositive    int
	HasReviews         bool
}

type PurchaseOption struct {
	FinalPriceInCents    string `json:"final_price_in_cents"`
	OriginalPriceInCents string `json:"original_price_in_cents"`
	FormattedFinalPrice  string `json:"formatted_final_price"`
	DiscountPct          int    `json:"discount_pct"`
}

type RateError struct {
	Status int
}

func (e *RateError) Error() string {
	return fmt.Sprintf("steam http %d", e.Status)
}

type HTTPError struct {
	Status int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("steam http %d", e.Status)
}

type StatusError struct {
	Code int
	Msg  string
}

func (e *StatusError) Error() string {
	return e.Msg
}

func Status(code int, msg string) *StatusError {
	return &StatusError{Code: code, Msg: msg}
}

func New(apiKey, country, currency string) *Client {
	if country == "" {
		country = "US"
	}
	if currency == "" {
		currency = "USD"
	}
	return &Client{
		HTTP:     &http.Client{Timeout: 12 * time.Second},
		APIKey:   apiKey,
		Country:  strings.ToUpper(country),
		Currency: strings.ToUpper(currency),
	}
}

func (c *Client) get(rawURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500 {
		return nil, &RateError{Status: res.StatusCode}
	}
	if res.StatusCode != http.StatusOK {
		return nil, &HTTPError{Status: res.StatusCode}
	}
	return body, nil
}

func (c *Client) TagList() ([]store.DictTag, string, error) {
	body, err := c.get(tagListURL)
	if err != nil {
		return nil, "", err
	}
	var parsed struct {
		Response struct {
			VersionHash string `json:"version_hash"`
			Tags        []struct {
				TagID int    `json:"tagid"`
				Name  string `json:"name"`
			} `json:"tags"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, "", err
	}
	out := make([]store.DictTag, 0, len(parsed.Response.Tags))
	for _, tag := range parsed.Response.Tags {
		out = append(out, store.DictTag{TagID: tag.TagID, Name: tag.Name})
	}
	return out, parsed.Response.VersionHash, nil
}

func (c *Client) GetItems(appids []int) ([]Item, error) {
	ids := make([]map[string]int, 0, len(appids))
	for _, id := range appids {
		ids = append(ids, map[string]int{"appid": id})
	}
	input, err := json.Marshal(map[string]any{
		"ids": ids,
		"context": map[string]string{
			"language":     "english",
			"country_code": c.Country,
		},
		"data_request": map[string]any{
			"include_tag_count":            20,
			"include_all_purchase_options": true,
			"include_reviews":              true,
		},
	})
	if err != nil {
		return nil, err
	}
	raw := itemsURL + "?input_json=" + url.QueryEscape(string(input))
	body, err := c.get(raw)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Response struct {
			StoreItems []struct {
				AppID  int    `json:"appid"`
				Name   string `json:"name"`
				IsFree bool   `json:"is_free"`
				Tags   []struct {
					TagID  int `json:"tagid"`
					Weight int `json:"weight"`
				} `json:"tags"`
				BestPurchaseOption *PurchaseOption `json:"best_purchase_option"`
				Reviews            *struct {
					SummaryFiltered *struct {
						ReviewCount     int `json:"review_count"`
						PercentPositive int `json:"percent_positive"`
					} `json:"summary_filtered"`
				} `json:"reviews"`
			} `json:"store_items"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(parsed.Response.StoreItems))
	for _, row := range parsed.Response.StoreItems {
		item := Item{
			AppID:              row.AppID,
			Name:               row.Name,
			IsFree:             row.IsFree,
			BestPurchaseOption: row.BestPurchaseOption,
		}
		if row.Reviews != nil && row.Reviews.SummaryFiltered != nil {
			item.HasReviews = true
			item.ReviewCount = row.Reviews.SummaryFiltered.ReviewCount
			item.PercentPositive = row.Reviews.SummaryFiltered.PercentPositive
		}
		for _, tag := range row.Tags {
			item.Tags = append(item.Tags, store.GameTag{TagID: tag.TagID, Weight: tag.Weight})
		}
		items = append(items, item)
	}
	return items, nil
}

func (c *Client) PriceFromItem(item Item, now int64) store.Price {
	opt := item.BestPurchaseOption
	if item.IsFree && opt == nil {
		return store.Price{Name: item.Name, T: now, Currency: c.Currency, Formatted: "Free"}
	}
	if opt == nil {
		return store.Price{Name: item.Name, T: now, Currency: "", Formatted: "Free or unlisted"}
	}
	final := atoi(opt.FinalPriceInCents)
	initial := atoi(opt.OriginalPriceInCents)
	if initial == 0 {
		initial = final
	}
	return store.Price{
		Name:      item.Name,
		T:         now,
		Currency:  c.Currency,
		Initial:   initial,
		Final:     final,
		Discount:  opt.DiscountPct,
		Formatted: opt.FormattedFinalPrice,
	}
}

func (c *Client) AppListPage(lastAppid int) (apps []store.App, next int, more bool, err error) {
	if c.APIKey == "" {
		return nil, lastAppid, false, fmt.Errorf("STEAM_API_KEY is required for the app list")
	}
	u, err := url.Parse(appListURL)
	if err != nil {
		return nil, lastAppid, false, err
	}
	q := u.Query()
	q.Set("key", c.APIKey)
	q.Set("include_games", "true")
	q.Set("max_results", "50000")
	if lastAppid > 0 {
		q.Set("last_appid", strconv.Itoa(lastAppid))
	}
	u.RawQuery = q.Encode()
	body, err := c.get(u.String())
	if err != nil {
		return nil, lastAppid, false, err
	}
	var parsed struct {
		Response struct {
			Apps []struct {
				AppID             int    `json:"appid"`
				Name              string `json:"name"`
				PriceChangeNumber int    `json:"price_change_number"`
			} `json:"apps"`
			HaveMoreResults bool `json:"have_more_results"`
			LastAppid       int  `json:"last_appid"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, lastAppid, false, err
	}
	apps = make([]store.App, 0, len(parsed.Response.Apps))
	for _, app := range parsed.Response.Apps {
		change := app.PriceChangeNumber
		apps = append(apps, store.App{
			AppID:             app.AppID,
			Name:              app.Name,
			PriceChangeNumber: &change,
		})
	}
	next = parsed.Response.LastAppid
	if next == 0 && len(apps) > 0 {
		next = apps[len(apps)-1].AppID
	}
	return apps, next, parsed.Response.HaveMoreResults, nil
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
