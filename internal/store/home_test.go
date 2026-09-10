package store

import (
	"path/filepath"
	"testing"
)

func TestHomeRailsQueries(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "home.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_ = db.UpsertGame(1, "Mega Hit")
	_ = db.UpsertGame(2, "Deep Sale")
	_ = db.UpsertGame(3, "Sleeper Gem")
	_ = db.UpsertGame(4, "Busy Now")
	_ = db.UpsertGame(9_000_000, "Brand New Hot")
	_ = db.SetReviews(1, 50000, 95, 99)
	_ = db.SetReviews(2, 800, 80, 70)
	_ = db.SetReviews(3, 1200, 96, 80)
	_ = db.SetReviews(9_000_000, 900, 94, 80)
	_ = db.SetPrice(2, Price{Name: "Deep Sale", T: 1, Currency: "USD", Initial: 1999, Final: 499, Discount: 75, Formatted: "$4.99"})
	_ = db.SetPrice(1, Price{Name: "Mega Hit", T: 1, Currency: "USD", Initial: 5999, Final: 5999, Discount: 0, Formatted: "$59.99"})
	if err := db.SetPlayerSample(4, 12000); err != nil {
		t.Fatal(err)
	}
	if err := db.SetPlayerSample(9_000_000, 800); err != nil {
		t.Fatal(err)
	}
	if err := db.SetPlayerSample(1, 800); err != nil {
		t.Fatal(err)
	}

	rails := db.HomeRails(8)
	if len(rails.NewWithPlayers) < 1 || rails.NewWithPlayers[0].AppID != 9_000_000 {
		t.Fatalf("new with players %+v", rails.NewWithPlayers)
	}
	if len(rails.RisingReviews) < 1 || rails.RisingReviews[0].AppID != 9_000_000 {
		t.Fatalf("rising %+v", rails.RisingReviews)
	}
	if len(rails.OnSale) != 1 || rails.OnSale[0].AppID != 2 || rails.OnSale[0].Discount != 75 {
		t.Fatalf("on sale %+v", rails.OnSale)
	}
	if len(rails.TopPlayers) < 1 || rails.TopPlayers[0].AppID != 4 {
		t.Fatalf("top players %+v", rails.TopPlayers)
	}
}
