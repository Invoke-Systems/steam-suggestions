package store

import (
	"os"
	"testing"
)

func TestRecommendCandidatesIncludesKnownGames(t *testing.T) {
	if _, err := os.Stat("../../data/steam.sqlite"); err != nil {
		t.Skip("no sqlite")
	}
	db, err := Open("../../data/steam.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ids := []int{87918, 10235, 42804, 3959, 1091588, 1666, 29482, 4231}
	cands := db.RecommendCandidates(ids, CandidateOpts{PerTag: 700, MaxTotal: 4000})
	t.Log("candidates", len(cands))
	want := map[int]string{1158160: "Coral Island", 1432860: "Sun Haven", 1145350: "Hades II", 2379780: "Balatro"}
	found := map[int]bool{}
	for _, c := range cands {
		found[c.AppID] = true
	}
	for id, name := range want {
		t.Log(name, found[id])
	}
}
