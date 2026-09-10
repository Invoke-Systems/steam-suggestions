package web

import (
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"steam-suggestions/internal/steam"
	"steam-suggestions/internal/store"
)

var playerTmpl = template.Must(template.ParseFS(tmplFS, "templates/player.html"))

type playerView struct {
	SteamID    string
	ShareURL   string
	PlayerName string
	Avatar     string
	SteamURL   string
	Meta       string
	GameCount  int
	Clusters   []siftCluster
	Games      []siftGame
	MoreGames  []siftGame
	SignedIn   bool
	Error      string
	Analytics  template.HTML
}

func (s *Server) handlePlayer(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.PathValue("id"))
	raw = strings.Trim(raw, "/")
	if raw == "" || len(raw) > maxIdentifier {
		http.NotFound(w, r)
		return
	}

	key, err := s.resolveKey("")
	if err != nil {
		http.Redirect(w, r, "/?err=key", http.StatusSeeOther)
		return
	}
	client := s.clientFor(key)

	parsed, err := steam.ParseSteamInput(raw)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	steamid, err := client.ResolveSteamID(parsed)
	if err != nil || !store.ValidSteamID(steamid) {
		http.NotFound(w, r)
		return
	}
	// Canonical share URL is always steamid64.
	if raw != steamid {
		http.Redirect(w, r, steam.PlayerURL(steamid), http.StatusFound)
		return
	}

	view := playerView{
		SteamID:    steamid,
		ShareURL:   steam.PlayerURL(steamid),
		PlayerName: "Steam player",
		SteamURL:   "https://steamcommunity.com/profiles/" + steamid + "/",
	}
	_, view.SignedIn = s.sessionUser(r)

	payload, err := s.libraryFor(client, steamid)
	if err != nil {
		view.Error = playerErrorMessage(err)
		s.writePlayer(w, view)
		return
	}
	view = s.playerFromPayload(steamid, payload)
	_, view.SignedIn = s.sessionUser(r)
	s.writePlayer(w, view)
}

func (s *Server) writePlayer(w http.ResponseWriter, view playerView) {
	view.Analytics = analyticsHTML()
	setHTMLHeaders(w, htmlHeaderOpts{Avatars: true})
	if err := playerTmpl.Execute(w, view); err != nil {
		log.Printf("player template: %v", err)
	}
}

func (s *Server) playerFromPayload(steamid string, payload map[string]any) playerView {
	view := playerView{
		SteamID:    steamid,
		ShareURL:   steam.PlayerURL(steamid),
		PlayerName: "Steam player",
		SteamURL:   "https://steamcommunity.com/profiles/" + steamid + "/",
	}
	if player, _ := payload["player"].(map[string]any); player != nil {
		if name, _ := player["name"].(string); strings.TrimSpace(name) != "" {
			view.PlayerName = name
		}
		if av, _ := player["avatar"].(string); strings.HasPrefix(av, "https://") {
			view.Avatar = av
		}
		if profile, _ := player["profileUrl"].(string); strings.HasPrefix(profile, "https://") {
			view.SteamURL = profile
		}
	}
	games := gamesFromPayload(payload)
	view.GameCount = len(games)
	hours := 0.0
	for _, game := range games {
		hours += game.Hours
	}
	capped := games
	if len(capped) > maxSiftGames {
		capped = capped[:maxSiftGames]
	}
	top := capped
	if len(top) > maxSiftTop {
		top = top[:maxSiftTop]
		view.MoreGames = toSiftGames(capped[maxSiftTop:])
	}
	view.Games = toSiftGames(top)
	view.Meta = strconv.Itoa(view.GameCount) + " games · " + strconv.Itoa(int(hours+0.5)) + " hours"
	if clusters := clustersFromPayload(payload); len(clusters) > 0 {
		maxPct := 1
		for _, c := range clusters {
			if c.Percent > maxPct {
				maxPct = c.Percent
			}
		}
		for _, c := range clusters {
			width := 0
			if maxPct > 0 {
				width = c.Percent * 100 / maxPct
			}
			view.Clusters = append(view.Clusters, siftCluster{
				Name:  strings.ReplaceAll(c.Name, "-", " "),
				Key:   tagKey(c.Name),
				Width: width,
			})
		}
	}
	return view
}

func playerErrorMessage(err error) string {
	switch lookupCode(err) {
	case "private":
		return "This Steam library is private. Ask them to set Game details to Public."
	case "notfound":
		return "Couldn’t find that Steam profile."
	case "ratelimit":
		return "Steam is rate-limiting us. Try again in a minute."
	default:
		return "Couldn’t load this library right now."
	}
}
