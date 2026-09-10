package web

import (
	"net/http"
	"strconv"
	"time"

	"steam-suggestions/internal/steam"
	"steam-suggestions/internal/store"
)

const homeFresh = 5 * time.Minute

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if cached, ok := s.cachedHome(); ok {
		writeJSON(w, http.StatusOK, cached)
		return
	}
	rails := s.buildHomeRails()
	s.mu.Lock()
	s.home = rails
	s.homeAt = time.Now()
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, rails)
}

func (s *Server) buildHomeRails() store.HomeRails {
	rails := s.DB.HomeRails(12)
	featured, ok := s.cachedStorefront()
	if !ok {
		featured = s.Client.FeaturedCategories()
		s.mu.Lock()
		if len(featured.Specials) > 0 || len(featured.NewReleases) > 0 || len(featured.TopSellers) > 0 {
			s.storefront = featured
			s.storefrontAt = time.Now()
		}
		s.mu.Unlock()
	}
	if rising := s.risingFromSteam(featured); len(rising) > 0 {
		rails.RisingReviews = rising
	}
	s.refreshHomeNames(&rails)
	return rails
}

func (s *Server) refreshHomeNames(rails *store.HomeRails) {
	if rails == nil {
		return
	}
	ids := make([]int, 0, 48)
	add := func(items []store.HomeItem) {
		for _, item := range items {
			if item.AppID > 0 {
				ids = append(ids, item.AppID)
			}
		}
	}
	add(rails.NewWithPlayers)
	add(rails.RisingReviews)
	add(rails.OnSale)
	add(rails.TopPlayers)
	if len(ids) == 0 {
		return
	}
	items, err := s.Client.GetItems(ids)
	if err != nil || len(items) == 0 {
		return
	}
	fresh := map[int]string{}
	for _, item := range items {
		if item.AppID > 0 && item.Name != "" {
			fresh[item.AppID] = item.Name
			_ = s.DB.UpsertGame(item.AppID, item.Name)
		}
	}
	apply := func(items []store.HomeItem) {
		for i := range items {
			if name := fresh[items[i].AppID]; name != "" {
				items[i].Name = name
			}
		}
	}
	apply(rails.NewWithPlayers)
	apply(rails.RisingReviews)
	apply(rails.OnSale)
	apply(rails.TopPlayers)
}

// risingFromSteam prefers Steam new releases / top sellers that also look strong in our review table.
func (s *Server) risingFromSteam(featured steam.Featured) []store.HomeItem {
	seen := map[int]bool{}
	var ids []int
	var order []steam.SaleItem
	for _, list := range [][]steam.SaleItem{featured.NewReleases, featured.TopSellers} {
		for _, item := range list {
			if item.AppID <= 0 || item.Name == "" || seen[item.AppID] {
				continue
			}
			seen[item.AppID] = true
			ids = append(ids, item.AppID)
			order = append(order, item)
		}
	}
	if len(order) == 0 {
		return nil
	}
	reviews := s.DB.GetReviews(ids)
	var strong []store.HomeItem
	var rest []store.HomeItem
	for _, item := range order {
		meta := "Steam new / hot"
		if rev, ok := reviews[item.AppID]; ok && rev.Total > 0 && rev.Positive > 0 {
			meta = strconv.Itoa(rev.Positive) + "% of " + store.CommaInt(rev.Total) + " reviews"
		}
		card := store.HomeItemFromApp(item.AppID, item.Name, meta)
		if item.Header != "" {
			card.Header = item.Header
		}
		if item.Discount > 0 {
			card.Discount = item.Discount
			card.Formatted = item.Formatted
		}
		if rev, ok := reviews[item.AppID]; ok && rev.Positive >= 85 && rev.Total >= 100 {
			strong = append(strong, card)
		} else {
			rest = append(rest, card)
		}
	}
	out := append(strong, rest...)
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}


func (s *Server) cachedHome() (store.HomeRails, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.homeAt) > homeFresh {
		return store.HomeRails{}, false
	}
	if len(s.home.NewWithPlayers) == 0 && len(s.home.RisingReviews) == 0 &&
		len(s.home.OnSale) == 0 && len(s.home.TopPlayers) == 0 {
		return store.HomeRails{}, false
	}
	return s.home, true
}
