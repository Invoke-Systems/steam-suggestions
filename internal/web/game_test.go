package web

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"steam-suggestions/internal/steam"
	"steam-suggestions/internal/store"
)

func TestHandleGameRendersCollectedMetrics(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "game.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.UpsertGame(730, "Counter-Strike 2"); err != nil {
		t.Fatal(err)
	}
	if err := db.SetReviews(730, 1000, 88, 70); err != nil {
		t.Fatal(err)
	}
	if err := db.SetPlayerSample(730, 4200); err != nil {
		t.Fatal(err)
	}
	score := 81
	if err := db.SetAppDetails(store.AppDetails{
		AppID:            730,
		Type:             "game",
		Developers:       []string{"Valve"},
		ShortDescription: "Tactical FPS",
		Metacritic:       &score,
	}); err != nil {
		t.Fatal(err)
	}

	s := &Server{DB: db, Client: steam.New("", "US", "USD")}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/app/730", nil)
	req.SetPathValue("appid", "730")
	s.handleGame(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Counter-Strike 2", "Open on Steam", "88%", "4,200", "Tactical FPS"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, body[:min(400, len(body))])
		}
	}
	if !strings.Contains(body, `href="https://store.steampowered.com/app/730/"`) {
		t.Fatal("expected Steam store CTA href")
	}
}

func TestAppURL(t *testing.T) {
	if steam.AppURL(730) != "/app/730" {
		t.Fatal(steam.AppURL(730))
	}
}
