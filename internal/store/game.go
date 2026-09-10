package store

import (
	"strings"
	"time"
)

type GameRow struct {
	AppID  int
	Name   string
	TagsOK bool
}

func (db *DB) GetGame(appid int) (GameRow, bool) {
	if appid <= 0 {
		return GameRow{}, false
	}
	var row GameRow
	var tagsOK int
	err := db.sql.QueryRow(`
		SELECT appid, name, COALESCE(tags_ok, 0) FROM games WHERE appid = ?
	`, appid).Scan(&row.AppID, &row.Name, &tagsOK)
	if err != nil {
		return GameRow{}, false
	}
	row.TagsOK = tagsOK == 1
	return row, true
}

func (db *DB) GetGameNames(ids []int) map[int]string {
	out := map[int]string{}
	if len(ids) == 0 {
		return out
	}
	for _, chunk := range chunks(ids, 400) {
		q := `SELECT appid, name FROM games WHERE appid IN (` + placeholders(len(chunk)) + `)`
		rows, err := db.sql.Query(q, asArgs(chunk)...)
		if err != nil {
			continue
		}
		for rows.Next() {
			var appid int
			var name string
			if err := rows.Scan(&appid, &name); err != nil {
				continue
			}
			out[appid] = name
		}
		_ = rows.Err()
		rows.Close()
	}
	return out
}

func (db *DB) SearchGames(query string, limit int) []GameRow {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	if limit <= 0 {
		limit = 12
	}
	if len(query) > 80 {
		query = query[:80]
	}
	like := "%" + query + "%"
	prefix := query + "%"
	rows, err := db.sql.Query(`
		SELECT g.appid, g.name, COALESCE(g.tags_ok, 0)
		FROM games g
		LEFT JOIN reviews r ON r.appid = g.appid
		WHERE g.name != ''
		  AND g.name LIKE ? COLLATE NOCASE
		ORDER BY
		  CASE
		    WHEN g.name LIKE ? COLLATE NOCASE THEN 0
		    WHEN g.name LIKE ? COLLATE NOCASE THEN 1
		    ELSE 2
		  END,
		  COALESCE(r.total, 0) DESC,
		  length(g.name),
		  g.name
		LIMIT ?
	`, like, prefix, "% "+query+"%", limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []GameRow
	for rows.Next() {
		var row GameRow
		var tagsOK int
		if err := rows.Scan(&row.AppID, &row.Name, &tagsOK); err != nil {
			continue
		}
		row.TagsOK = tagsOK == 1
		out = append(out, row)
	}
	_ = rows.Err()
	return out
}

func (db *DB) GetAppNews(appid, limit int) []NewsItem {
	if appid <= 0 {
		return nil
	}
	if limit <= 0 {
		limit = 8
	}
	rows, err := db.sql.Query(`
		SELECT appid, gid, title, url, author, feedlabel, date, contents
		FROM app_news WHERE appid = ?
		ORDER BY date DESC
		LIMIT ?
	`, appid, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []NewsItem
	for rows.Next() {
		var item NewsItem
		if err := rows.Scan(
			&item.AppID, &item.GID, &item.Title, &item.URL, &item.Author,
			&item.FeedLabel, &item.Date, &item.Contents,
		); err != nil {
			continue
		}
		out = append(out, item)
	}
	_ = rows.Err()
	return out
}

type PlayerPoint struct {
	Hour    int64
	Current int
}

func (db *DB) GetPlayerHistory(appid int, keepDays int) []PlayerPoint {
	if appid <= 0 {
		return nil
	}
	if keepDays <= 0 {
		keepDays = 14
	}
	cutoff := time.Now().Unix()/3600 - int64(keepDays*24)
	rows, err := db.sql.Query(`
		SELECT hour, current FROM player_history
		WHERE appid = ? AND hour >= ?
		ORDER BY hour
	`, appid, cutoff)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []PlayerPoint
	for rows.Next() {
		var p PlayerPoint
		if err := rows.Scan(&p.Hour, &p.Current); err != nil {
			continue
		}
		out = append(out, p)
	}
	_ = rows.Err()
	return out
}

type PricePoint struct {
	Day   int64
	Final int
}

func (db *DB) GetPriceHistory(appid int, maxDays int) []PricePoint {
	if appid <= 0 {
		return nil
	}
	if maxDays <= 0 {
		maxDays = 730
	}
	rows, err := db.sql.Query(`
		SELECT day, final FROM price_history
		WHERE appid = ? AND final > 0
		ORDER BY day DESC
		LIMIT ?
	`, appid, maxDays)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []PricePoint
	for rows.Next() {
		var p PricePoint
		if err := rows.Scan(&p.Day, &p.Final); err != nil {
			continue
		}
		out = append(out, p)
	}
	_ = rows.Err()
	// chronological
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
