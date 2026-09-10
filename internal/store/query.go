package store

import (
	"fmt"
	"strings"
	"time"
)

type NamedTag struct {
	TagID  int    `json:"tagid"`
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

type PriceView struct {
	AppID     int     `json:"appid"`
	T         int64   `json:"t"`
	Currency  string  `json:"currency"`
	Initial   int     `json:"initial"`
	Final     int     `json:"final"`
	Discount  int     `json:"discount"`
	Formatted string  `json:"formatted"`
	OnSale    bool    `json:"onSale"`
	Low       *string `json:"low"`
	AtLow     bool    `json:"atLow"`
}

type Review struct {
	AppID      int   `json:"appid"`
	T          int64 `json:"t"`
	Total      int   `json:"total"`
	Popularity int   `json:"popularity"`
	Positive   int   `json:"positive"`
}

func (db *DB) UpsertGame(appid int, name string) error {
	_, err := db.sql.Exec(`
		INSERT INTO games (appid, name) VALUES (?, ?)
		ON CONFLICT(appid) DO UPDATE SET name = CASE
		  WHEN excluded.name != '' THEN excluded.name ELSE games.name END
	`, appid, name)
	return err
}

func (db *DB) StaleAppIDs(ids []int, maxAgeMs int64) []int {
	cutoff := time.Now().UnixMilli() - maxAgeMs
	var missing []int
	seen := map[int]bool{}
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		_ = db.UpsertGame(id, "")
		var fetched int64
		err := db.sql.QueryRow(
			`SELECT tags_fetched_at FROM games WHERE appid = ? AND tags_fetched_at IS NOT NULL AND tags_fetched_at > ?`,
			id, cutoff,
		).Scan(&fetched)
		if err != nil {
			missing = append(missing, id)
		}
	}
	return missing
}

func FormatCents(cents int, currency string) *string {
	if cents <= 0 {
		return nil
	}
	amount := fmt.Sprintf("%.2f", float64(cents)/100)
	text := "$" + amount
	if currency != "" && currency != "USD" {
		text = amount + " " + currency
	}
	return &text
}

func (db *DB) SetReviews(appid, total, positive, popularity int) error {
	_, err := db.sql.Exec(`
		INSERT INTO reviews (appid, t, total, positive, popularity) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(appid) DO UPDATE SET
		  t = excluded.t, total = excluded.total, positive = excluded.positive, popularity = excluded.popularity
	`, appid, time.Now().UnixMilli(), total, positive, popularity)
	return err
}

func chunks(ids []int, size int) [][]int {
	if size <= 0 {
		size = 400
	}
	var out [][]int
	var chunk []int
	seen := map[int]bool{}
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		chunk = append(chunk, id)
		if len(chunk) >= size {
			out = append(out, chunk)
			chunk = nil
		}
	}
	if len(chunk) > 0 {
		out = append(out, chunk)
	}
	return out
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	b := strings.Builder{}
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('?')
	}
	return b.String()
}

func asArgs(ids []int) []any {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return args
}

func (db *DB) GetTags(ids []int) map[int][]NamedTag {
	out := make(map[int][]NamedTag, len(ids))
	for _, id := range ids {
		if id != 0 {
			out[id] = []NamedTag{}
		}
	}
	for _, chunk := range chunks(ids, 400) {
		q := `
			SELECT gt.appid, gt.tagid, gt.weight, COALESCE(td.name, 'Tag ' || gt.tagid)
			FROM game_tags gt
			LEFT JOIN tag_dict td ON td.tagid = gt.tagid
			WHERE gt.appid IN (` + placeholders(len(chunk)) + `)
			ORDER BY gt.appid, gt.weight DESC
		`
		rows, err := db.sql.Query(q, asArgs(chunk)...)
		if err != nil {
			continue
		}
		for rows.Next() {
			var appid int
			var tag NamedTag
			if err := rows.Scan(&appid, &tag.TagID, &tag.Weight, &tag.Name); err != nil {
				continue
			}
			out[appid] = append(out[appid], tag)
		}
		_ = rows.Err()
		rows.Close()
	}
	return out
}

