package store

import (
	"encoding/json"
	"strings"
	"time"
)

const maxSavedSearches = 25

type SavedSearch struct {
	ID        int             `json:"id"`
	Name      string          `json:"name"`
	Terms     string          `json:"terms"`
	Kind      string          `json:"kind"`
	Filters   json.RawMessage `json:"filters"`
	CreatedAt int64           `json:"createdAt"`
	UpdatedAt int64           `json:"updatedAt"`
}

func NormalizeSearchKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "library":
		return "library"
	default:
		return "recs"
	}
}

func (db *DB) ListSavedSearches(steamid string) ([]SavedSearch, error) {
	rows, err := db.sql.Query(`
		SELECT id, name, terms, kind, filters, created_at, updated_at
		FROM saved_searches
		WHERE steamid = ?
		ORDER BY updated_at DESC, id DESC
	`, steamid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SavedSearch{}
	for rows.Next() {
		var item SavedSearch
		var raw string
		if err := rows.Scan(&item.ID, &item.Name, &item.Terms, &item.Kind, &raw, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Filters = json.RawMessage(raw)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (db *DB) CreateSavedSearch(steamid, name, terms, kind string, filters json.RawMessage) (SavedSearch, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return SavedSearch{}, errSearch("Name a saved search.")
	}
	if len(name) > 60 {
		name = name[:60]
	}
	terms = strings.Join(strings.Fields(strings.ReplaceAll(terms, ",", ", ")), " ")
	terms = strings.TrimSpace(terms)
	if len(terms) > 200 {
		terms = terms[:200]
	}
	kind = NormalizeSearchKind(kind)
	if !json.Valid(filters) || len(filters) == 0 || filters[0] != '{' {
		return SavedSearch{}, errSearch("Filters must be a JSON object.")
	}
	var count int
	if err := db.sql.QueryRow(`SELECT COUNT(*) FROM saved_searches WHERE steamid = ?`, steamid).Scan(&count); err != nil {
		return SavedSearch{}, err
	}
	if count >= maxSavedSearches {
		return SavedSearch{}, errSearch("You already have 25 saved searches.")
	}
	now := time.Now().Unix()
	res, err := db.sql.Exec(
		`INSERT INTO saved_searches (steamid, name, terms, kind, filters, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		steamid, name, terms, kind, string(filters), now, now,
	)
	if err != nil {
		return SavedSearch{}, err
	}
	id, _ := res.LastInsertId()
	return SavedSearch{ID: int(id), Name: name, Terms: terms, Kind: kind, Filters: filters, CreatedAt: now, UpdatedAt: now}, nil
}

func (db *DB) DeleteSavedSearch(steamid string, id int) bool {
	res, err := db.sql.Exec(`DELETE FROM saved_searches WHERE id = ? AND steamid = ?`, id, steamid)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}

type searchError string

func (e searchError) Error() string { return string(e) }

func errSearch(msg string) error { return searchError(msg) }

func IsSearchError(err error) bool {
	_, ok := err.(searchError)
	return ok
}
