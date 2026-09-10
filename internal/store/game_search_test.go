package store

import (
	"path/filepath"
	"testing"
)

func TestSearchGamesOrdersPrefixMatches(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "search.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.UpsertGame(1, "Hades"); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertGame(2, "Shadow of the Tomb Raider"); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertGame(3, "Hadestown"); err != nil {
		t.Fatal(err)
	}
	_ = db.SetReviews(2, 5000, 90, 80)
	got := db.SearchGames("hade", 10)
	if len(got) < 2 {
		t.Fatalf("%+v", got)
	}
	if got[0].Name != "Hades" && got[0].Name != "Hadestown" {
		t.Fatalf("expected prefix hit first, got %+v", got[0])
	}
}
