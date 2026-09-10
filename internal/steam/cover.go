package steam

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const maxCoverBytes = 2 << 20

var jpegMagic = []byte{0xff, 0xd8, 0xff}
var pngMagic = []byte{0x89, 0x50, 0x4e, 0x47}

// CachedCoverURL is the local on-demand cover path served by this app.
func CachedCoverURL(appid int) string {
	if appid <= 0 {
		return ""
	}
	return "/images/" + strconv.Itoa(appid) + ".jpg"
}

// CoverURLs is the Steam CDN fallback list for a store header.
func CoverURLs(appid int) []string {
	id := strconv.Itoa(appid)
	return []string{
		HeaderURL(appid),
		"https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/" + id + "/header.jpg",
		"https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/" + id + "/capsule_616x353.jpg",
		"https://cdn.cloudflare.steamstatic.com/steam/apps/" + id + "/header.jpg",
	}
}

func LooksLikeImage(raw []byte) bool {
	return bytes.HasPrefix(raw, jpegMagic) || bytes.HasPrefix(raw, pngMagic)
}

// FetchCover tries header, then capsule, then library hero. First valid image wins.
func (c *Client) FetchCover(appid int) ([]byte, error) {
	if appid <= 0 {
		return nil, &HTTPError{Status: http.StatusNotFound}
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	var lastErr error
	for _, rawURL := range CoverURLs(appid) {
		raw, err := fetchCoverURL(httpClient, rawURL)
		if err == nil {
			return raw, nil
		}
		lastErr = err
		if rate, ok := err.(*RateError); ok {
			return nil, rate
		}
	}
	if lastErr == nil {
		lastErr = &HTTPError{Status: http.StatusNotFound}
	}
	return nil, lastErr
}

func fetchCoverURL(httpClient *http.Client, rawURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500 {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 8<<10))
		return nil, &RateError{Status: res.StatusCode}
	}
	if res.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 8<<10))
		return nil, &HTTPError{Status: res.StatusCode}
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxCoverBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || len(raw) > maxCoverBytes || !LooksLikeImage(raw) {
		return nil, fmt.Errorf("not an image")
	}
	return raw, nil
}

func ParseCoverAppID(raw string) (int, bool) {
	raw = strings.TrimSuffix(strings.TrimSpace(raw), ".jpg")
	appid, err := strconv.Atoi(raw)
	if err != nil || appid <= 0 {
		return 0, false
	}
	return appid, true
}
