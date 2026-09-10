package store

import (
	"time"
)

type NewsItem struct {
	AppID     int
	GID       string
	Title     string
	URL       string
	Author    string
	FeedLabel string
	Date      int64
	Contents  string
}

func (db *DB) SetAppNews(appid int, items []NewsItem) error {
	if appid <= 0 {
		return nil
	}
	now := time.Now().Unix()
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM app_news WHERE appid = ?`, appid); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		INSERT INTO app_news (appid, gid, title, url, author, feedlabel, date, contents)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, item := range items {
		gid := item.GID
		if gid == "" {
			continue
		}
		if _, err := stmt.Exec(appid, gid, item.Title, item.URL, item.Author, item.FeedLabel, item.Date, item.Contents); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`
		INSERT INTO app_news_meta (appid, fetched_at) VALUES (?, ?)
		ON CONFLICT(appid) DO UPDATE SET fetched_at = excluded.fetched_at
	`, appid, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) NeedNewsWork(limit int, staleSec int64) ([]int, error) {
	if limit <= 0 {
		limit = 40
	}
	if staleSec <= 0 {
		staleSec = 24 * 3600
	}
	cutoff := time.Now().Unix() - staleSec
	rows, err := db.sql.Query(`
		SELECT g.appid FROM games g
		LEFT JOIN app_news_meta n ON n.appid = g.appid
		LEFT JOIN reviews r ON r.appid = g.appid
		WHERE g.tags_ok = 1 AND COALESCE(r.total, 0) >= 100
		  AND (n.appid IS NULL OR n.fetched_at < ?)
		ORDER BY COALESCE(r.total, 0) DESC, g.appid
		LIMIT ?
	`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIDs(rows)
}
