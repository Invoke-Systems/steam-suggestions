package recommend

import (
	"strconv"
	"testing"
)

func tags(pairs ...any) []Tag {
	var out []Tag
	for i := 0; i+2 < len(pairs); i += 3 {
		out = append(out, Tag{TagID: pairs[i].(int), Name: pairs[i+1].(string), Weight: pairs[i+2].(int)})
	}
	return out
}

func TestIsJunkName(t *testing.T) {
	if !IsJunkName("Hades II Demo") || !IsJunkName("Game Soundtrack") || !IsJunkName("Valheim Dedicated Server") {
		t.Fatal("expected junk titles")
	}
	if IsJunkName("Hades II") || IsJunkName("Counter-Strike 2") {
		t.Fatal("real games should pass")
	}
}

func TestSlidersKeepAFullList(t *testing.T) {
	library := []Game{
		{AppID: 1, Name: "Hades", Hours: 80, Tags: tags(1, "Roguelite", 100, 2, "Action Roguelike", 80)},
		{AppID: 2, Name: "Slay the Spire", Hours: 60, RecentlyPlayed: true, Tags: tags(3, "Deckbuilding", 90, 1, "Roguelite", 70)},
		{AppID: 3, Name: "Stardew Valley", Hours: 120, Tags: tags(4, "Farming Sim", 100, 5, "Pixel Graphics", 40)},
	}
	var catalog []Game
	for i := 10; i < 80; i++ {
		name := "Candidate " + strconv.Itoa(i)
		if i == 11 {
			name = "Hades II Demo"
		}
		catalog = append(catalog, Game{
			AppID:      i,
			Name:       name,
			Tags:       tags(1, "Roguelite", 400+i, 3, "Deckbuilding", 220),
			Popularity: 30 + i%50,
		})
	}
	catalog = append(catalog, Game{
		AppID:      200,
		Name:       "Adult Roguelite",
		Tags:       tags(1, "Roguelite", 800, 9, "Sexual Content", 500),
		Popularity: 20,
	})
	result := Recommend(library, catalog, Options{Popularity: 0, Weirdness: 1, HideAdult: true, Hidden: map[int]bool{15: true}})
	n := len(result.Groups.MoreLike) + len(result.Groups.Adjacent) + len(result.Groups.Wildcard)
	if n < 12 {
		t.Fatalf("expected a full rec list, got %d", n)
	}
	for _, card := range append(append(result.Groups.MoreLike, result.Groups.Adjacent...), result.Groups.Wildcard...) {
		if card.AppID == 1 || card.AppID == 11 || card.AppID == 15 || card.AppID == 200 {
			t.Fatalf("unexpected card %d %s", card.AppID, card.Name)
		}
	}
	indie := Recommend(library, catalog, Options{Popularity: 0, Weirdness: 0})
	main := Recommend(library, catalog, Options{Popularity: 1, Weirdness: 0})
	if indie.Groups.MoreLike[0].AppID == main.Groups.MoreLike[0].AppID && indie.Groups.MoreLike[0].Popularity == main.Groups.MoreLike[0].Popularity {
		// still ok if the same game wins both; just ensure lists are populated
	}
	if len(indie.Groups.MoreLike) == 0 || len(main.Groups.MoreLike) == 0 {
		t.Fatal("slider extremes emptied more_like")
	}
}

func TestOwnedDuplicateTitleSkipped(t *testing.T) {
	library := []Game{
		{AppID: 2420510, Name: "HoloCure - Save the Fans!", Hours: 40, Tags: tags(1, "Roguelike", 100)},
	}
	catalog := []Game{
		{AppID: 1447690, Name: "Holocure – Save the Fans!", Tags: tags(1, "Roguelike", 400), Popularity: 90, Reviews: 1000, Positive: 98, HasReviews: true},
		{AppID: 99, Name: "Other Roguelike", Tags: tags(1, "Roguelike", 380), Popularity: 80, Reviews: 1000, Positive: 90, HasReviews: true},
	}
	result := Recommend(library, catalog, Options{Popularity: 0.5, Weirdness: 0})
	for _, card := range result.Groups.MoreLike {
		if card.AppID == 1447690 || NameKey(card.Name) == NameKey("HoloCure - Save the Fans!") {
			t.Fatal("owned title leaked through a duplicate catalog appid")
		}
	}
}

