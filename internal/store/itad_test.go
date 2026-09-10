package store

import (
	"path/filepath"
	"testing"
)

func TestPriceLowMergesIntoGetPrices(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "itad.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.SetPrice(10, Price{
		Name: "Demo", T: 1_700_000_000_000, Currency: "USD", Initial: 1999, Final: 999, Discount: 50, Formatted: "$9.99",
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetPriceLow(PriceLow{AppID: 10, Cents: 499, Currency: "USD", Shop: "Steam", ShopID: 61}); err != nil {
		t.Fatal(err)
	}
	got := db.GetPrices([]int{10})[10]
	if got.Low == nil || *got.Low != "$4.99" {
		t.Fatalf("%+v", got)
	}
	if !got.AtLow {
		// 999 > 499, on sale, so at-low should be false
	}
	if got.AtLow {
		t.Fatal("current 9.99 should not be at 4.99 low")
	}
}

func TestITADLookupQueue(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "map.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_ = db.UpsertGame(1, "A")
	_ = db.UpsertGame(100, "Newer")
	_ = db.SetReviews(1, 99999, 90, 80)
	ids, err := db.NeedITADLookup(10, ITADOrderNewest, 0)
	if err != nil || len(ids) != 2 {
		t.Fatalf("%v %v", ids, err)
	}
	if ids[0] != 100 {
		t.Fatalf("expected newer appid first, got %v", ids)
	}
	ids, err = db.NeedITADLookup(10, ITADOrderReviews, 0)
	if err != nil || len(ids) != 2 || ids[0] != 1 {
		t.Fatalf("expected reviews order (app 1 first), got %v %v", ids, err)
	}
	ids, err = db.NeedITADLookup(10, ITADOrderNewest, 1)
	if err != nil || len(ids) != 1 || ids[0] != 1 {
		t.Fatalf("expected min-reviews to skip unreviewed, got %v %v", ids, err)
	}
	if err := db.SetITADMap(100, "gid-100"); err != nil {
		t.Fatal(err)
	}
	if err := db.SetITADMap(1, "gid-1"); err != nil {
		t.Fatal(err)
	}
	ids, err = db.NeedITADLookup(10, ITADOrderNewest, 0)
	if err != nil || len(ids) != 0 {
		t.Fatalf("expected empty after map, got %v", ids)
	}
	rows, err := db.NeedITADLowFill(10, ITADOrderReviews, 0)
	if err != nil || len(rows) != 2 || rows[0].AppID != 1 {
		t.Fatalf("reviews order fill: %+v %v", rows, err)
	}
	rows, err = db.NeedITADLowFill(10, ITADOrderNewest, 50)
	if err != nil || len(rows) != 1 || rows[0].AppID != 1 {
		t.Fatalf("min-reviews fill: %+v %v", rows, err)
	}
}
