package store

import (
	"fmt"
	"time"
)

const (
	focusTTL      = 24 * time.Hour
	maxFocusRows  = 4000
	maxSteamIDLen = 17
)

func ValidSteamID(id string) bool {
	if len(id) != maxSteamIDLen {
		return false
	}
	for i := 0; i < len(id); i++ {
		if id[i] < '0' || id[i] > '9' {
			return false
		}
	}
	return true
}

func (db *DB) CreateLibraryFocus(steamid string) (token string, err error) {
	if !ValidSteamID(steamid) {
		return "", fmt.Errorf("invalid steamid")
	}
	token, err = NewSessionToken()
	if err != nil {
		return "", err
	}
	now := time.Now().Unix()
	_, err = db.sql.Exec(
		`INSERT INTO library_focus (token_hash, steamid, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		HashSessionToken(token), steamid, now+int64(focusTTL.Seconds()), now,
	)
	if err != nil {
		return "", err
	}
	_, _ = db.sql.Exec(`DELETE FROM library_focus WHERE expires_at < ?`, now)
	var n int
	_ = db.sql.QueryRow(`SELECT COUNT(*) FROM library_focus`).Scan(&n)
	if n > maxFocusRows {
		_, _ = db.sql.Exec(
			`DELETE FROM library_focus WHERE token_hash IN (
				SELECT token_hash FROM library_focus ORDER BY created_at ASC LIMIT ?
			)`, n-maxFocusRows,
		)
	}
	return token, nil
}

func (db *DB) LibraryFocusSteamID(token string) (string, bool) {
	if token == "" {
		return "", false
	}
	var steamid string
	err := db.sql.QueryRow(
		`SELECT steamid FROM library_focus WHERE token_hash = ? AND expires_at > ?`,
		HashSessionToken(token), time.Now().Unix(),
	).Scan(&steamid)
	if err != nil || !ValidSteamID(steamid) {
		return "", false
	}
	return steamid, true
}

func (db *DB) DeleteLibraryFocus(token string) {
	if token == "" {
		return
	}
	_, _ = db.sql.Exec(`DELETE FROM library_focus WHERE token_hash = ?`, HashSessionToken(token))
}
