package recommend

import (
	"os"
	"testing"

	"steam-suggestions/internal/store"
)

func TestRealLibraryPrefersKnownNeighbors(t *testing.T) {
	path := "../../data/steam.sqlite"
	if _, err := os.Stat(path); err != nil {
		t.Skip("no sqlite")
	}
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	lib := []struct {
		id     int
		name   string
		hours  float64
		recent bool
	}{
		{413150, "Stardew Valley", 186, true},
		{1245620, "ELDEN RING", 97, false},
		{1145360, "Hades", 74, false},
		{646570, "Slay the Spire", 61, true},
	}
	ids := make([]int, len(lib))
	for i, g := range lib {
		ids[i] = g.id
	}
	tagMap := db.GetTags(ids)
	var games []Game
	for _, g := range lib {
		games = append(games, Game{AppID: g.id, Name: g.name, Hours: g.hours, RecentlyPlayed: g.recent, Tags: tagMap[g.id]})
	}

	poolIDs := []int{1158160, 1432860, 599140, 2379780, 1145350, 2354050, 3010150, 1777020}
	poolTags := db.GetTags(poolIDs)
	names := map[int]string{1158160: "Coral Island", 1432860: "Sun Haven", 599140: "Graveyard Keeper", 2379780: "Balatro", 1145350: "Hades II", 2354050: "Dreieck", 3010150: "BLOODY HELL 2", 1777020: "Corpsenia"}
	var catalog []Game
	for _, id := range poolIDs {
		catalog = append(catalog, Game{AppID: id, Name: names[id], Tags: poolTags[id], Popularity: 50})
	}
	result := Recommend(games, catalog, Options{Popularity: 0.5, Weirdness: 0.25})
	for _, c := range result.Groups.MoreLike {
		t.Logf("more %d %s fit=%.3f", c.AppID, c.Name, c.Fit)
	}
	top := result.Groups.MoreLike[0]
	if top.Fit < 0.4 {
		t.Fatalf("expected a strong neighbor first, got %d %s fit=%.3f", top.AppID, top.Name, top.Fit)
	}
}
