package web

import (
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"steam-suggestions/internal/steam"
)

var searchTmpl = template.Must(template.ParseFS(tmplFS, "templates/search.html"))

type searchView struct {
	Query     string
	Games     []searchHit
	SignedIn  bool
	Analytics template.HTML
}

type searchHit struct {
	Name   string
	Href   string
	Header string
}

func (s *Server) handleSearchGamesAPI(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) > 80 {
		q = q[:80]
	}
	limit := 8
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 40 {
			limit = n
		}
	}
	rows := s.DB.SearchGames(q, limit)
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"appid":   row.AppID,
			"name":    row.Name,
			"pageUrl": steam.AppURL(row.AppID),
			"header":  steam.CachedCoverURL(row.AppID),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"games": out, "q": q})
}

func (s *Server) handleSearchPage(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) > 80 {
		q = q[:80]
	}
	view := searchView{Query: q, Analytics: analyticsHTML()}
	_, view.SignedIn = s.sessionUser(r)
	if q != "" {
		for _, row := range s.DB.SearchGames(q, 40) {
			view.Games = append(view.Games, searchHit{
				Name:   row.Name,
				Href:   steam.AppURL(row.AppID),
				Header: steam.CachedCoverURL(row.AppID),
			})
		}
	}
	setHTMLHeaders(w, htmlHeaderOpts{})
	if err := searchTmpl.Execute(w, view); err != nil {
		log.Printf("search template: %v", err)
	}
}
