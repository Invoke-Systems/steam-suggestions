package store

import (
	"fmt"
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
	NewWithPlayers []HomeItem `json:"newWithPlayers"`
	RisingReviews  []HomeItem `json:"risingReviews"`
	OnSale         []HomeItem `json:"onSale"`
	TopPlayers     []HomeItem `json:"topPlayers"`
}

func homeURLs(appid int) (header, page, steam string) {
	header = fmt.Sprintf("https://cdn.akamai.steamstatic.com/steam/apps/%d/header.jpg", appid)
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

// HomeNewWithPlayers: newer Steam titles that currently have many players.
func (db *DB) HomeNewWithPlayers(limit int) []HomeItem {
	if limit <= 0 {
		limit = 12
	}
	rows, err := db.sql.Query(`
		SELECT g.appid, g.name, ps.current, COALESCE(r.positive, 0), COALESCE(r.total, 0)
		FROM player_stats ps
		JOIN games g ON g.appid = ps.appid
		LEFT JOIN reviews r ON r.appid = g.appid
		WHERE ps.current >= 50 AND g.name != ''
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
		var current, positive, total int
		if err := rows.Scan(&item.AppID, &item.Name, &current, &positive, &total); err != nil {
			continue
		}
		item.Header, item.PageURL, item.SteamURL = homeURLs(item.AppID)
		item.Meta = CommaInt(current) + " in-game"
		if positive > 0 && total > 0 {
			item.Meta += " · " + strconv.Itoa(positive) + "% of " + CommaInt(total)
		}
		out = append(out, item)
	}
	_ = rows.Err()
	return out
}

// HomeRisingReviews: newer games with excellent review scores (lifetime % —
// Steam does not expose a stored recent-vs-overall split in our crawl yet).
func (db *DB) HomeRisingReviews(limit int) []HomeItem {
	if limit <= 0 {
		limit = 12
	}
	rows, err := db.sql.Query(`
		SELECT g.appid, g.name, r.positive, r.total
		FROM reviews r
		JOIN games g ON g.appid = r.appid
		WHERE g.name != ''
		  AND r.positive >= 90
		  AND r.total >= 500
		  AND r.total <= 80000
		ORDER BY g.appid DESC, r.positive DESC, r.total DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []HomeItem
	for rows.Next() {
		var item HomeItem
		var positive, total int
		if err := rows.Scan(&item.AppID, &item.Name, &positive, &total); err != nil {
			continue
		}
		item.Header, item.PageURL, item.SteamURL = homeURLs(item.AppID)
		item.Meta = strconv.Itoa(positive) + "% of " + CommaInt(total) + " reviews"
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
		NewWithPlayers: db.HomeNewWithPlayers(limit),
		RisingReviews:  db.HomeRisingReviews(limit),
		OnSale:         db.HomeOnSale(limit),
		TopPlayers:     db.HomeTopPlayers(limit),
	}
}
