package jobs

import (
	"log"

	"steam-suggestions/internal/store"
)

// LogCatalogStatus prints DB coverage and whether a catch-up crawl is needed.
func LogCatalogStatus(db *store.DB) store.Stats {
	stats, err := db.Stats()
	if err != nil {
		log.Printf("catalog status: unavailable (%v)", err)
		return store.Stats{}
	}
	log.Printf("catalog status: %s", stats.Summary())
	if stats.NeedsCatchUp() {
		log.Printf("catalog status: CATCH-UP NEEDED (thin SQLite — first worker pass will backfill)")
	} else {
		log.Printf("catalog status: healthy enough for interim operation")
	}
	return stats
}
