package main

import (
	"log"
	"net/http"
	"path/filepath"
	"time"

	"steam-suggestions/internal/env"
	"steam-suggestions/internal/steam"
	"steam-suggestions/internal/store"
	"steam-suggestions/internal/web"
)

func main() {
	env.Load(".env")
	root := web.FindRoot()
	env.Load(filepath.Join(root, ".env"))

	dbPath := env.Get("STEAM_DB", filepath.Join(root, "data", "steam.sqlite"))
	db, err := store.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	catalog, fallback, err := web.LoadCatalog(root)
	if err != nil {
		log.Fatal(err)
	}

	client := steam.New(env.Get("STEAM_API_KEY", ""), env.Get("STEAM_CC", "US"), env.Get("STEAM_CURRENCY", "USD"))
	server := web.New(root, db, client, catalog, fallback)
	server.ImageDir = filepath.Join(filepath.Dir(dbPath), "images")
	server.PublicURL = env.Get("PUBLIC_URL", "")
	server.Warmup()

	addr := ":" + env.Get("PORT", "3847")
	log.Printf("Should I Play running at http://localhost%s", addr)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Fatal(httpServer.ListenAndServe())
}
