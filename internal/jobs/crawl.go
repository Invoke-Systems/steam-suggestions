package jobs

import (
	"fmt"
	"log"
	"time"

	"steam-suggestions/internal/steam"
	"steam-suggestions/internal/store"
)

func RefreshTagDict(db *store.DB, client *steam.Client) error {
	tags, hash, err := client.TagList()
	if err != nil {
		return err
	}
	return db.ReplaceTagDict(tags, hash)
}

func IngestAppList(db *store.DB, client *steam.Client) error {
	if client.APIKey == "" {
		return fmt.Errorf("no API key")
	}
	last := 0
	more := true
	total := 0
	for more {
		apps, next, haveMore, err := client.AppListPage(last)
		if err != nil {
			return err
		}
		if err := db.UpsertGames(apps); err != nil {
			return err
		}
		total += len(apps)
		more = haveMore
		last = next
		log.Printf("applist: page %d apps (running total %d)", len(apps), total)
	}
	_ = db.SetMeta("applist_at", fmt.Sprintf("%d", time.Now().UnixMilli()))
	_ = db.SetMeta("applist_count", fmt.Sprintf("%d", total))
	return nil
}
