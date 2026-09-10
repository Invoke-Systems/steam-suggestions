package store

import (
	"path/filepath"
	"testing"
)

func TestReviewSnippetsRoundTrip(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "snippets.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	items := []ReviewSnippet{{
		AppID:     570,
		ReviewID:  "abc",
		SteamID:   "76561198000000000",
		VotedUp:   true,
		Hours:     12,
		CreatedAt: 1700000000,
		Text:      "Still one of the best MOBA matches.",
		URL:       "https://steamcommunity.com/profiles/76561198000000000/recommended/570/",
	}}
	if err := db.SetReviewSnippets(570, items); err != nil {
		t.Fatal(err)
	}
	got, fetchedAt, ok := db.GetReviewSnippets(570, 5)
	if !ok || fetchedAt <= 0 || len(got) != 1 {
		t.Fatalf("got %+v ok=%v fetched=%d", got, ok, fetchedAt)
	}
	if got[0].Text != items[0].Text || !got[0].VotedUp || got[0].Hours != 12 {
		t.Fatalf("%+v", got[0])
	}
}
