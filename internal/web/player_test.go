package web

import (
	"strings"
	"testing"

	"steam-suggestions/internal/recommend"
	"steam-suggestions/internal/steam"
)

func TestPlayerFromPayloadBuildsShareableLibrary(t *testing.T) {
	s := &Server{}
	view := s.playerFromPayload("76561198000000000", map[string]any{
		"player": map[string]any{
			"name":       "Ada",
			"avatar":     "https://avatars.steamstatic.com/x.jpg",
			"profileUrl": "https://steamcommunity.com/id/ada/",
		},
		"games": []libraryGame{
			{AppID: 10, Name: "Hades", Minutes: 6000, Hours: 100, Tags: []string{"Roguelike"}},
			{AppID: 11, Name: "Celeste", Minutes: 1200, Hours: 20, Tags: []string{"Platformer"}},
		},
		"taste": map[string]any{
			"clusters": []recommend.TasteCluster{{Name: "Roguelike", Percent: 40}},
		},
	})
	if view.PlayerName != "Ada" {
		t.Fatal(view.PlayerName)
	}
	if view.ShareURL != steam.PlayerURL("76561198000000000") {
		t.Fatal(view.ShareURL)
	}
	if view.GameCount != 2 || len(view.Games) != 2 {
		t.Fatalf("games %+v", view.Games)
	}
	if !strings.Contains(view.Meta, "2 games") {
		t.Fatal(view.Meta)
	}
	if len(view.Clusters) != 1 || view.Clusters[0].Name != "Roguelike" {
		t.Fatalf("%+v", view.Clusters)
	}
	if view.Games[0].Href != "/app/10" {
		t.Fatal(view.Games[0].Href)
	}
}

func TestPlayerURL(t *testing.T) {
	if steam.PlayerURL("76561198000000000") != "/u/76561198000000000" {
		t.Fatal(steam.PlayerURL("76561198000000000"))
	}
}