func (db *DB) GetPrices(ids []int) map[int]PriceView {
	out := make(map[int]PriceView, len(ids))
	histLow := map[int]int{}
	for _, chunk := range chunks(ids, 400) {
		q := `
			SELECT
			  pl.appid, pl.t, pl.currency, pl.initial, pl.final, pl.discount, pl.formatted,
			  (SELECT MIN(ph.final) FROM price_history ph WHERE ph.appid = pl.appid AND ph.final > 0) AS low_final,
			  (SELECT MAX(ph.final) FROM price_history ph WHERE ph.appid = pl.appid AND ph.final > 0) AS high_final
			FROM price_latest pl
			WHERE pl.appid IN (` + placeholders(len(chunk)) + `)
		`
		rows, err := db.sql.Query(q, asArgs(chunk)...)
		if err != nil {
			continue
		}
		for rows.Next() {
			var view PriceView
			var lowFinal, highFinal *int
			if err := rows.Scan(
				&view.AppID, &view.T, &view.Currency, &view.Initial, &view.Final, &view.Discount, &view.Formatted,
				&lowFinal, &highFinal,
			); err != nil {
				continue
			}
			view.OnSale = view.Discount > 0
			if lowFinal != nil && *lowFinal > 0 {
				histLow[view.AppID] = *lowFinal
			}
			_ = highFinal
			out[view.AppID] = view
		}
		_ = rows.Err()
		rows.Close()
	}
	itadLows := db.GetPriceLows(ids)
	for id, view := range out {
		lowCents := histLow[id]
		if seed, ok := itadLows[id]; ok && seed.Cents > 0 {
			if lowCents <= 0 || seed.Cents < lowCents {
				lowCents = seed.Cents
			}
			if view.Currency == "" && seed.Currency != "" {
				view.Currency = seed.Currency
			}
		}
		if lowCents > 0 {
			view.Low = FormatCents(lowCents, view.Currency)
			view.AtLow = view.Discount > 0 && view.Final <= lowCents
		}
		out[id] = view
	}
	return out
}

func (db *DB) GetReviews(ids []int) map[int]Review {
	out := make(map[int]Review, len(ids))
	for _, chunk := range chunks(ids, 400) {
		q := `SELECT appid, t, total, popularity, COALESCE(positive, 0) FROM reviews WHERE appid IN (` + placeholders(len(chunk)) + `)`
		rows, err := db.sql.Query(q, asArgs(chunk)...)
		if err != nil {
			continue
		}
		for rows.Next() {
			var row Review
			if err := rows.Scan(&row.AppID, &row.T, &row.Total, &row.Popularity, &row.Positive); err != nil {
				continue
			}
			out[row.AppID] = row
		}
		_ = rows.Err()
		rows.Close()
	}
	return out
}

func (db *DB) PriceSparklines(ids []int, maxPoints int) map[int][]int {
	if maxPoints <= 0 {
		maxPoints = 30
	}
	out := make(map[int][]int, len(ids))
	for _, chunk := range chunks(ids, 200) {
		q := `
			SELECT appid, day, final FROM price_history
			WHERE appid IN (` + placeholders(len(chunk)) + `) AND final > 0
			ORDER BY appid, day
		`
		rows, err := db.sql.Query(q, asArgs(chunk)...)
		if err != nil {
			continue
		}
		for rows.Next() {
			var appid, day, final int
			if err := rows.Scan(&appid, &day, &final); err != nil {
				continue
			}
			out[appid] = append(out[appid], final)
		}
		_ = rows.Err()
		rows.Close()
	}
	for id, series := range out {
		if len(series) > maxPoints {
			out[id] = series[len(series)-maxPoints:]
		}
		if len(out[id]) < 2 {
			delete(out, id)
		}
	}
	return out
}

func (db *DB) TagIDsByNames(names []string) []int {
	var ids []int
	seen := map[int]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		var id int
		err := db.sql.QueryRow(`SELECT tagid FROM tag_dict WHERE name = ? COLLATE NOCASE`, name).Scan(&id)
		if err != nil || id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

func (db *DB) SearchTags(query string, limit int) []DictTag {
	if limit <= 0 {
		limit = 12
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	rows, err := db.sql.Query(`
		SELECT tagid, name FROM tag_dict
		WHERE name LIKE ? COLLATE NOCASE
		ORDER BY CASE WHEN name LIKE ? THEN 0 ELSE 1 END, length(name), name
		LIMIT ?
	`, "%"+query+"%", query+"%", limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []DictTag
	for rows.Next() {
		var tag DictTag
		if err := rows.Scan(&tag.TagID, &tag.Name); err != nil {
			continue
		}
		out = append(out, tag)
	}
	_ = rows.Err()
	return out
}
