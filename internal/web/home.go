package web

import (
	"net/http"
	"time"

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
	rails := s.DB.HomeRails(24)
	s.refreshHomeNames(&rails)
	return rails
}

func (s *Server) refreshHomeNames(rails *store.HomeRails) {
	if rails == nil {
		return
	}
	ids := make([]int, 0, 96)
	add := func(items []store.HomeItem) {
		for _, item := range items {
			if item.AppID > 0 {
				ids = append(ids, item.AppID)
			}
		}
	}
	add(rails.Hot)
	add(rails.NewWithPlayers)
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
	apply(rails.Hot)
	apply(rails.NewWithPlayers)
	apply(rails.OnSale)
	apply(rails.TopPlayers)
}

func (s *Server) cachedHome() (store.HomeRails, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.homeAt) > homeFresh {
		return store.HomeRails{}, false
	}
	if len(s.home.Hot) == 0 && len(s.home.NewWithPlayers) == 0 &&
		len(s.home.OnSale) == 0 && len(s.home.TopPlayers) == 0 {
		return store.HomeRails{}, false
	}
	return s.home, true
}
