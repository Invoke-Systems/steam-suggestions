package web

import (
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"steam-suggestions/internal/recommend"
	"steam-suggestions/internal/steam"
)

var picksTmpl = template.Must(template.ParseFS(tmplFS, "templates/picks.html"))

const maxOwnedPicks = 36

type picksView struct {
	SignedIn   bool
	ShareURL   string
	PlayerName string
	Avatar     string
	Meta       string
	CandidateN int
	Cards      []siftGame
	EmptyHint  string
	Analytics  template.HTML
}

func (s *Server) handleLibraryPicks(w http.ResponseWriter, r *http.Request) {
	steamid, ok := s.activeSteamID(r)
	if !ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	key, err := s.resolveKey("")
	if err != nil {
		http.Redirect(w, r, "/?err=key", http.StatusSeeOther)
		return
	}
	payload, err := s.libraryFor(s.clientFor(key), steamid)
	if err != nil {
		http.Redirect(w, r, "/?err="+lookupCode(err), http.StatusSeeOther)
		return
	}
	view := s.picksFromPayload(payload)
	view.ShareURL = steam.PlayerURL(steamid)
	view.Analytics = analyticsHTML()
	_, view.SignedIn = s.sessionUser(r)
	setHTMLHeaders(w, htmlHeaderOpts{Avatars: true})
	if err := picksTmpl.Execute(w, view); err != nil {
		log.Printf("picks template: %v", err)
	}
}

func (s *Server) picksFromPayload(payload map[string]any) picksView {
	view := picksView{PlayerName: "Steam player"}
	if player, _ := payload["player"].(map[string]any); player != nil {
		if name, _ := player["name"].(string); strings.TrimSpace(name) != "" {
			view.PlayerName = name
		}
		if av, _ := player["avatar"].(string); strings.HasPrefix(av, "https://") {
			view.Avatar = av
		}
	}
	games := gamesFromPayload(payload)
	view.Meta = strconv.Itoa(len(games)) + " games in library"

	candidates := make([]libraryGame, 0)
	for _, game := range games {
		if game.Minutes < recommend.OwnedPickMaxMinutes {
			candidates = append(candidates, game)
		}
	}
	view.CandidateN = len(candidates)
	if len(candidates) == 0 {
		view.EmptyHint = "Nothing under 2 hours in this library."
		return view
	}

	names := map[int]string{}
	candIDs := make([]int, 0, len(candidates))
	tasteIDs := make([]int, 0, len(games))
	for _, game := range games {
		if game.Name != "" {
			names[game.AppID] = game.Name
		}
		if game.Hours >= 1 {
			tasteIDs = append(tasteIDs, game.AppID)
		}
	}
	for _, game := range candidates {
		candIDs = append(candIDs, game.AppID)
	}

	tagIDs := append([]int{}, tasteIDs...)
	tagIDs = append(tagIDs, candIDs...)
	tagMap := s.DB.GetTags(tagIDs)
	missing := make([]int, 0)
	for _, id := range candIDs {
		if len(tagMap[id]) == 0 {
			missing = append(missing, id)
		}
	}
	for _, id := range topPlayedIDs(games, 16) {
		if len(tagMap[id]) == 0 {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 && s.Client != nil {
		if fetched, err := s.Client.EnsureTags(s.DB, dedupeIDs(missing), names); err == nil {
			for id, tags := range fetched {
				tagMap[id] = tags
			}
		}
	}

	library := make([]recommend.Game, 0, len(games))
	for _, game := range games {
		library = append(library, recommend.Game{
			AppID:          game.AppID,
			Name:           game.Name,
			Hours:          game.Hours,
			RecentlyPlayed: game.RecentlyPlayed,
			Tags:           recommend.FallbackTags(tagMap[game.AppID], nil),
		})
	}
	candGames := make([]recommend.Game, 0, len(candidates))
	hoursByID := map[int]float64{}
	for _, game := range candidates {
		hoursByID[game.AppID] = game.Hours
		candGames = append(candGames, recommend.Game{
			AppID:          game.AppID,
			Name:           game.Name,
			Hours:          game.Hours,
			RecentlyPlayed: game.RecentlyPlayed,
			Tags:           recommend.FallbackTags(tagMap[game.AppID], nil),
		})
	}

	cards := recommend.RankOwnedFits(library, candGames, maxOwnedPicks)
	if len(cards) == 0 {
		view.EmptyHint = "Need a bit more playtime (1h+) on some games before taste ranking works."
		return view
	}
	for _, card := range cards {
		play := formatHours(hoursByID[card.AppID])
		meta := play
		if clusters := strings.ReplaceAll(strings.Join(card.Clusters, " · "), "-", " "); clusters != "" {
			meta += " · " + clusters
		}
		view.Cards = append(view.Cards, siftGame{
			Name:     card.Name,
			Header:   steam.CachedCoverURL(card.AppID),
			Href:     steam.AppURL(card.AppID),
			SteamURL: card.SteamURL,
			Hours:    play,
			Reason:   card.Reason,
			Meta:     meta,
		})
	}
	return view
}

func dedupeIDs(ids []int) []int {
	seen := map[int]bool{}
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
