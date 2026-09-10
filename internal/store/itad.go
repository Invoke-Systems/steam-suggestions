package store

import (
	"database/sql"
	"time"
)

type PriceLow struct {
	AppID     int
	Cents     int
	Currency  string
	Shop      string
	ShopID    int
	LowAt     int64
	Source    string
	FetchedAt int64
}

func (db *DB) SetITADMap(appid int, gid string) error {
	if appid <= 0 {
		return nil
	}
	var gidArg any
	if gid != "" {
		gidArg = gid
	}
	_, err := db.sql.Exec(`
		INSERT INTO itad_map (appid, gid, looked_up_at) VALUES (?, ?, ?)
		ON CONFLICT(appid) DO UPDATE SET
		  gid = excluded.gid,
		  looked_up_at = excluded.looked_up_at
	`, appid, gidArg, time.Now().Unix())
	return err
}

func (db *DB) SetPriceLow(low PriceLow) error {
	if low.AppID <= 0 || low.Cents <= 0 {
		return nil
	}
	if low.FetchedAt <= 0 {
		low.FetchedAt = time.Now().Unix()
	}
	if low.Source == "" {
		low.Source = "itad"
	}
	if low.Shop == "" {
		low.Shop = "Steam"
	}
	if low.ShopID == 0 {
		low.ShopID = 61
	}
	var lowAt any
	if low.LowAt > 0 {
		lowAt = low.LowAt
	}
	_, err := db.sql.Exec(`
		INSERT INTO price_low (appid, cents, currency, shop, shop_id, low_at, source, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(appid) DO UPDATE SET
		  cents = CASE
		    WHEN price_low.cents <= 0 OR excluded.cents < price_low.cents THEN excluded.cents
		    ELSE price_low.cents
		  END,
		  currency = excluded.currency,
		  shop = excluded.shop,
		  shop_id = excluded.shop_id,
		  low_at = CASE
		    WHEN price_low.cents <= 0 OR excluded.cents < price_low.cents THEN excluded.low_at
		    ELSE price_low.low_at
		  END,
		  source = excluded.source,
		  fetched_at = excluded.fetched_at
	`, low.AppID, low.Cents, low.Currency, low.Shop, low.ShopID, lowAt, low.Source, low.FetchedAt)
	return err
}

func (db *DB) MarkPriceLowMissing(appid int) error {
	if appid <= 0 {
		return nil
	}
	now := time.Now().Unix()
	_, err := db.sql.Exec(`
		INSERT INTO price_low (appid, cents, currency, shop, shop_id, low_at, source, fetched_at)
		VALUES (?, 0, '', 'Steam', 61, NULL, 'itad-miss', ?)
		ON CONFLICT(appid) DO UPDATE SET
		  fetched_at = excluded.fetched_at,
		  source = CASE WHEN price_low.cents > 0 THEN price_low.source ELSE excluded.source END
	`, appid, now)
	return err
}

func (db *DB) GetPriceLows(ids []int) map[int]PriceLow {
	out := map[int]PriceLow{}
	if len(ids) == 0 {
		return out
	}
	for _, chunk := range chunks(ids, 200) {
		q := `SELECT appid, cents, currency, shop, shop_id, low_at, source, fetched_at
		      FROM price_low WHERE appid IN (` + placeholders(len(chunk)) + `) AND cents > 0`
		rows, err := db.sql.Query(q, asArgs(chunk)...)
		if err != nil {
			continue
		}
		for rows.Next() {
			var row PriceLow
			var lowAt sql.NullInt64
			if err := rows.Scan(&row.AppID, &row.Cents, &row.Currency, &row.Shop, &row.ShopID, &lowAt, &row.Source, &row.FetchedAt); err != nil {
				continue
			}
			if lowAt.Valid {
				row.LowAt = lowAt.Int64
			}
			out[row.AppID] = row
		}
		_ = rows.Err()
		rows.Close()
	}
	return out
}

// ITAD queue order: newest = catch up new appids; reviews = backfill known games first.
const (
	ITADOrderNewest  = "newest"
	ITADOrderReviews = "reviews"
)

func normalizeITADOrder(order string) string {
	if order == ITADOrderReviews {
		return ITADOrderReviews
	}
	return ITADOrderNewest
}

func itadOrderSQL(alias string, order string) string {
	if normalizeITADOrder(order) == ITADOrderReviews {
		return "COALESCE(r.total, 0) DESC, " + alias + ".appid DESC"
	}
	return alias + ".appid DESC, COALESCE(r.total, 0) DESC"
}

// NeedITADLookup returns appids that still need an ITAD id map row.
func (db *DB) NeedITADLookup(limit int, order string, minReviews int) ([]int, error) {
	if limit <= 0 {
		limit = 100
	}
	if minReviews < 0 {
		minReviews = 0
	}
	rows, err := db.sql.Query(`
		SELECT g.appid FROM games g
		LEFT JOIN itad_map m ON m.appid = g.appid
		LEFT JOIN reviews r ON r.appid = g.appid
		WHERE g.appid > 0 AND m.appid IS NULL
		  AND COALESCE(r.total, 0) >= ?
		ORDER BY `+itadOrderSQL("g", order)+`
		LIMIT ?
	`, minReviews, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIDs(rows)
}

type ITADMapped struct {
	AppID int
	GID   string
}

// NeedITADLowFill returns mapped apps that still need a Steam historical low.
// Apps already in price_low (hit or miss marker) are skipped.
// With minReviews > 0, sparse/new apps stay unqueued for a later catch-up pass.
func (db *DB) NeedITADLowFill(limit int, order string, minReviews int) ([]ITADMapped, error) {
	if limit <= 0 {
		limit = 100
	}
	if minReviews < 0 {
		minReviews = 0
	}
	rows, err := db.sql.Query(`
		SELECT m.appid, m.gid FROM itad_map m
		LEFT JOIN price_low p ON p.appid = m.appid
		LEFT JOIN reviews r ON r.appid = m.appid
		WHERE m.appid > 0 AND m.gid IS NOT NULL AND m.gid != ''
		  AND p.appid IS NULL
		  AND COALESCE(r.total, 0) >= ?
		ORDER BY `+itadOrderSQL("m", order)+`
		LIMIT ?
	`, minReviews, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ITADMapped
	for rows.Next() {
		var row ITADMapped
		if err := rows.Scan(&row.AppID, &row.GID); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
