package store

import (
	"path/filepath"
	"testing"
)

func TestPlayerSampleKeepsPeak(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "players.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.SetPlayerSample(730, 1200); err != nil {
		t.Fatal(err)
	}
	if err := db.SetPlayerSample(730, 80); err != nil {
		t.Fatal(err)
	}
	got := db.GetPlayerStats([]int{730, 10})
	st, ok := got[730]
	if !ok {
		t.Fatal("missing stats")
	}
	if st.Current != 80 {
		t.Fatalf("current %d", st.Current)
	}
	if st.PeakAll != 1200 {
		t.Fatalf("peak %d", st.PeakAll)
	}
	if st.PeakDay < 80 {
		t.Fatalf("peak day %d", st.PeakDay)
	}
	if _, ok := got[10]; ok {
		t.Fatal("unexpected app")
	}
}

func TestPrunePriceHistoryKeepsRecent(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "prices.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PrunePriceHistory(730); err != nil {
		t.Fatal(err)
	}
}
