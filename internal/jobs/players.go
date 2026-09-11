package jobs

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"steam-suggestions/internal/steam"
	"steam-suggestions/internal/store"
)

func scrapePlayers(ctx context.Context, db *store.DB, client *steam.Client, opt Config) error {
	if opt.Batch <= 0 {
		opt.Batch = 40
	}
	if opt.Delay <= 0 {
		opt.Delay = 150 * time.Millisecond
	}
	if opt.Max429 <= 0 {
		opt.Max429 = 8
	}
	if opt.Limit <= 0 {
		opt.Limit = 200
	}
	if opt.Workers <= 0 {
		opt.Workers = 8
	}

	if err := applyChartsSnapshot(ctx, db, client); err != nil {
		log.Printf("players: charts snapshot: %v (continuing with per-app samples)", err)
	}

	var done atomic.Int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		remaining := opt.Limit - int(done.Load())
		if remaining <= 0 {
			break
		}
		want := opt.Batch * opt.Workers
		if want < opt.Batch {
			want = opt.Batch
		}
		if remaining < want {
			want = remaining
		}
		ids, err := db.NeedPlayerWork(want)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		n, err := samplePlayersParallel(ctx, db, client, ids, opt)
		done.Add(int64(n))
		log.Printf("players: sampled %d this pass (%d total)", n, done.Load())
		if err != nil {
			return err
		}
		if n == 0 {
			break
		}
	}
	_ = db.PrunePlayerHistory(14)
	return nil
}

func applyChartsSnapshot(ctx context.Context, db *store.DB, client *steam.Client) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ranks, err := client.GamesByConcurrentPlayers()
	if err != nil {
		return err
	}
	wrote := 0
	for _, rank := range ranks {
		if err := ctx.Err(); err != nil {
			return err
		}
		current := rank.ConcurrentInGame
		if current < 0 {
			current = 0
		}
		if err := db.SetPlayerSampleEx(rank.AppID, current, rank.PeakInGame); err != nil {
			return err
		}
		wrote++
	}
	log.Printf("players: charts top %d applied (concurrent + peak today)", wrote)
	return nil
}

func samplePlayersParallel(ctx context.Context, db *store.DB, client *steam.Client, ids []int, opt Config) (int, error) {
	workers := opt.Workers
	if workers > len(ids) {
		workers = len(ids)
	}
	jobsCh := make(chan int)
	var wg sync.WaitGroup
	var dbMu sync.Mutex
	var sampled atomic.Int64
	var consecutive429 atomic.Int64
	errCh := make(chan error, 1)

	worker := func() {
		defer wg.Done()
		for appid := range jobsCh {
			if err := ctx.Err(); err != nil {
				return
			}
			if consecutive429.Load() >= int64(opt.Max429) {
				return
			}
			count, err := client.CurrentPlayers(appid)
			if err != nil {
				var rate *steam.RateError
				if errors.As(err, &rate) {
					n := consecutive429.Add(1)
					if n >= int64(opt.Max429) {
						select {
						case errCh <- err:
						default:
						}
						return
					}
					_ = sleepCtx(ctx, opt.Delay*time.Duration(n+1))
					continue
				}
				log.Printf("players %d: %v", appid, err)
				continue
			}
			consecutive429.Store(0)
			dbMu.Lock()
			writeErr := db.SetPlayerSample(appid, count)
			dbMu.Unlock()
			if writeErr != nil {
				select {
				case errCh <- writeErr:
				default:
				}
				return
			}
			sampled.Add(1)
			if err := sleepCtx(ctx, opt.Delay); err != nil {
				return
			}
		}
	}

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go worker()
	}
	send:
	for _, appid := range ids {
		select {
		case <-ctx.Done():
			break send
		case err := <-errCh:
			close(jobsCh)
			wg.Wait()
			return int(sampled.Load()), err
		case jobsCh <- appid:
		}
	}
	close(jobsCh)
	wg.Wait()
	select {
	case err := <-errCh:
		return int(sampled.Load()), err
	default:
		return int(sampled.Load()), ctx.Err()
	}
}
