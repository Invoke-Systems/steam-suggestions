package web

import (
	"net/http"
	"time"

	"steam-suggestions/internal/steam"
)

const storefrontFresh = 30 * time.Minute

func (s *Server) handleStorefront(w http.ResponseWriter, r *http.Request) {
	if cached, ok := s.cachedStorefront(); ok {
		writeJSON(w, http.StatusOK, cached)
		return
	}
	data := s.Client.FeaturedCategories()
	s.mu.Lock()
	if len(data.Specials) > 0 || len(data.ComingSoon) > 0 {
		s.storefront = data
		s.storefrontAt = time.Now()
	} else if time.Since(s.storefrontAt) < 6*time.Hour && (len(s.storefront.Specials) > 0 || len(s.storefront.ComingSoon) > 0) {
		data = s.storefront
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) cachedStorefront() (steam.Featured, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.storefrontAt) > storefrontFresh {
		return steam.Featured{}, false
	}
	if len(s.storefront.Specials) == 0 && len(s.storefront.ComingSoon) == 0 {
		return steam.Featured{}, false
	}
	return s.storefront, true
}
