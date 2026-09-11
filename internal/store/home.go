package store

import (
	"strconv"
)

// HomeItem is a compact catalog card for landing-page rails.
type HomeItem struct {
	AppID     int    `json:"appid"`
	Name      string `json:"name"`
	Header    string `json:"header"`
	PageURL   string `json:"pageUrl"`
	SteamURL  string `json:"steamUrl"`
	Discount  int    `json:"discount,omitempty"`
	Formatted string `json:"formatted,omitempty"`
	Meta      string `json:"meta"`
}

// HomeRails groups discovery lists for the anonymous landing page.
type HomeRails struct {
	Hot            []HomeItem `json:"hot"`
	NewWithPlayers []HomeItem `json:"newWithPlayers"`
	OnSale         []HomeItem `json:"onSale"`
	TopPlayers     []HomeItem `json:"topPlayers"`
}

func homeURLs(appid int) (header, page, steam string) {
	// Serve through /images so the app applies Steam CDN fallbacks + disk cache.
	header = "/images/" + strconv.Itoa(appid) + ".jpg"
	page = "/app/" + strconv.Itoa(appid)
	steam = "https://store.steampowered.com/app/" + strconv.Itoa(appid) + "/"
	return header, page, steam
}

func HomeItemFromApp(appid int, name, meta string) HomeItem {
	item := HomeItem{AppID: appid, Name: name, Meta: meta}
	item.Header, item.PageURL, item.SteamURL = homeURLs(appid)
	return item
}

func CommaInt(n int) string {
	if n < 0 {
		n = -n
	}
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var b []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b = append(b, ',')
		}
		b = append(b, byte(c))
	}
	return string(b)
}

// HomeHot: concurrent close to our observed peak (ever) — tiny delta = hot.
// Floor 5k players; must be within 20% of peak_all (ratio ≥ 0.80).
func (db *DB) HomeHot(limit int) []HomeItem {
	if limit <= 0 {
		limit = 12
	}
	rows, err := db.sql.Query(`
		SELECT g.appid, g.name, ps.current
		FROM player_stats ps
		JOIN games g ON g.appid = ps.appid
		WHERE g.name != ''
		  AND ps.current >= 5000
		  AND ps.peak_all > ps.current
		  AND (1.0 * ps.current / ps.peak_all) >= 0.80
		ORDER BY (1.0 * ps.current / ps.peak_all) DESC, ps.current DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []HomeItem
	for rows.Next() {
		var item HomeItem
		var current int
		if err := rows.Scan(&item.AppID, &item.Name, &current); err != nil {
			continue
		}
		item.Header, item.PageURL, item.SteamURL = homeURLs(item.AppID)
		item.Meta = CommaInt(current) + " in-game"
		out = append(out, item)
	}
	_ = rows.Err()
	return out
}

// HomeNewWithPlayers: newer Steam titles with a real concurrent floor (≥5k).
func (db *DB) HomeNewWithPlayers(limit int) []HomeItem {
	if limit <= 0 {
		limit = 12
	}
	rows, err := db.sql.Query(`
		SELECT g.appid, g.name, ps.current
		FROM player_stats ps
		JOIN games g ON g.appid = ps.appid
		WHERE ps.current >= 5000 AND g.name != ''
		ORDER BY g.appid DESC, ps.current DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []HomeItem
	for rows.Next() {
		var item HomeItem
		var current int
		if err := rows.Scan(&item.AppID, &item.Name, &current); err != nil {
			continue
		}
		item.Header, item.PageURL, item.SteamURL = homeURLs(item.AppID)
		item.Meta = CommaInt(current) + " in-game"
		out = append(out, item)
	}
	_ = rows.Err()
	return out
}

func (db *DB) HomeTopPlayers(limit int) []HomeItem {
	if limit <= 0 {
		limit = 12
	}
	rows, err := db.sql.Query(`
		SELECT g.appid, g.name, ps.current
		FROM player_stats ps
		JOIN games g ON g.appid = ps.appid
		WHERE ps.current > 0 AND g.name != ''
		ORDER BY ps.current DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []HomeItem
	for rows.Next() {
		var item HomeItem
		var current int
		if err := rows.Scan(&item.AppID, &item.Name, &current); err != nil {
			continue
		}
		item.Header, item.PageURL, item.SteamURL = homeURLs(item.AppID)
		item.Meta = CommaInt(current) + " in-game"
		out = append(out, item)
	}
	_ = rows.Err()
	return out
}

func (db *DB) HomeOnSale(limit int) []HomeItem {
	if limit <= 0 {
		limit = 12
	}
	rows, err := db.sql.Query(`
		SELECT g.appid, g.name, pl.discount, pl.formatted, pl.final
		FROM price_latest pl
		JOIN games g ON g.appid = pl.appid
		JOIN reviews r ON r.appid = g.appid
		WHERE pl.discount >= 40
		  AND pl.final > 0
		  AND g.name != ''
		  AND r.total >= 100
		ORDER BY pl.discount DESC, r.total DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []HomeItem
	for rows.Next() {
		var item HomeItem
		var final int
		var formatted string
		if err := rows.Scan(&item.AppID, &item.Name, &item.Discount, &formatted, &final); err != nil {
			continue
		}
		item.Formatted = formatted
		item.Header, item.PageURL, item.SteamURL = homeURLs(item.AppID)
		if formatted != "" {
			item.Meta = strconv.Itoa(item.Discount) + "% off · " + formatted
		} else {
			item.Meta = strconv.Itoa(item.Discount) + "% off"
		}
		out = append(out, item)
	}
	_ = rows.Err()
	return out
}

func (db *DB) HomeRails(limit int) HomeRails {
	if limit <= 0 {
		limit = 12
	}
	return HomeRails{
		Hot:            db.HomeHot(limit),
		NewWithPlayers: db.HomeNewWithPlayers(limit),
		OnSale:         db.HomeOnSale(limit),
		TopPlayers:     db.HomeTopPlayers(limit),
	}
}
