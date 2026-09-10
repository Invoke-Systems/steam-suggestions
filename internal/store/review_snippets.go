package store

import (
	"time"
)

type ReviewSnippet struct {
	AppID     int
	ReviewID  string
	SteamID   string
	VotedUp   bool
	Hours     int
	CreatedAt int64
	Text      string
	URL       string
}

func (db *DB) SetReviewSnippets(appid int, items []ReviewSnippet) error {
	if appid <= 0 {
		return nil
	}
	now := time.Now().Unix()
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM app_review_snippets WHERE appid = ?`, appid); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		INSERT INTO app_review_snippets
			(appid, review_id, steamid, voted_up, hours, created_at, body, url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, item := range items {
		id := item.ReviewID
		if id == "" {
			continue
		}
		voted := 0
		if item.VotedUp {
			voted = 1
		}
		if _, err := stmt.Exec(appid, id, item.SteamID, voted, item.Hours, item.CreatedAt, item.Text, item.URL); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`
		INSERT INTO app_review_snippets_meta (appid, fetched_at) VALUES (?, ?)
		ON CONFLICT(appid) DO UPDATE SET fetched_at = excluded.fetched_at
	`, appid, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) GetReviewSnippets(appid, limit int) ([]ReviewSnippet, int64, bool) {
	if appid <= 0 {
		return nil, 0, false
	}
	if limit <= 0 {
		limit = 5
	}
	var fetchedAt int64
	err := db.sql.QueryRow(`SELECT fetched_at FROM app_review_snippets_meta WHERE appid = ?`, appid).Scan(&fetchedAt)
	if err != nil {
		return nil, 0, false
	}
	rows, err := db.sql.Query(`
		SELECT review_id, steamid, voted_up, hours, created_at, body, url
		FROM app_review_snippets
		WHERE appid = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, appid, limit)
	if err != nil {
		return nil, fetchedAt, false
	}
	defer rows.Close()
	out := make([]ReviewSnippet, 0, limit)
	for rows.Next() {
		var item ReviewSnippet
		var voted int
		if err := rows.Scan(&item.ReviewID, &item.SteamID, &voted, &item.Hours, &item.CreatedAt, &item.Text, &item.URL); err != nil {
			return nil, fetchedAt, false
		}
		item.AppID = appid
		item.VotedUp = voted == 1
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fetchedAt, false
	}
	return out, fetchedAt, true
}
