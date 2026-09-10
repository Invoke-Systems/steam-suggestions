package steam

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"steam-suggestions/internal/store"
)

const reviewFreshMs = 24 * 60 * 60 * 1000

func PersistReviews(db *store.DB, item Item) {
	if !item.HasReviews {
		return
	}
	_ = db.SetReviews(item.AppID, item.ReviewCount, item.PercentPositive, ReviewsToPopularity(item.ReviewCount))
}

func ReviewsToPopularity(n int) int {
	if n <= 0 {
		return 25
	}
	log := math.Log10(float64(n) + 1)
	min := math.Log10(80)
	max := math.Log10(1_500_000)
	v := ((log - min) / (max - min)) * 100
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return int(math.Round(v))
}

func (c *Client) ReviewCount(appid int) (int, error) {
	u, _ := url.Parse(fmt.Sprintf("https://store.steampowered.com/appreviews/%d", appid))
	q := u.Query()
	q.Set("json", "1")
	q.Set("language", "all")
	q.Set("purchase_type", "all")
	q.Set("num_per_page", "0")
	q.Set("filter", "all")
	u.RawQuery = q.Encode()
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", UserAgent)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500 {
		return 0, &RateError{Status: res.StatusCode}
	}
	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("reviews http %d", res.StatusCode)
	}
	var parsed struct {
		QuerySummary struct {
			TotalReviews int `json:"total_reviews"`
		} `json:"query_summary"`
	}
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return 0, err
	}
	return parsed.QuerySummary.TotalReviews, nil
}

func (c *Client) EnsureReviews(db *store.DB, appids []int) map[int]store.Review {
	ids := uniqueIDs(appids)
	cached := db.GetReviews(ids)
	now := time.Now().UnixMilli()
	var missing []int
	out := map[int]store.Review{}
	for _, id := range ids {
		row, ok := cached[id]
		if ok && now-row.T < reviewFreshMs {
			out[id] = row
			continue
		}
		missing = append(missing, id)
	}
	for _, id := range missing {
		total, err := c.ReviewCount(id)
		if err != nil {
			if _, ok := err.(*RateError); ok {
				time.Sleep(900 * time.Millisecond)
			}
			continue
		}
		pop := ReviewsToPopularity(total)
		_ = db.SetReviews(id, total, 0, pop)
		out[id] = store.Review{AppID: id, T: now, Total: total, Popularity: pop}
		time.Sleep(80 * time.Millisecond)
	}
	return out
}

func (c *Client) FillReviews(db *store.DB, appids []int, max int) map[int]store.Review {
	if max <= 0 {
		max = 320
	}
	have := db.GetReviews(uniqueIDs(appids))
	var missing []int
	for _, id := range uniqueIDs(appids) {
		if _, ok := have[id]; ok {
			continue
		}
		missing = append(missing, id)
		if len(missing) >= max {
			break
		}
	}
	const batch = 40
	for i := 0; i < len(missing); i += batch {
		end := i + batch
		if end > len(missing) {
			end = len(missing)
		}
		items, err := c.GetItems(missing[i:end])
		if err != nil {
			if _, ok := err.(*RateError); ok {
				time.Sleep(900 * time.Millisecond)
			}
			continue
		}
		for _, item := range items {
			PersistReviews(db, item)
		}
		if end < len(missing) {
			time.Sleep(150 * time.Millisecond)
		}
	}
	return db.GetReviews(appids)
}

type SaleItem struct {
	AppID     int    `json:"appid"`
	Name      string `json:"name"`
	Discount  int    `json:"discount"`
	Formatted string `json:"formatted"`
	Final     int    `json:"final"`
	Header    string `json:"header"`
	SteamURL  string `json:"steamUrl"`
}

type Featured struct {
	Specials    []SaleItem `json:"specials"`
	ComingSoon  []SaleItem `json:"comingSoon"`
	NewReleases []SaleItem `json:"newReleases"`
	TopSellers  []SaleItem `json:"topSellers"`
}

type featuredItems struct {
	Items []featuredItem `json:"items"`
}

type featuredItem struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	DiscountPercent   int    `json:"discount_percent"`
	FinalPrice        int    `json:"final_price"`
	LargeCapsuleImage string `json:"large_capsule_image"`
	SmallCapsuleImage string `json:"small_capsule_image"`
}

func (c *Client) FeaturedSales() []SaleItem {
	return c.FeaturedCategories().Specials
}

func (c *Client) FeaturedCategories() Featured {
	u, _ := url.Parse("https://store.steampowered.com/api/featuredcategories")
	q := u.Query()
	q.Set("cc", c.Country)
	q.Set("l", "english")
	u.RawQuery = q.Encode()
	body, err := c.get(u.String())
	if err != nil {
		return Featured{}
	}
	return parseFeaturedCategories(body)
}