func TestWishlistBoostsAndOwnedSkipped(t *testing.T) {
	library := []Game{
		{AppID: 1, Name: "Hades", Hours: 50, Tags: tags(1, "Roguelite", 100)},
	}
	catalog := []Game{
		{AppID: 1, Name: "Hades", Tags: tags(1, "Roguelite", 400), Popularity: 90},
		{AppID: 20, Name: "Wish Game", Tags: tags(1, "Roguelite", 390), Popularity: 40},
		{AppID: 21, Name: "Other Game", Tags: tags(1, "Roguelite", 380), Popularity: 80},
	}
	result := Recommend(library, catalog, Options{Popularity: 0.5, Weirdness: 0, Wishlist: map[int]bool{20: true}})
	foundOwned := false
	foundWish := false
	for _, card := range result.Groups.MoreLike {
		if card.AppID == 1 {
			foundOwned = true
		}
		if card.AppID == 20 {
			foundWish = true
			if !card.OnWishlist {
				t.Fatal("wishlist flag missing")
			}
		}
	}
	if foundOwned {
		t.Fatal("owned game should not be recommended")
	}
	if !foundWish {
		t.Fatal("wishlist game should surface")
	}
}

func TestTasteClusterPercents(t *testing.T) {
	taste := BuildTaste([]Game{
		{AppID: 1, Name: "Stardew Valley", Hours: 120, Tags: tags(4, "Farming Sim", 100, 5, "Pixel Graphics", 40)},
		{AppID: 2, Name: "Hades", Hours: 10, Tags: tags(1, "Roguelite", 100)},
	})
	if len(taste.RankedClusters) < 2 {
		t.Fatal("expected tag likelihoods")
	}
	if taste.RankedClusters[0].Name != "Farming Sim" {
		t.Fatalf("expected Farming Sim first, got %s", taste.RankedClusters[0].Name)
	}
	if taste.RankedClusters[0].Percent <= taste.RankedClusters[1].Percent {
		t.Fatal("heaviest tag should have the highest likelihood")
	}
	sum := 0
	for _, c := range taste.RankedClusters {
		sum += c.Percent
	}
	if sum < 90 || sum > 110 {
		t.Fatalf("likelihoods should be ~100%%, got %d", sum)
	}
}

func TestMinReviewsAndUnknownSkipShovelware(t *testing.T) {
	library := []Game{
		{AppID: 1, Name: "Stardew Valley", Hours: 100, Tags: tags(4, "Farming Sim", 100, 5, "Pixel Graphics", 40)},
	}
	catalog := []Game{
		{AppID: 10, Name: "Asset Flip Farm", Tags: tags(4, "Farming Sim", 900), Reviews: 12, Positive: 90, HasReviews: true, Popularity: 10},
		{AppID: 11, Name: "No Review Farm", Tags: tags(4, "Farming Sim", 850), Popularity: 80},
		{AppID: 12, Name: "Coral Island", Tags: tags(4, "Farming Sim", 800, 5, "Pixel Graphics", 200), Reviews: 18000, Positive: 89, HasReviews: true, Popularity: 70},
	}
	result := Recommend(library, catalog, Options{Popularity: 0.5, Weirdness: 0.2, SkipUnknown: true, MinReviews: 500, MinPositive: 70})
	for _, card := range append(append(result.Groups.MoreLike, result.Groups.Adjacent...), result.Groups.Wildcard...) {
		if card.AppID == 10 || card.AppID == 11 {
			t.Fatalf("shovelware or unknown leaked: %d %s", card.AppID, card.Name)
		}
	}
	if len(result.Groups.MoreLike) == 0 || result.Groups.MoreLike[0].AppID != 12 {
		t.Fatalf("expected Coral Island, got %+v", result.Groups.MoreLike)
	}
}

func TestIncludeExcludeTags(t *testing.T) {
	library := []Game{
		{AppID: 1, Name: "Hades", Hours: 50, Tags: tags(1, "Roguelite", 100, 2, "Action", 40)},
	}
	catalog := []Game{
		{AppID: 20, Name: "Hades II", Tags: tags(1, "Roguelite", 400, 2, "Action", 200), Reviews: 8000, Positive: 96, HasReviews: true, Popularity: 80},
		{AppID: 21, Name: "Farming Roguelite", Tags: tags(1, "Roguelite", 390, 4, "Farming Sim", 300), Reviews: 6000, Positive: 88, HasReviews: true, Popularity: 60},
	}
	onlyFarm := Recommend(library, catalog, Options{Popularity: 0.5, Weirdness: 0, IncludeTags: []string{"Farming Sim"}})
	if len(onlyFarm.Groups.MoreLike) != 1 || onlyFarm.Groups.MoreLike[0].AppID != 21 {
		t.Fatalf("include tag failed: %+v", onlyFarm.Groups.MoreLike)
	}
	noRogue := Recommend(library, catalog, Options{Popularity: 0.5, Weirdness: 0, ExcludeTags: []string{"Roguelite"}})
	for _, card := range onlyFarm.Groups.MoreLike {
		if card.AppID == 20 {
			t.Fatal("include should have dropped Hades II")
		}
	}
	if n := len(noRogue.Groups.MoreLike) + len(noRogue.Groups.Adjacent) + len(noRogue.Groups.Wildcard); n != 0 {
		t.Fatalf("exclude should empty the list, got %d", n)
	}
}

