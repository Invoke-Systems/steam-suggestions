package store

import (
	"database/sql"
	"encoding/json"
	"time"
)

type AppDetails struct {
	AppID            int
	Type             string
	Developers       []string
	Publishers       []string
	Genres           []string
	Categories       []string
	ReleaseDate      string
	Metacritic       *int
	ShortDescription string
	Missing          bool
	FetchedAt        int64
}

func (db *DB) SetAppDetails(d AppDetails) error {
	if d.AppID <= 0 {
		return nil
	}
	now := d.FetchedAt
	if now <= 0 {
		now = time.Now().Unix()
	}
	devs, _ := json.Marshal(nonNilStrings(d.Developers))
	pubs, _ := json.Marshal(nonNilStrings(d.Publishers))
	genres, _ := json.Marshal(nonNilStrings(d.Genres))
	cats, _ := json.Marshal(nonNilStrings(d.Categories))
	missing := 0
	if d.Missing {
		missing = 1
	}
	var meta any
	if d.Metacritic != nil {
		meta = *d.Metacritic
	}
	_, err := db.sql.Exec(`
		INSERT INTO app_details (
		  appid, type, developers, publishers, genres, categories,
		  release_date, metacritic, short_description, missing, fetched_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(appid) DO UPDATE SET
		  type = excluded.type,
		  developers = excluded.developers,
		  publishers = excluded.publishers,
		  genres = excluded.genres,
		  categories = excluded.categories,
		  release_date = excluded.release_date,
		  metacritic = excluded.metacritic,
		  short_description = excluded.short_description,
		  missing = excluded.missing,
		  fetched_at = excluded.fetched_at
	`, d.AppID, d.Type, string(devs), string(pubs), string(genres), string(cats),
		d.ReleaseDate, meta, d.ShortDescription, missing, now)
	return err
}

func (db *DB) GetAppDetails(ids []int) map[int]AppDetails {
	out := map[int]AppDetails{}
	if len(ids) == 0 {
		return out
	}
	for _, chunk := range chunks(ids, 200) {
		q := `SELECT appid, type, developers, publishers, genres, categories,
		             release_date, metacritic, short_description, missing, fetched_at
		      FROM app_details WHERE appid IN (` + placeholders(len(chunk)) + `)`
		rows, err := db.sql.Query(q, asArgs(chunk)...)
		if err != nil {
			continue
		}
		for rows.Next() {
			var row AppDetails
			var devs, pubs, genres, cats string
			var meta sql.NullInt64
			var missing int
			if err := rows.Scan(
				&row.AppID, &row.Type, &devs, &pubs, &genres, &cats,
				&row.ReleaseDate, &meta, &row.ShortDescription, &missing, &row.FetchedAt,
			); err != nil {
				continue
			}
			row.Developers = decodeStrings(devs)
			row.Publishers = decodeStrings(pubs)
			row.Genres = decodeStrings(genres)
			row.Categories = decodeStrings(cats)
			row.Missing = missing == 1
			if meta.Valid {
				v := int(meta.Int64)
				row.Metacritic = &v
			}
			out[row.AppID] = row
		}
		_ = rows.Err()
		rows.Close()
	}
	return out
}

func (db *DB) NeedDetailsWork(limit int, staleSec int64) ([]int, error) {
	if limit <= 0 {
		limit = 40
	}
	if staleSec <= 0 {
		staleSec = 14 * 24 * 3600
	}
	cutoff := time.Now().Unix() - staleSec
	rows, err := db.sql.Query(`
		SELECT g.appid FROM games g
		LEFT JOIN app_details d ON d.appid = g.appid
		LEFT JOIN reviews r ON r.appid = g.appid
		WHERE g.tags_ok = 1 AND (d.appid IS NULL OR d.fetched_at < ?)
		ORDER BY COALESCE(r.total, 0) DESC, g.appid
		LIMIT ?
	`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIDs(rows)
}

func nonNilStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func decodeStrings(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func scanIDs(rows *sql.Rows) ([]int, error) {
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
