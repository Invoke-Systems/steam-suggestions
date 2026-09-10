package steam

import (
	"encoding/json"
	"testing"
)

func TestPlaytimeMinutesFallsBackToPlatformFields(t *testing.T) {
	var game OwnedGame
	if err := json.Unmarshal([]byte(`{
		"appid": 730,
		"name": "Counter-Strike 2",
		"playtime_forever": 0,
		"playtime_windows_forever": 270000,
		"rtime_last_played": 0
	}`), &game); err != nil {
		t.Fatal(err)
	}
	got := game.PlaytimeMinutes()
	if got != 270000 {
		t.Fatalf("expected 270000 minutes from windows playtime, got %d", got)
	}
	if MinutesToHours(got) < 4500 {
		t.Fatalf("expected ~4500 hours, got %.1f", MinutesToHours(got))
	}
}

func TestPlayedRecentlyUsesLastPlayedWindow(t *testing.T) {
	now := int64(1_700_000_000)
	game := OwnedGame{RtimeLastPlayed: now - 3*24*60*60}
	if !game.PlayedRecently(now) {
		t.Fatal("played 3 days ago should count as recent")
	}
	old := OwnedGame{RtimeLastPlayed: now - 40*24*60*60}
	if old.PlayedRecently(now) {
		t.Fatal("played 40 days ago should not count as recent")
	}
}
