package store

import (
	"encoding/json"
	"time"
)

type AchievementPct struct {
	Name    string  `json:"name"`
	Percent float64 `json:"percent"`
}

func (db *DB) SetAchievements(appid int, items []AchievementPct) error {
	if appid <= 0 {
		return nil
	}
	if items == nil {
		items = []AchievementPct{}
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return err
	}
	_, err = db.sql.Exec(`
		INSERT INTO app_achievements (appid, payload, fetched_at)
		VALUES (?, ?, ?)
		ON CONFLICT(appid) DO UPDATE SET
		  payload = excluded.payload,
		  fetched_at = excluded.fetched_at
	`, appid, string(raw), time.Now().Unix())
	return err
}

func (db *DB) GetAchievements(appid int) ([]AchievementPct, bool) {
	var raw string
	err := db.sql.QueryRow(`SELECT payload FROM app_achievements WHERE appid = ?`, appid).Scan(&raw)
	if err != nil {
		return nil, false
	}
	var items []AchievementPct
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, false
	}
	return items, true
}

func (db *DB) NeedAchievementsWork(limit int, staleSec int64) ([]int, error) {
	if limit <= 0 {
		limit = 40
	}
	if staleSec <= 0 {
		staleSec = 7 * 24 * 3600
	}
	cutoff := time.Now().Unix() - staleSec
	rows, err := db.sql.Query(`
		SELECT g.appid FROM games g
		LEFT JOIN app_achievements a ON a.appid = g.appid
		LEFT JOIN reviews r ON r.appid = g.appid
		WHERE g.tags_ok = 1 AND COALESCE(r.total, 0) >= 500
		  AND (a.appid IS NULL OR a.fetched_at < ?)
		ORDER BY COALESCE(r.total, 0) DESC, g.appid
		LIMIT ?
	`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIDs(rows)
}
