package store

import (
	"time"
)

type PlayerStats struct {
	AppID     int
	Current   int
	PeakDay   int
	PeakAll   int
	SampledAt int64
}

func (db *DB) SetPlayerSample(appid, current int) error {
	if appid <= 0 || current < 0 {
		return nil
	}
	now := time.Now().Unix()
	hour := now / 3600
	dayStart := now - 24*3600
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
		INSERT INTO player_history (appid, hour, current) VALUES (?, ?, ?)
		ON CONFLICT(appid, hour) DO UPDATE SET current = MAX(player_history.current, excluded.current)
	`, appid, hour, current); err != nil {
		return err
	}
	var peakDay, peakAll int
	_ = tx.QueryRow(
		`SELECT COALESCE(MAX(current), 0) FROM player_history WHERE appid = ? AND hour >= ?`,
		appid, dayStart/3600,
	).Scan(&peakDay)
	if peakDay < current {
		peakDay = current
	}
	_ = tx.QueryRow(
		`SELECT COALESCE(MAX(peak_all), 0) FROM player_stats WHERE appid = ?`,
		appid,
	).Scan(&peakAll)
	if current > peakAll {
		peakAll = current
	}
	if peakDay > peakAll {
		peakAll = peakDay
	}
	if _, err := tx.Exec(`
		INSERT INTO player_stats (appid, current, peak_day, peak_all, sampled_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(appid) DO UPDATE SET
		  current = excluded.current,
		  peak_day = excluded.peak_day,
		  peak_all = MAX(player_stats.peak_all, excluded.peak_all),
		  sampled_at = excluded.sampled_at
	`, appid, current, peakDay, peakAll, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) GetPlayerStats(ids []int) map[int]PlayerStats {
	out := map[int]PlayerStats{}
	if len(ids) == 0 {
		return out
	}
	for _, chunk := range chunks(ids, 200) {
		q := `SELECT appid, current, peak_day, peak_all, sampled_at FROM player_stats WHERE appid IN (` + placeholders(len(chunk)) + `)`
		rows, err := db.sql.Query(q, asArgs(chunk)...)
		if err != nil {
			continue
		}
		for rows.Next() {
			var row PlayerStats
			if err := rows.Scan(&row.AppID, &row.Current, &row.PeakDay, &row.PeakAll, &row.SampledAt); err != nil {
				continue
			}
			out[row.AppID] = row
		}
		_ = rows.Err()
		rows.Close()
	}
	return out
}

func (db *DB) NeedPlayerWork(limit int) ([]int, error) {
	if limit <= 0 {
		limit = 40
	}
	stale := time.Now().Unix() - 3*3600
	rows, err := db.sql.Query(`
		SELECT r.appid FROM reviews r
		LEFT JOIN player_stats p ON p.appid = r.appid
		WHERE r.total >= 500 AND (p.sampled_at IS NULL OR p.sampled_at < ?)
		ORDER BY r.total DESC
		LIMIT ?
	`, stale, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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

func (db *DB) PrunePlayerHistory(keepDays int) error {
	if keepDays <= 0 {
		keepDays = 14
	}
	cutoff := time.Now().Unix()/3600 - int64(keepDays*24)
	_, err := db.sql.Exec(`DELETE FROM player_history WHERE hour < ?`, cutoff)
	return err
}

func (db *DB) PrunePriceHistory(keepDays int) error {
	if keepDays <= 0 {
		keepDays = 730
	}
	cutoffDay := time.Now().UnixMilli()/86400000 - int64(keepDays)
	_, err := db.sql.Exec(`DELETE FROM price_history WHERE day < ?`, cutoffDay)
	return err
}
