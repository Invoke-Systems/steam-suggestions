package web

import (
	"strings"
	"testing"

	"steam-suggestions/internal/recommend"
	"steam-suggestions/internal/store"
)

func TestSiftLibraryShowsTopTwelve(t *testing.T) {
	s := &Server{}
	games := make([]libraryGame, 40)
	for i := range games {
		games[i] = libraryGame{
			AppID: i + 1,
			Name:  "Game",
			Hours: float64(40 - i),
			Tags:  []string{"Roguelike", "Action"},
		}
	}
	view := s.siftFromPayload(map[string]any{
		"player": map[string]any{"name": "Ada"},
		"games":  games,
		"taste": map[string]any{
			"clusters": []recommend.TasteCluster{{Name: "Action-RPG", Percent: 22}},
		},
	})
	if view.PlayerName != "Ada" {
		t.Fatal(view.PlayerName)
	}
	if view.GameCount != 40 {
		t.Fatal(view.GameCount)
	}
	if !strings.Contains(view.Meta, "40 games") || strings.Contains(strings.ToLower(view.Meta), "week") {
		t.Fatal(view.Meta)
	}
	if len(view.Games) != maxSiftTop {
		t.Fatalf("top %d", len(view.Games))
	}
	if len(view.MoreGames) != 28 {
		t.Fatalf("more %d", len(view.MoreGames))
	}
	if view.Clusters[0].Key != "action rpg" || view.Clusters[0].Name != "Action RPG" {
		t.Fatalf("%+v", view.Clusters[0])
	}
	if !strings.Contains(view.Games[0].Tags, "|roguelike|") {
		t.Fatal(view.Games[0].Tags)
	}
	if strings.HasPrefix(view.Games[0].Header, "/images/") {
		t.Fatal("sift covers should use Steam CDN, not placeholders")
	}
}

func TestSiftCapsLibraryAtEighty(t *testing.T) {
	s := &Server{}
	games := make([]libraryGame, 120)
	for i := range games {
		games[i] = libraryGame{AppID: i + 1, Name: "Game", Hours: float64(120 - i)}
	}
	view := s.siftFromPayload(map[string]any{"games": games})
	if len(view.Games)+len(view.MoreGames) != maxSiftGames {
		t.Fatalf("got %d", len(view.Games)+len(view.MoreGames))
	}
}

func TestFormatPlayersSkipsZeros(t *testing.T) {
	if formatPlayers(store.PlayerStats{}) != "" {
		t.Fatal("empty stats should stay blank")
	}
	got := formatPlayers(store.PlayerStats{Current: 1200, PeakAll: 54000})
	if got != "1,200 in-game · peak 54,000" {
		t.Fatal(got)
	}
}

func TestFormatPriceSkipsUnlisted(t *testing.T) {
	if formatPrice(recommend.Card{Price: "Free or unlisted"}) != "" {
		t.Fatal("unlisted")
	}
	got := formatPrice(recommend.Card{Price: "$4.99", AtLow: true})
	if got != "$4.99 · Steam low" {
		t.Fatal(got)
	}
}
