package jobs

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"steam-suggestions/internal/steam"
	"steam-suggestions/internal/store"
)

// Config selects independent Steam endpoint scrapers.
// Each enabled scraper runs in its own goroutine.
type Config struct {
	TagList      bool
	AppList      bool
	Items        bool // IStoreBrowseService/GetItems → tags, prices, reviews
	Players      bool // GetNumberOfCurrentPlayers
	Details      bool // store/api/appdetails
	News         bool // ISteamNews/GetNewsForApp
	Achievements bool // GetGlobalAchievementPercentagesForApp

	// Legacy aliases mapped onto Items writes.
	Tags    bool
	Prices  bool
	Reviews bool

	Batch   int
	Delay   time.Duration
	Max429  int
	Limit   int
	Every   time.Duration
	Workers int // concurrent HTTP workers (players scraper)

	// ITAD historical-low queue controls (used by FillITADLows).
	// Order: "newest" (appid DESC, catch-up) or "reviews" (known games first).
	ITADOrder      string
	ITADMinReviews int // skip apps with fewer reviews (0 = no filter)
}

func (c *Config) normalize() {
	if c.Tags || c.Prices || c.Reviews {
		c.Items = true
	}
	if !c.TagList && !c.AppList && !c.Items && !c.Players && !c.Details && !c.News && !c.Achievements {
		c.TagList = true
		c.AppList = true
		c.Items = true
		c.Players = true
		c.Details = true
		c.News = true
		c.Achievements = true
		c.Tags = true
		c.Prices = true
		c.Reviews = true
	}
	if c.Items && !c.Tags && !c.Prices && !c.Reviews {
		c.Tags = true
		c.Prices = true
		c.Reviews = true
	}
	if c.Batch <= 0 {
		c.Batch = 40
	}
	if c.Delay <= 0 {
		c.Delay = 2 * time.Second
	}
	if c.Max429 <= 0 {
		c.Max429 = 5
	}
}

// Run starts one goroutine per enabled Steam endpoint and waits for all to finish
// (or for ctx cancellation). When Every > 0, each scraper loops until cancelled.
func Run(ctx context.Context, db *store.DB, client *steam.Client, cfg Config) error {
	cfg.normalize()

	type job struct {
		name string
		fn   func(context.Context) error
	}
	var jobs []job
	if cfg.TagList {
		jobs = append(jobs, job{"taglist", func(ctx context.Context) error {
			return scrapeLoop(ctx, cfg.Every, func(ctx context.Context) error {
				return scrapeTagList(ctx, db, client)
			})
		}})
	}
	if cfg.AppList {
		jobs = append(jobs, job{"applist", func(ctx context.Context) error {
			return scrapeLoop(ctx, cfg.Every, func(ctx context.Context) error {
				return scrapeAppList(ctx, db, client)
			})
		}})
	}
	if cfg.Items {
		jobs = append(jobs, job{"items", func(ctx context.Context) error {
			return scrapeLoop(ctx, cfg.Every, func(ctx context.Context) error {
				return scrapeItems(ctx, db, client, cfg)
			})
		}})
	}
	if cfg.Players {
		jobs = append(jobs, job{"players", func(ctx context.Context) error {
			return scrapeLoop(ctx, cfg.Every, func(ctx context.Context) error {
				p := cfg
				if p.Delay > 150*time.Millisecond {
					p.Delay = 150 * time.Millisecond
				}
				if p.Limit <= 0 {
					p.Limit = 200
				}
				if p.Workers <= 0 {
					p.Workers = 8
				}
				return scrapePlayers(ctx, db, client, p)
			})
		}})
	}
	if cfg.Details {
		jobs = append(jobs, job{"details", func(ctx context.Context) error {
			return scrapeLoop(ctx, cfg.Every, func(ctx context.Context) error {
				d := cfg
				if d.Delay > 250*time.Millisecond {
					d.Delay = 250 * time.Millisecond
				}
				return scrapeDetails(ctx, db, client, d)
			})
		}})
	}
	if cfg.News {
		jobs = append(jobs, job{"news", func(ctx context.Context) error {
			return scrapeLoop(ctx, cfg.Every, func(ctx context.Context) error {
				n := cfg
				if n.Delay > 300*time.Millisecond {
					n.Delay = 300 * time.Millisecond
				}
				return scrapeNews(ctx, db, client, n)
			})
		}})
	}
	if cfg.Achievements {
		jobs = append(jobs, job{"achievements", func(ctx context.Context) error {
			return scrapeLoop(ctx, cfg.Every, func(ctx context.Context) error {
				a := cfg
				if a.Delay > 300*time.Millisecond {
					a.Delay = 300 * time.Millisecond
				}
				return scrapeAchievements(ctx, db, client, a)
			})
		}})
	}

	names := make([]string, 0, len(jobs))
	for _, j := range jobs {
		names = append(names, j.name)
	}
	if cfg.Every > 0 {
		log.Printf("steam scrapers: %v (immediate pass, then every %s)", names, cfg.Every)
	} else {
		log.Printf("steam scrapers: %v (oneshot)", names)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(jobs))
	for _, j := range jobs {
		j := j
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Printf("%s: starting", j.name)
			if err := j.fn(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("%s: %v", j.name, err)
				errCh <- err
				return
			}
			log.Printf("%s: done", j.name)
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

func scrapeLoop(ctx context.Context, every time.Duration, once func(context.Context) error) error {
	// Always run once immediately (boot / oneshot), then optionally loop.
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := once(ctx); err != nil {
			return err
		}
		if every <= 0 {
			return nil
		}
		log.Printf("sleeping %s until next pass", every)
		timer := time.NewTimer(every)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
