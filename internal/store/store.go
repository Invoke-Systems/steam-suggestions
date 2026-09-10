package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS meta (
  key TEXT PRIMARY KEY,
  value TEXT
);
CREATE TABLE IF NOT EXISTS tag_dict (
  tagid INTEGER PRIMARY KEY,
  name TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS games (
  appid INTEGER PRIMARY KEY,
  name TEXT NOT NULL DEFAULT '',
  tags_fetched_at INTEGER,
  tags_ok INTEGER NOT NULL DEFAULT 0,
  price_fetched_at INTEGER,
  price_change_number INTEGER
);
CREATE TABLE IF NOT EXISTS game_tags (
  appid INTEGER NOT NULL,
  tagid INTEGER NOT NULL,
  weight INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (appid, tagid)
);
CREATE INDEX IF NOT EXISTS idx_game_tags_tagid ON game_tags(tagid);
CREATE TABLE IF NOT EXISTS price_latest (
  appid INTEGER PRIMARY KEY,
  t INTEGER NOT NULL,
  currency TEXT NOT NULL DEFAULT '',
  initial INTEGER NOT NULL DEFAULT 0,
  final INTEGER NOT NULL DEFAULT 0,
  discount INTEGER NOT NULL DEFAULT 0,
  formatted TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS price_history (
  appid INTEGER NOT NULL,
  day INTEGER NOT NULL,
  t INTEGER NOT NULL,
  currency TEXT NOT NULL DEFAULT '',
  initial INTEGER NOT NULL DEFAULT 0,
  final INTEGER NOT NULL DEFAULT 0,
  discount INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (appid, day)
);
CREATE TABLE IF NOT EXISTS reviews (
  appid INTEGER PRIMARY KEY,
  t INTEGER NOT NULL,
  total INTEGER NOT NULL DEFAULT 0,
  popularity INTEGER NOT NULL DEFAULT 50,
  positive INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS users (
  steamid TEXT PRIMARY KEY,
  name TEXT NOT NULL DEFAULT '',
  avatar TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  last_login_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
  token_hash TEXT PRIMARY KEY,
  steamid TEXT NOT NULL,
  expires_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_steamid ON sessions(steamid);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
CREATE TABLE IF NOT EXISTS library_focus (
  token_hash TEXT PRIMARY KEY,
  steamid TEXT NOT NULL,
  expires_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_library_focus_expires ON library_focus(expires_at);
CREATE TABLE IF NOT EXISTS player_stats (
  appid INTEGER PRIMARY KEY,
  current INTEGER NOT NULL DEFAULT 0,
  peak_day INTEGER NOT NULL DEFAULT 0,
  peak_all INTEGER NOT NULL DEFAULT 0,
  sampled_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS player_history (
  appid INTEGER NOT NULL,
  hour INTEGER NOT NULL,
  current INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (appid, hour)
);
CREATE INDEX IF NOT EXISTS idx_player_history_hour ON player_history(hour);
CREATE TABLE IF NOT EXISTS app_details (
  appid INTEGER PRIMARY KEY,
  type TEXT NOT NULL DEFAULT '',
  developers TEXT NOT NULL DEFAULT '[]',
  publishers TEXT NOT NULL DEFAULT '[]',
  genres TEXT NOT NULL DEFAULT '[]',
  categories TEXT NOT NULL DEFAULT '[]',
  release_date TEXT NOT NULL DEFAULT '',
  metacritic INTEGER,
  short_description TEXT NOT NULL DEFAULT '',
  missing INTEGER NOT NULL DEFAULT 0,
  fetched_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS app_news (
  appid INTEGER NOT NULL,
  gid TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  url TEXT NOT NULL DEFAULT '',
  author TEXT NOT NULL DEFAULT '',
  feedlabel TEXT NOT NULL DEFAULT '',
  date INTEGER NOT NULL DEFAULT 0,
  contents TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (appid, gid)
);
CREATE INDEX IF NOT EXISTS idx_app_news_date ON app_news(appid, date DESC);
CREATE TABLE IF NOT EXISTS app_news_meta (
  appid INTEGER PRIMARY KEY,
  fetched_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS app_review_snippets (
  appid INTEGER NOT NULL,
  review_id TEXT NOT NULL,
  steamid TEXT NOT NULL DEFAULT '',
  voted_up INTEGER NOT NULL DEFAULT 1,
  hours INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL DEFAULT 0,
  body TEXT NOT NULL DEFAULT '',
  url TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (appid, review_id)
);
CREATE INDEX IF NOT EXISTS idx_app_review_snippets_created ON app_review_snippets(appid, created_at DESC);
CREATE TABLE IF NOT EXISTS app_review_snippets_meta (
  appid INTEGER PRIMARY KEY,
  fetched_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS app_achievements (
  appid INTEGER PRIMARY KEY,
  payload TEXT NOT NULL DEFAULT '[]',
  fetched_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS itad_map (
  appid INTEGER PRIMARY KEY,
  gid TEXT,
  looked_up_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS price_low (
  appid INTEGER PRIMARY KEY,
  cents INTEGER NOT NULL DEFAULT 0,
  currency TEXT NOT NULL DEFAULT '',
  shop TEXT NOT NULL DEFAULT 'Steam',
  shop_id INTEGER NOT NULL DEFAULT 61,
  low_at INTEGER,
  source TEXT NOT NULL DEFAULT 'itad',
  fetched_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS saved_searches (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  steamid TEXT NOT NULL,
  name TEXT NOT NULL,
  terms TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL DEFAULT 'recs',
  filters TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_saved_searches_steamid ON saved_searches(steamid);
`

type DB struct {
	sql *sql.DB
}

type Stats struct {
	Games    int
	Tagged   int
	Priced   int
	TagNames int
}

type GameTag struct {
	TagID  int
	Weight int
}

type DictTag struct {
	TagID int    `json:"tagid"`
	Name  string `json:"name"`
}

type Price struct {
	Name      string
	T         int64
	Currency  string
	Initial   int
	Final     int
	Discount  int
	Formatted string
}

type App struct {
	AppID             int
	Name              string
	PriceChangeNumber *int
}

func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", path)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	if _, err := sqlDB.Exec(schema); err != nil {
		sqlDB.Close()
		return nil, err
	}
	_, _ = sqlDB.Exec("ALTER TABLE games ADD COLUMN price_fetched_at INTEGER")
	_, _ = sqlDB.Exec("ALTER TABLE games ADD COLUMN price_change_number INTEGER")
	_, _ = sqlDB.Exec("ALTER TABLE reviews ADD COLUMN positive INTEGER NOT NULL DEFAULT 0")
	_, _ = sqlDB.Exec("ALTER TABLE saved_searches ADD COLUMN terms TEXT NOT NULL DEFAULT ''")
	_, _ = sqlDB.Exec("ALTER TABLE saved_searches ADD COLUMN kind TEXT NOT NULL DEFAULT 'recs'")
	return &DB{sql: sqlDB}, nil
}

func (db *DB) Close() error {
	return db.sql.Close()
}

func (db *DB) Stats() (Stats, error) {
	var s Stats
	row := db.sql.QueryRow(`
		SELECT
		  (SELECT COUNT(*) FROM games),
		  (SELECT COUNT(*) FROM games WHERE tags_ok = 1),
		  (SELECT COUNT(*) FROM games WHERE price_fetched_at IS NOT NULL),
		  (SELECT COUNT(*) FROM tag_dict)
	`)
	err := row.Scan(&s.Games, &s.Tagged, &s.Priced, &s.TagNames)
	return s, err
}

func (db *DB) Meta(key string) string {
	var value string
	err := db.sql.QueryRow("SELECT value FROM meta WHERE key = ?", key).Scan(&value)
	if err != nil {
		return ""
	}
	return value
}

func (db *DB) SetMeta(key, value string) error {
	_, err := db.sql.Exec(
		"INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	return err
}

func (db *DB) ReplaceTagDict(tags []DictTag, versionHash string) error {
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM tag_dict"); err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO tag_dict (tagid, name) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, tag := range tags {
		if _, err := stmt.Exec(tag.TagID, tag.Name); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if versionHash != "" {
		_ = db.SetMeta("tag_list_hash", versionHash)
	}
	return db.SetMeta("tag_list_at", fmt.Sprintf("%d", time.Now().UnixMilli()))
}

func (db *DB) UpsertGames(apps []App) error {
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	plain, err := tx.Prepare(`
		INSERT INTO games (appid, name) VALUES (?, ?)
		ON CONFLICT(appid) DO UPDATE SET name = CASE
		  WHEN excluded.name != '' THEN excluded.name ELSE games.name END
	`)
	if err != nil {
		return err
	}
	defer plain.Close()
	withChange, err := tx.Prepare(`
		INSERT INTO games (appid, name, price_change_number) VALUES (?, ?, ?)
		ON CONFLICT(appid) DO UPDATE SET
		  name = CASE WHEN excluded.name != '' THEN excluded.name ELSE games.name END,
		  price_fetched_at = CASE
		    WHEN games.price_change_number IS NOT NULL
		      AND excluded.price_change_number IS NOT NULL
		      AND games.price_change_number != excluded.price_change_number
		    THEN NULL
		    ELSE games.price_fetched_at
		  END,
		  price_change_number = COALESCE(excluded.price_change_number, games.price_change_number)
	`)
	if err != nil {
		return err
	}
	defer withChange.Close()
	for _, app := range apps {
		if app.PriceChangeNumber == nil {
			if _, err := plain.Exec(app.AppID, app.Name); err != nil {
				return err
			}
			continue
		}
		if _, err := withChange.Exec(app.AppID, app.Name, *app.PriceChangeNumber); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (db *DB) SetGameTags(appid int, name string, tags []GameTag, ok bool) error {
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
		INSERT INTO games (appid, name) VALUES (?, ?)
		ON CONFLICT(appid) DO UPDATE SET name = CASE
		  WHEN excluded.name != '' THEN excluded.name ELSE games.name END
	`, appid, name); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM game_tags WHERE appid = ?", appid); err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO game_tags (appid, tagid, weight) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, tag := range tags {
		if _, err := stmt.Exec(appid, tag.TagID, tag.Weight); err != nil {
			return err
		}
	}
	okInt := 0
	if ok {
		okInt = 1
	}
	if _, err := tx.Exec("UPDATE games SET tags_fetched_at = ?, tags_ok = ? WHERE appid = ?", time.Now().UnixMilli(), okInt, appid); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) SetPrice(appid int, price Price) error {
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
		INSERT INTO games (appid, name) VALUES (?, ?)
		ON CONFLICT(appid) DO UPDATE SET name = CASE
		  WHEN excluded.name != '' THEN excluded.name ELSE games.name END
	`, appid, price.Name); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO price_latest (appid, t, currency, initial, final, discount, formatted)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(appid) DO UPDATE SET
		  t = excluded.t, currency = excluded.currency, initial = excluded.initial,
		  final = excluded.final, discount = excluded.discount, formatted = excluded.formatted
	`, appid, price.T, price.Currency, price.Initial, price.Final, price.Discount, price.Formatted); err != nil {
		return err
	}
	day := price.T / 86400000
	if _, err := tx.Exec(`
		INSERT INTO price_history (appid, day, t, currency, initial, final, discount)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(appid, day) DO UPDATE SET
		  t = excluded.t, currency = excluded.currency, initial = excluded.initial,
		  final = excluded.final, discount = excluded.discount
	`, appid, day, price.T, price.Currency, price.Initial, price.Final, price.Discount); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE games SET price_fetched_at = ? WHERE appid = ?", price.T, appid); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) NeedWork(wantTags, wantPrices, wantReviews bool, limit int) ([]int, error) {
	if limit <= 0 {
		limit = 40
	}
	now := time.Now().UnixMilli()
	tagStale := now - 14*24*60*60*1000
	priceStale := now - 24*60*60*1000
	reviewStale := now - 7*24*60*60*1000
	rows, err := db.sql.Query(`
		SELECT g.appid FROM games g
		LEFT JOIN reviews r ON r.appid = g.appid
		WHERE (? = 1 AND (g.tags_fetched_at IS NULL OR g.tags_fetched_at < ?))
		   OR (? = 1 AND (g.price_fetched_at IS NULL OR g.price_fetched_at < ?))
		   OR (? = 1 AND g.tags_ok = 1 AND (r.appid IS NULL OR r.t < ?))
		ORDER BY COALESCE(r.total, 0) DESC, g.appid
		LIMIT ?
	`, boolInt(wantTags), tagStale, boolInt(wantPrices), priceStale, boolInt(wantReviews), reviewStale, limit)
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

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
