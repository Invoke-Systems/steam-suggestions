package jobs

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"steam-suggestions/internal/steam"
	"steam-suggestions/internal/store"
)

func scrapeTagList(ctx context.Context, db *store.DB, client *steam.Client) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return RefreshTagDict(db, client)
}

func scrapeAppList(ctx context.Context, db *store.DB, client *steam.Client) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := IngestAppList(db, client); err != nil {
		log.Printf("applist: %v", err)
	}
	return nil
}

func scrapeItems(ctx context.Context, db *store.DB, client *steam.Client, opt Config) error {
	consecutive429 := 0
	done := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if opt.Limit > 0 && done >= opt.Limit {
			break
		}
		want := opt.Batch
		if opt.Limit > 0 && opt.Limit-done < want {
			want = opt.Limit - done
		}
		ids, err := db.NeedWork(opt.Tags, opt.Prices, opt.Reviews, want)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		items, err := client.GetItems(ids)
		if err != nil {
			var rate *steam.RateError
			if errors.As(err, &rate) {
				consecutive429++
				if consecutive429 >= opt.Max429 {
					return fmt.Errorf("items: stopped after %d consecutive 429/5xx", consecutive429)
				}
				backoff := opt.Delay * time.Duration(1<<uint(consecutive429))
				if backoff > 60*time.Second {
					backoff = 60 * time.Second
				}
				log.Printf("items: steam %d; backing off %s (%d/%d)", rate.Status, backoff, consecutive429, opt.Max429)
				if err := sleepCtx(ctx, backoff); err != nil {
					return err
				}
				continue
			}
			return err
		}
		consecutive429 = 0
		now := time.Now().UnixMilli()
		seen := map[int]steam.Item{}
		for _, item := range items {
			seen[item.AppID] = item
		}
		for _, appid := range ids {
			item, ok := seen[appid]
			if opt.Tags {
				if err := db.SetGameTags(appid, item.Name, item.Tags, ok && len(item.Tags) > 0); err != nil {
					return err
				}
			}
			if opt.Prices {
				price := client.PriceFromItem(item, now)
				if err := db.SetPrice(appid, price); err != nil {
					return err
				}
			}
			if item.HasReviews {
				if err := db.SetReviews(appid, item.ReviewCount, item.PercentPositive, steam.ReviewsToPopularity(item.ReviewCount)); err != nil {
					return err
				}
			} else if opt.Reviews && ok {
				if err := db.SetReviews(appid, 0, 0, steam.ReviewsToPopularity(0)); err != nil {
					return err
				}
			}
		}
		done += len(ids)
		stats, _ := db.Stats()
		log.Printf("items: batch %d (run %d) — %d tagged, %d priced / %d games", len(ids), done, stats.Tagged, stats.Priced, stats.Games)
		if err := sleepCtx(ctx, opt.Delay); err != nil {
			return err
		}
	}
	if opt.Prices {
		if err := db.PrunePriceHistory(730); err != nil {
			log.Printf("items: price history prune: %v", err)
		}
	}
	stats, _ := db.Stats()
	log.Printf("items: done — %d tagged, %d priced / %d games", stats.Tagged, stats.Priced, stats.Games)
	return nil
}
