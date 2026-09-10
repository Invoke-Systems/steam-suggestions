package steam

import "testing"

func TestParseFeaturedCategories(t *testing.T) {
	got := parseFeaturedCategories([]byte(`{
		"specials": {"items": [
			{"id": 10, "name": "Sale Game", "discount_percent": 40, "final_price": 599, "large_capsule_image": "https://cdn/sale.jpg"}
		]},
		"coming_soon": {"items": [
			{"id": 20, "name": "Soon Game", "discount_percent": 0, "final_price": 1999, "small_capsule_image": "https://cdn/soon.jpg"}
		]},
		"new_releases": {"items": [
			{"id": 30, "name": "New Game", "discount_percent": 0, "final_price": 2999, "large_capsule_image": "https://cdn/new.jpg"}
		]},
		"top_sellers": {"items": [
			{"id": 40, "name": "Hot Game", "discount_percent": 10, "final_price": 2699, "large_capsule_image": "https://cdn/hot.jpg"}
		]}
	}`))
	if len(got.Specials) != 1 || got.Specials[0].AppID != 10 || got.Specials[0].Discount != 40 || got.Specials[0].Formatted != "$5.99" {
		t.Fatalf("specials: %+v", got.Specials)
	}
	if len(got.ComingSoon) != 1 || got.ComingSoon[0].AppID != 20 || got.ComingSoon[0].SteamURL != "https://store.steampowered.com/app/20/" {
		t.Fatalf("coming soon: %+v", got.ComingSoon)
	}
	if got.ComingSoon[0].Header != "https://cdn/soon.jpg" {
		t.Fatalf("header fallback: %q", got.ComingSoon[0].Header)
	}
	if len(got.NewReleases) != 1 || got.NewReleases[0].AppID != 30 {
		t.Fatalf("new releases: %+v", got.NewReleases)
	}
	if len(got.TopSellers) != 1 || got.TopSellers[0].AppID != 40 {
		t.Fatalf("top sellers: %+v", got.TopSellers)
	}
}