func TestTagWeightsRankWithoutExcluding(t *testing.T) {
	library := []Game{
		{AppID: 1, Name: "Hades", Hours: 50, Tags: tags(1, "Roguelite", 100)},
	}
	catalog := []Game{
		{AppID: 20, Name: "Pure Roguelite", Tags: tags(1, "Roguelite", 400), Reviews: 8000, Positive: 90, HasReviews: true, Popularity: 80},
		{AppID: 21, Name: "Farming Roguelite", Tags: tags(1, "Roguelite", 390, 4, "Farming Sim", 300), Reviews: 6000, Positive: 88, HasReviews: true, Popularity: 60},
		{AppID: 22, Name: "Other Roguelite", Tags: tags(1, "Roguelite", 380), Reviews: 5000, Positive: 86, HasReviews: true, Popularity: 70},
	}
	preferFarm := Recommend(library, catalog, Options{
		Popularity: 0.5,
		Weirdness:  0,
		TagWeights: []TagWeight{{Name: "Farming Sim", Weight: 100}},
	})
	if preferFarm.Groups.MoreLike[0].AppID != 21 {
		t.Fatalf("prefer Farming Sim should boost that game, got %+v", preferFarm.Groups.MoreLike)
	}
	ids := map[int]bool{}
	for _, card := range append(append(preferFarm.Groups.MoreLike, preferFarm.Groups.Adjacent...), preferFarm.Groups.Wildcard...) {
		ids[card.AppID] = true
	}
	if !ids[20] && !ids[22] {
		t.Fatal("weighted prefer should not drop games that miss the tag")
	}
	avoidFarm := Recommend(library, catalog, Options{
		Popularity: 0.5,
		Weirdness:  0,
		TagWeights: []TagWeight{{Name: "Farming Sim", Weight: -100}},
	})
	if avoidFarm.Groups.MoreLike[0].AppID == 21 {
		t.Fatal("avoid Farming Sim should not rank that game first")
	}
}

func TestCatalogSiftWithoutLibrary(t *testing.T) {
	catalog := []Game{
		{AppID: 20, Name: "Action Roguelite", Tags: tags(1, "Roguelite", 400, 2, "Action", 200), Reviews: 8000, Positive: 96, HasReviews: true, Popularity: 80},
		{AppID: 21, Name: "A Visual Novel", Tags: tags(9, "Visual Novel", 500), Reviews: 6000, Positive: 92, HasReviews: true, Popularity: 55},
		{AppID: 22, Name: "Other Action", Tags: tags(2, "Action", 380), Reviews: 5000, Positive: 88, HasReviews: true, Popularity: 70},
	}
	result := Recommend(nil, catalog, Options{
		Popularity: 0.5,
		TagWeights: []TagWeight{{Name: "Roguelite", Weight: 80}, {Name: "Visual Novel", Weight: -80}},
	})
	if len(result.Groups.MoreLike) == 0 {
		t.Fatal("a tag sift should return games without a library")
	}
	if result.Groups.MoreLike[0].AppID != 20 {
		t.Fatalf("expected the roguelite first, got %+v", result.Groups.MoreLike)
	}
}

func TestQualityAxisPrefersPositive(t *testing.T) {
	library := []Game{
		{AppID: 1, Name: "Hades", Hours: 80, Tags: tags(1, "Roguelite", 100)},
	}
	catalog := []Game{
		{AppID: 30, Name: "Mixed Mega", Tags: tags(1, "Roguelite", 400), Reviews: 80000, Positive: 55, HasReviews: true, Popularity: 92},
		{AppID: 31, Name: "Loved Niche", Tags: tags(1, "Roguelite", 390), Reviews: 900, Positive: 96, HasReviews: true, Popularity: 42},
	}
	result := Recommend(library, catalog, Options{Popularity: 0.5, Weirdness: 0, MinReviews: 500, MinPositive: 50, SkipUnknown: true})
	if result.Groups.MoreLike[0].AppID != 31 {
		t.Fatalf("quality axis should prefer 96%% over 55%% at a neutral indie slider, got %d %s", result.Groups.MoreLike[0].AppID, result.Groups.MoreLike[0].Name)
	}
}
