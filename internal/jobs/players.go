package jobs

import (
	"context"
	"errors"
	"log"
	"time"

	"steam-suggestions/internal/steam"
	"steam-suggestions/internal/store"
)

func scrapePlayers(ctx context.Context, db *store.DB, client *steam.Client, opt Config) error {
	if opt.Batch <= 0 {
		opt.Batch = 40
	}
	if opt.Delay <= 0 {
		opt.Delay = 400 * time.Millisecond
	}
	if opt.Max429 <= 0 {
		opt.Max429 = 5
	}
	if opt.Limit <= 0 {
		opt.Limit = 200
	}
	consecutive429 := 0
	done := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if done >= opt.Limit {
			break
		}
		want := opt.Batch
		if opt.Limit-done < want {
			want = opt.Limit - done
		}
		ids, err := db.NeedPlayerWork(want)
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
			if done >= opt.Limit {
				break
			}
			count, err := client.CurrentPlayers(appid)
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
				log.Printf("players %d: %v", appid, err)
				continue
			}
			consecutive429 = 0
			if err := db.SetPlayerSample(appid, count); err != nil {
				return err
			}
			done++
			if err := sleepCtx(ctx, opt.Delay); err != nil {
				return err
			}
		}
		log.Printf("players: sampled %d this pass", done)
	}
	_ = db.PrunePlayerHistory(14)
	return nil
}
