package recommend

import "testing"

func TestRankOwnedFitsOrdersByTasteAndKeepsOwned(t *testing.T) {
	library := []Game{
		{AppID: 1, Name: "Roguelike Main", Hours: 40, Tags: []Tag{{Name: "Roguelike", Weight: 90}, {Name: "Action", Weight: 70}}},
		{AppID: 2, Name: "Strategy Main", Hours: 30, Tags: []Tag{{Name: "Strategy", Weight: 90}, {Name: "Turn-Based", Weight: 80}}},
		{AppID: 10, Name: "Roguelike Backlog", Hours: 0.5, Tags: []Tag{{Name: "Roguelike", Weight: 85}, {Name: "Action", Weight: 60}}},
		{AppID: 11, Name: "Strategy Backlog", Hours: 0, Tags: []Tag{{Name: "Strategy", Weight: 80}}},
		{AppID: 12, Name: "Unrelated Backlog", Hours: 0.2, Tags: []Tag{{Name: "Sports", Weight: 90}}},
	}
	candidates := []Game{library[2], library[3], library[4]}
	got := RankOwnedFits(library, candidates, 10)
	if len(got) != 3 {
		t.Fatalf("len %d", len(got))
	}
	if got[0].AppID != 10 {
		t.Fatalf("expected roguelike backlog first, got %+v", got[0])
	}
	if got[0].Fit <= got[2].Fit {
		t.Fatalf("fit order: %+v", got)
	}
}

func TestRankOwnedFitsNeedsTaste(t *testing.T) {
	library := []Game{
		{AppID: 1, Name: "Unplayed", Hours: 0, Tags: []Tag{{Name: "Action", Weight: 50}}},
	}
	if got := RankOwnedFits(library, library, 5); len(got) != 0 {
		t.Fatalf("expected empty without played taste, got %+v", got)
	}
}
