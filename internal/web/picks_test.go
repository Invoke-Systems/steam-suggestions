package web

import (
	"path/filepath"
	"strings"
	"testing"

	"steam-suggestions/internal/steam"
	"steam-suggestions/internal/store"
)

func TestPicksFromPayloadRanksUnderTwoHours(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "picks.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.ReplaceTagDict([]store.DictTag{
		{TagID: 1, Name: "Roguelike"},
		{TagID: 2, Name: "Action"},
		{TagID: 3, Name: "Sports"},
	}, "test"); err != nil {
		t.Fatal(err)
	}
	seed := []struct {
		id   int
		name string
		tags []store.GameTag
	}{
		{1, "Roguelike Main", []store.GameTag{{TagID: 1, Weight: 90}, {TagID: 2, Weight: 70}}},
		{10, "Roguelike Shelf", []store.GameTag{{TagID: 1, Weight: 85}}},
		{11, "Sports Shelf", []store.GameTag{{TagID: 3, Weight: 90}}},
	}
	for _, row := range seed {
		if err := db.SetGameTags(row.id, row.name, row.tags, true); err != nil {
			t.Fatal(err)
		}
	}

	s := &Server{DB: db, Client: steam.New("", "US", "USD")}
	view := s.picksFromPayload(map[string]any{
		"player": map[string]any{"name": "Ada"},
		"games": []libraryGame{
			{AppID: 1, Name: "Roguelike Main", Minutes: 2400, Hours: 40},
			{AppID: 10, Name: "Roguelike Shelf", Minutes: 30, Hours: 0.5},
			{AppID: 11, Name: "Sports Shelf", Minutes: 0, Hours: 0},
			{AppID: 12, Name: "Deep Played", Minutes: 600, Hours: 10},
		},
	})
	if view.PlayerName != "Ada" {
		t.Fatal(view.PlayerName)
	}
	if view.CandidateN != 2 {
		t.Fatalf("candidates %d", view.CandidateN)
	}
	if len(view.Cards) != 2 {
		t.Fatalf("cards %d empty=%q", len(view.Cards), view.EmptyHint)
	}
	if view.Cards[0].Name != "Roguelike Shelf" {
		t.Fatalf("first %+v", view.Cards[0])
	}
	if !strings.Contains(view.Cards[0].Meta, "m") && !strings.Contains(view.Cards[0].Meta, "Unplayed") {
		t.Fatalf("meta %q", view.Cards[0].Meta)
	}
}

func TestPicksEmptyWithoutTaste(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "picks2.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := &Server{DB: db, Client: steam.New("", "US", "USD")}
	view := s.picksFromPayload(map[string]any{
		"games": []libraryGame{
			{AppID: 10, Name: "Shelf", Minutes: 10, Hours: 0.1},
		},
	})
	if len(view.Cards) != 0 || view.EmptyHint == "" {
		t.Fatalf("%+v", view)
	}
}
