package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"steam-suggestions/internal/env"
	"steam-suggestions/internal/itad"
	"steam-suggestions/internal/jobs"
	"steam-suggestions/internal/steam"
	"steam-suggestions/internal/store"
)

func main() {
	env.Load(".env")

	tagList := flag.Bool("taglist", false, "refresh IStoreService/GetTagList")
	appList := flag.Bool("applist", false, "ingest IStoreService/GetAppList")
	items := flag.Bool("items", false, "crawl IStoreBrowseService/GetItems (tags/prices/reviews)")
	tags := flag.Bool("tags", false, "write tags from GetItems (implies -items)")
	prices := flag.Bool("prices", false, "write prices from GetItems (implies -items)")
	reviews := flag.Bool("reviews", false, "write reviews from GetItems (implies -items)")
	players := flag.Bool("players", false, "sample GetNumberOfCurrentPlayers")
	details := flag.Bool("details", false, "crawl store/api/appdetails")
	news := flag.Bool("news", false, "crawl ISteamNews/GetNewsForApp")
	achievements := flag.Bool("achievements", false, "crawl GetGlobalAchievementPercentagesForApp")
	itadLows := flag.Bool("itad-lows", false, "one-time/resumable ITAD Steam historical-low backfill")
	itadOrder := flag.String("itad-order", "newest", "ITAD queue order: reviews (known games first) or newest (catch-up)")
	itadMinReviews := flag.Int("itad-min-reviews", 0, "ITAD: only queue apps with at least this many reviews (leave sparse apps for later)")
	delay := flag.Duration("delay", 2*time.Second, "base delay between requests/batches")
	batch := flag.Int("batch", 40, "appids per batch where applicable")
	workers := flag.Int("workers", 8, "concurrent HTTP workers for -players")
	dbPath := flag.String("db", "data/steam.sqlite", "sqlite path")
	max429 := flag.Int("max-429", 5, "exit a scraper after this many consecutive 429/5xx")
	limit := flag.Int("limit", 0, "stop each scraper after this many apps (0 = drain queue)")
	every := flag.Duration("every", 0, "repeat each scraper on this interval (0 = once)")
	flag.Parse()

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	steamOn := *tagList || *appList || *items || *tags || *prices || *reviews ||
		*players || *details || *news || *achievements
	if !steamOn && !*itadLows {
		steamOn = true
	}

	if *itadLows {
		itadDelay := *delay
		if *delay == 2*time.Second {
			itadDelay = 400 * time.Millisecond
		}
		itadBatch := *batch
		if *batch == 40 {
			itadBatch = 100
		}
		client := itad.New(os.Getenv("ITAD_API_KEY"), env.Get("STEAM_CC", "US"))
		log.Printf("worker: ITAD Steam historical-low backfill (country=%s order=%s min-reviews=%d)",
			client.Country, *itadOrder, *itadMinReviews)
		if err := jobs.FillITADLows(ctx, db, client, jobs.Config{
			Batch:          itadBatch,
			Delay:          itadDelay,
			Max429:         *max429,
			Limit:          *limit,
			ITADOrder:      *itadOrder,
			ITADMinReviews: *itadMinReviews,
		}); err != nil {
			log.Fatal(err)
		}
	}

	if !steamOn {
		return
	}

	client := steam.New(
		os.Getenv("STEAM_API_KEY"),
		os.Getenv("STEAM_CC"),
		os.Getenv("STEAM_CURRENCY"),
	)
	cfg := jobs.Config{
		TagList:      *tagList,
		AppList:      *appList,
		Items:        *items,
		Tags:         *tags,
		Prices:       *prices,
		Reviews:      *reviews,
		Players:      *players,
		Details:      *details,
		News:         *news,
		Achievements: *achievements,
		Batch:        *batch,
		Delay:        *delay,
		Max429:       *max429,
		Limit:        *limit,
		Every:        *every,
		Workers:      *workers,
	}
	log.Printf("worker: launching independent Steam scrapers")
	if err := jobs.Run(ctx, db, client, cfg); err != nil {
		log.Fatal(err)
	}
}
