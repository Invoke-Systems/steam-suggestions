package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"time"
)

const sessionTTL = 30 * 24 * time.Hour

type User struct {
	SteamID     string
	Name        string
	Avatar      string
	CreatedAt   int64
	LastLoginAt int64
}

func HashSessionToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func NewSessionToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func (db *DB) UpsertUser(user User) error {
	now := time.Now().Unix()
	if user.CreatedAt == 0 {
		user.CreatedAt = now
	}
	if user.LastLoginAt == 0 {
		user.LastLoginAt = now
	}
	_, err := db.sql.Exec(`
		INSERT INTO users (steamid, name, avatar, created_at, last_login_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(steamid) DO UPDATE SET
		  name = CASE WHEN excluded.name != '' THEN excluded.name ELSE users.name END,
		  avatar = CASE WHEN excluded.avatar != '' THEN excluded.avatar ELSE users.avatar END,
		  last_login_at = excluded.last_login_at
	`, user.SteamID, user.Name, user.Avatar, user.CreatedAt, user.LastLoginAt)
	return err
}

func (db *DB) GetUser(steamid string) (User, bool) {
	var user User
	err := db.sql.QueryRow(
		`SELECT steamid, name, avatar, created_at, last_login_at FROM users WHERE steamid = ?`,
		steamid,
	).Scan(&user.SteamID, &user.Name, &user.Avatar, &user.CreatedAt, &user.LastLoginAt)
	if err != nil {
		return User{}, false
	}
	return user, true
}

func (db *DB) CreateSession(steamid string) (token string, err error) {
	token, err = NewSessionToken()
	if err != nil {
		return "", err
	}
	now := time.Now().Unix()
	_, err = db.sql.Exec(
		`INSERT INTO sessions (token_hash, steamid, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		HashSessionToken(token), steamid, now+int64(sessionTTL.Seconds()), now,
	)
	if err != nil {
		return "", err
	}
	_, _ = db.sql.Exec(`DELETE FROM sessions WHERE expires_at < ?`, now)
	return token, nil
}

func (db *DB) SessionUser(token string) (User, bool) {
	if token == "" {
		return User{}, false
	}
	var steamid string
	err := db.sql.QueryRow(
		`SELECT steamid FROM sessions WHERE token_hash = ? AND expires_at > ?`,
		HashSessionToken(token), time.Now().Unix(),
	).Scan(&steamid)
	if err != nil {
		if err != sql.ErrNoRows {
			return User{}, false
		}
		return User{}, false
	}
	return db.GetUser(steamid)
}

func (db *DB) DeleteSession(token string) {
	if token == "" {
		return
	}
	_, _ = db.sql.Exec(`DELETE FROM sessions WHERE token_hash = ?`, HashSessionToken(token))
}
