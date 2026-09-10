package store

import (
	"path/filepath"
	"testing"
)

func TestAppDetailsRoundTrip(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "details.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	score := 88
	if err := db.SetAppDetails(AppDetails{
		AppID:            730,
		Type:             "game",
		Developers:       []string{"Valve"},
		Publishers:       []string{"Valve"},
		Genres:           []string{"Action"},
		Categories:       []string{"Multi-player"},
		ReleaseDate:      "21 Aug, 2012",
		Metacritic:       &score,
		ShortDescription: "CS2",
	}); err != nil {
		t.Fatal(err)
	}
	got := db.GetAppDetails([]int{730})[730]
	if got.Type != "game" || len(got.Developers) != 1 || got.Metacritic == nil || *got.Metacritic != 88 {
		t.Fatalf("%+v", got)
	}
	ids, err := db.NeedDetailsWork(10, 14*24*3600)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		// no tagged games in empty catalog
		t.Fatalf("unexpected %v", ids)
	}
}

func TestNewsAndAchievementsRoundTrip(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "meta.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.SetAppNews(570, []NewsItem{{
		AppID: 570, GID: "1", Title: "Patch", URL: "https://example.com", Date: 100,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetAchievements(570, []AchievementPct{{Name: "WIN", Percent: 12.5}}); err != nil {
		t.Fatal(err)
	}
	items, ok := db.GetAchievements(570)
	if !ok || len(items) != 1 || items[0].Percent != 12.5 {
		t.Fatalf("%v %v", items, ok)
	}
}