func parseFeaturedCategories(body []byte) Featured {
	var parsed struct {
		Specials    featuredItems `json:"specials"`
		ComingSoon  featuredItems `json:"coming_soon"`
		NewReleases featuredItems `json:"new_releases"`
		TopSellers  featuredItems `json:"top_sellers"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Featured{}
	}
	return Featured{
		Specials:    mapFeaturedItems(parsed.Specials.Items),
		ComingSoon:  mapFeaturedItems(parsed.ComingSoon.Items),
		NewReleases: mapFeaturedItems(parsed.NewReleases.Items),
		TopSellers:  mapFeaturedItems(parsed.TopSellers.Items),
	}
}

func mapFeaturedItems(items []featuredItem) []SaleItem {
	if len(items) == 0 {
		return []SaleItem{}
	}
	out := make([]SaleItem, 0, len(items))
	for _, item := range items {
		if item.ID == 0 || item.Name == "" {
			continue
		}
		formatted := ""
		if item.FinalPrice != 0 {
			formatted = fmt.Sprintf("$%.2f", float64(item.FinalPrice)/100)
		}
		header := item.LargeCapsuleImage
		if header == "" {
			header = item.SmallCapsuleImage
		}
		out = append(out, SaleItem{
			AppID:     item.ID,
			Name:      item.Name,
			Discount:  item.DiscountPercent,
			Formatted: formatted,
			Final:     item.FinalPrice,
			Header:    header,
			SteamURL:  fmt.Sprintf("https://store.steampowered.com/app/%d/", item.ID),
		})
	}
	return out
}

func (c *Client) EnsurePrices(db *store.DB, appids []int, names map[int]string) map[int]store.PriceView {
	ids := uniqueIDs(appids)
	cached := db.GetPrices(ids)
	now := time.Now().UnixMilli()
	const fresh = 3 * 60 * 60 * 1000
	var missing []int
	out := map[int]store.PriceView{}
	for _, id := range ids {
		row, ok := cached[id]
		if ok && now-row.T < fresh {
			out[id] = row
			continue
		}
		missing = append(missing, id)
	}
	const batch = 40
	for i := 0; i < len(missing); i += batch {
		end := i + batch
		if end > len(missing) {
			end = len(missing)
		}
		chunk := missing[i:end]
		items, err := c.GetItems(chunk)
		if err != nil {
			continue
		}
		seen := map[int]bool{}
		stamp := time.Now().UnixMilli()
		for _, item := range items {
			seen[item.AppID] = true
			price := c.PriceFromItem(item, stamp)
			if price.Name == "" {
				price.Name = names[item.AppID]
			}
			_ = db.SetPrice(item.AppID, price)
			PersistReviews(db, item)
		}
		for _, id := range chunk {
			if !seen[id] {
				_ = db.SetPrice(id, store.Price{Name: names[id], T: stamp, Formatted: "Free or unlisted"})
			}
		}
		for id, view := range db.GetPrices(chunk) {
			out[id] = view
		}
		if end < len(missing) {
			time.Sleep(150 * time.Millisecond)
		}
	}
	return out
}

func missingDetails(appid int) map[string]any {
	return map[string]any{"appid": appid, "missing": true, "genres": []string{}}
}

func (c *Client) AppDetails(appid int) (store.AppDetails, error) {
	out := store.AppDetails{AppID: appid, Missing: true}
	if appid <= 0 {
		return out, nil
	}
	u, _ := url.Parse("https://store.steampowered.com/api/appdetails")
	q := u.Query()
	q.Set("appids", strconv.Itoa(appid))
	q.Set("filters", "basic,genres,developers,publishers,release_date,metacritic,categories")
	cc := c.Country
	if cc == "" {
		cc = "US"
	}
	q.Set("cc", strings.ToLower(cc))
	q.Set("l", "english")
	u.RawQuery = q.Encode()
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return out, nil
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return out, nil
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500 {
		return out, &RateError{Status: res.StatusCode}
	}
	if res.StatusCode != http.StatusOK {
		return out, nil
	}
	var parsed map[string]struct {
		Success bool `json:"success"`
		Data    struct {
			Type   string `json:"type"`
			Genres []struct {
				Description string `json:"description"`
			} `json:"genres"`
			Categories []struct {
				Description string `json:"description"`
			} `json:"categories"`
			Developers  []string `json:"developers"`
			Publishers  []string `json:"publishers"`
			ReleaseDate struct {
				Date string `json:"date"`
			} `json:"release_date"`
			Metacritic *struct {
				Score int `json:"score"`
			} `json:"metacritic"`
			ShortDescription string `json:"short_description"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return out, nil
	}
	entry, ok := parsed[strconv.Itoa(appid)]
	if !ok || !entry.Success {
		return out, nil
	}
	genres := []string{}
	for _, g := range entry.Data.Genres {
		if g.Description != "" {
			genres = append(genres, g.Description)
		}
	}
	cats := []string{}
	for _, g := range entry.Data.Categories {
		if g.Description != "" {
			cats = append(cats, g.Description)
		}
	}
	out.Missing = false
	out.Type = entry.Data.Type
	out.Genres = genres
	out.Categories = cats
	out.Developers = entry.Data.Developers
	out.Publishers = entry.Data.Publishers
	out.ReleaseDate = entry.Data.ReleaseDate.Date
	out.ShortDescription = entry.Data.ShortDescription
	if entry.Data.Metacritic != nil {
		score := entry.Data.Metacritic.Score
		out.Metacritic = &score
	}
	return out, nil
}

func (c *Client) StoreDetails(appid int) (map[string]any, error) {
	d, err := c.AppDetails(appid)
	if err != nil {
		return nil, err
	}
	if d.Missing {
		return missingDetails(appid), nil
	}
	var meta any
	if d.Metacritic != nil {
		meta = *d.Metacritic
	}
	return map[string]any{
		"appid":            appid,
		"type":             d.Type,
		"genres":           d.Genres,
		"categories":       d.Categories,
		"developers":       d.Developers,
		"publishers":       d.Publishers,
		"releaseDate":      d.ReleaseDate,
		"metacritic":       meta,
		"shortDescription": d.ShortDescription,
	}, nil
}
