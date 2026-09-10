package jobs

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"steam-suggestions/internal/itad"
	"steam-suggestions/internal/store"
)

// FillITADLows is a one-time (or resumable) backfill of Steam historical lows via IsThereAnyDeal.
// It does not scrape HTML; it uses the official ITAD API with ITAD_API_KEY.
func FillITADLows(ctx context.Context, db *store.DB, client *itad.Client, opt Config) error {
	if client == nil || client.APIKey == "" {
		return errors.New("ITAD_API_KEY is required for -itad-lows")
	}
	if opt.Batch <= 0 {
		opt.Batch = 100
	}
	if opt.Batch > 200 {
		opt.Batch = 200
	}
	if opt.Delay <= 0 {
		opt.Delay = 400 * time.Millisecond
	}
	if opt.Max429 <= 0 {
		opt.Max429 = 8
	}
	if opt.ITADOrder == "" {
		opt.ITADOrder = store.ITADOrderNewest
	}
	if opt.ITADMinReviews < 0 {
		opt.ITADMinReviews = 0
	}
	log.Printf("itad: order=%s min-reviews=%d", opt.ITADOrder, opt.ITADMinReviews)

	looked := 0
	filled := 0
	missed := 0
	consecutive429 := 0

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if opt.Limit > 0 && looked >= opt.Limit {
			break
		}
		want := opt.Batch
		if opt.Limit > 0 {
			left := opt.Limit - looked
			if left < want {
				want = left
			}
		}
		ids, err := db.NeedITADLookup(want, opt.ITADOrder, opt.ITADMinReviews)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		// Drop invalid ids (SetITADMap ignores appid<=0, which would loop forever).
		valid := ids[:0]
		for _, id := range ids {
			if id > 0 {
				valid = append(valid, id)
			}
		}
		ids = valid
		if len(ids) == 0 {
			log.Printf("itad lookup: only invalid appids left; moving on to store lows")
			break
		}
		names := db.GetGameNames(ids)
		mapped, err := client.LookupSteam(ids)
		if err != nil {
			var rate *itad.RateError
			if errors.As(err, &rate) {
				consecutive429++
				if consecutive429 >= opt.Max429 {
					return err
				}
				wait := rate.RetryAfter
				if wait <= 0 {
					wait = opt.Delay * time.Duration(consecutive429+1)
				}
				log.Printf("itad lookup: rate limited, sleeping %s", wait)
				if err := sleepCtx(ctx, wait); err != nil {
					return err
				}
				continue
			}
			return err
		}
		consecutive429 = 0
		for _, appid := range ids {
			gid := mapped[appid]
			if err := db.SetITADMap(appid, gid); err != nil {
				return err
			}
			name := names[appid]
			if name == "" {
				name = fmt.Sprintf("app %d", appid)
			}
			if gid != "" {
				log.Printf("itad lookup: %s (%d) → %s", name, appid, shortGID(gid))
			} else {
				log.Printf("itad lookup: %s (%d) → not on ITAD", name, appid)
			}
		}
		looked += len(ids)
		log.Printf("itad lookup: batch %d (found %d) — total looked up %d", len(ids), len(mapped), looked)
		if err := sleepCtx(ctx, opt.Delay); err != nil {
			return err
		}
	}

	// -limit applies per phase: lookups first, then storelow attempts (hits + misses).
	storeDone := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if opt.Limit > 0 && storeDone >= opt.Limit {
			break
		}
		want := opt.Batch
		if opt.Limit > 0 && opt.Limit-storeDone < want {
			want = opt.Limit - storeDone
		}
		rows, err := db.NeedITADLowFill(want, opt.ITADOrder, opt.ITADMinReviews)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		ids := make([]int, 0, len(rows))
		gids := make([]string, 0, len(rows))
		byGID := map[string]int{}
		for _, row := range rows {
			ids = append(ids, row.AppID)
			gids = append(gids, row.GID)
			byGID[row.GID] = row.AppID
		}
		names := db.GetGameNames(ids)
		lows, err := client.SteamStoreLows(gids)
		if err != nil {
			var rate *itad.RateError
			if errors.As(err, &rate) {
				consecutive429++
				if consecutive429 >= opt.Max429 {
					return err
				}
				wait := rate.RetryAfter
				if wait <= 0 {
					wait = opt.Delay * time.Duration(consecutive429+1)
				}
				log.Printf("itad storelow: rate limited, sleeping %s", wait)
				if err := sleepCtx(ctx, wait); err != nil {
					return err
				}
				continue
			}
			return err
		}
		consecutive429 = 0
		got := map[string]bool{}
		for _, low := range lows {
			appid := byGID[low.GID]
			if appid <= 0 {
				continue
			}
			got[low.GID] = true
			var lowAt int64
			if !low.Timestamp.IsZero() {
				lowAt = low.Timestamp.Unix()
			}
			if err := db.SetPriceLow(store.PriceLow{
				AppID:    appid,
				Cents:    low.Cents,
				Currency: low.Currency,
				Shop:     low.Shop,
				ShopID:   low.ShopID,
				LowAt:    lowAt,
				Source:   "itad",
			}); err != nil {
				return err
			}
			filled++
			name := names[appid]
			if name == "" {
				name = fmt.Sprintf("app %d", appid)
			}
			when := ""
			if !low.Timestamp.IsZero() {
				when = " · " + low.Timestamp.UTC().Format("2006-01-02")
			}
			log.Printf("itad low: %s (%d) → %s%s", name, appid, formatITADCents(low.Cents, low.Currency), when)
		}
		for _, row := range rows {
			if got[row.GID] {
				continue
			}
			if err := db.MarkPriceLowMissing(row.AppID); err != nil {
				return err
			}
			missed++
			name := names[row.AppID]
			if name == "" {
				name = fmt.Sprintf("app %d", row.AppID)
			}
			log.Printf("itad low: %s (%d) → no Steam low on ITAD", name, row.AppID)
		}
		storeDone += len(rows)
		log.Printf("itad storelow: batch done — %d lows, %d misses (filled %d total, processed %d)", len(got), len(rows)-len(got), filled, storeDone)
		if err := sleepCtx(ctx, opt.Delay); err != nil {
			return err
		}
	}

	log.Printf("itad: done — %d lookups, %d Steam lows, %d misses", looked, filled, missed)
	return nil
}

func shortGID(gid string) string {
	if len(gid) <= 12 {
		return gid
	}
	return gid[:8] + "…"
}

func formatITADCents(cents int, currency string) string {
	if p := store.FormatCents(cents, currency); p != nil {
		return *p
	}
	return fmt.Sprintf("%d %s", cents, currency)
}
