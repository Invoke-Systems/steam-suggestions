package jobs

import (
	"context"
	"errors"
	"log"
	"time"

	"steam-suggestions/internal/steam"
	"steam-suggestions/internal/store"
)

func scrapeDetails(ctx context.Context, db *store.DB, client *steam.Client, opt Config) error {
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
		ids, err := db.NeedDetailsWork(want, 14*24*3600)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		for _, appid := range ids {
			if err := ctx.Err(); err != nil {
				return err
			}
			if opt.Limit > 0 && done >= opt.Limit {
				break
			}
			d, err := client.AppDetails(appid)
			if err != nil {
				var rate *steam.RateError
				if errors.As(err, &rate) {
					consecutive429++
					if consecutive429 >= opt.Max429 {
						return err
					}
					if err := sleepCtx(ctx, opt.Delay*time.Duration(consecutive429+1)); err != nil {
						return err
					}
					continue
				}
				log.Printf("details %d: %v", appid, err)
				continue
			}
			consecutive429 = 0
			if err := db.SetAppDetails(d); err != nil {
				return err
			}
			done++
			if err := sleepCtx(ctx, opt.Delay); err != nil {
				return err
			}
		}
		log.Printf("details: fetched %d this pass", done)
	}
	return nil
}

func scrapeNews(ctx context.Context, db *store.DB, client *steam.Client, opt Config) error {
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
		ids, err := db.NeedNewsWork(want, 24*3600)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		for _, appid := range ids {
			if err := ctx.Err(); err != nil {
				return err
			}
			if opt.Limit > 0 && done >= opt.Limit {
				break
			}
			items, err := client.NewsForApp(appid, 5)
			if err != nil {
				var rate *steam.RateError
				if errors.As(err, &rate) {
					consecutive429++
					if consecutive429 >= opt.Max429 {
						return err
					}
					if err := sleepCtx(ctx, opt.Delay*time.Duration(consecutive429+1)); err != nil {
						return err
					}
					continue
				}
				log.Printf("news %d: %v", appid, err)
				_ = db.SetAppNews(appid, nil)
				done++
				continue
			}
			consecutive429 = 0
			if err := db.SetAppNews(appid, items); err != nil {
				return err
			}
			done++
			if err := sleepCtx(ctx, opt.Delay); err != nil {
				return err
			}
		}
		log.Printf("news: fetched %d this pass", done)
	}
	return nil
}

func scrapeAchievements(ctx context.Context, db *store.DB, client *steam.Client, opt Config) error {
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
		ids, err := db.NeedAchievementsWork(want, 7*24*3600)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		for _, appid := range ids {
			if err := ctx.Err(); err != nil {
				return err
			}
			if opt.Limit > 0 && done >= opt.Limit {
				break
			}
			items, err := client.AchievementPercentages(appid)
			if err != nil {
				var rate *steam.RateError
				if errors.As(err, &rate) {
					consecutive429++
					if consecutive429 >= opt.Max429 {
						return err
					}
					if err := sleepCtx(ctx, opt.Delay*time.Duration(consecutive429+1)); err != nil {
						return err
					}
					continue
				}
				// Many apps have no stats schema; store empty so we don't thrash.
				log.Printf("achievements %d: %v", appid, err)
				_ = db.SetAchievements(appid, nil)
				done++
				if err := sleepCtx(ctx, opt.Delay); err != nil {
					return err
				}
				continue
			}
			consecutive429 = 0
			if err := db.SetAchievements(appid, items); err != nil {
				return err
			}
			done++
			if err := sleepCtx(ctx, opt.Delay); err != nil {
				return err
			}
		}
		log.Printf("achievements: fetched %d this pass", done)
	}
	return nil
}
