package web

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"steam-suggestions/internal/recommend"
	"steam-suggestions/internal/steam"
	"steam-suggestions/internal/store"
)

//go:embed templates/*.html
var tmplFS embed.FS

const (
	focusCookie   = "playsift_focus"
	maxIdentifier = 200
	maxSiftTop    = 12
	maxSiftGames  = 80
	maxLibCache   = 256
)

var siftTmpl = template.Must(template.ParseFS(tmplFS, "templates/sift.html"))

type siftView struct {
	SignedIn   bool
	SteamID    string
	ShareURL   string
	PlayerName string
	Avatar     string
	Meta       string
	GameCount  int
	Clusters   []siftCluster
	Games      []siftGame
	MoreGames  []siftGame
	Recs       []siftRecGroup
	Analytics  template.HTML
}

type siftCluster struct {
	Name  string
	Key   string
	Width int
}

type siftGame struct {
	Name     string
	Header   string
	Href     string
	SteamURL string
	Hours    string
	Reason   string
	Meta     string
	Tags     string
}

type siftRecGroup struct {
	Title string
	Cards []siftGame
}

func (s *Server) handleConnectLibrary(w http.ResponseWriter, r *http.Request) {
	if err := s.rateLimit(r, libraryLimit); err != nil {
		http.Redirect(w, r, "/?err=ratelimit", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?err=lookup", http.StatusSeeOther)
		return
	}
	identifier := strings.TrimSpace(r.FormValue("identifier"))
	if len(identifier) > maxIdentifier {
		identifier = identifier[:maxIdentifier]
	}
	if identifier == "" {
		http.Redirect(w, r, "/?err=missing", http.StatusSeeOther)
		return
	}
	key, err := s.resolveKey("")
	if err != nil {
		http.Redirect(w, r, "/?err=key", http.StatusSeeOther)
		return
	}
	client := s.clientFor(key)
	parsed, err := steam.ParseSteamInput(identifier)
	if err != nil {
		http.Redirect(w, r, "/?err=notfound", http.StatusSeeOther)
		return
	}
	steamid, err := client.ResolveSteamID(parsed)
	if err != nil || !store.ValidSteamID(steamid) {
		http.Redirect(w, r, "/?err="+lookupCode(err), http.StatusSeeOther)
		return
	}
	if _, err := s.libraryFor(client, steamid); err != nil {
		http.Redirect(w, r, "/?err="+lookupCode(err), http.StatusSeeOther)
		return
	}
	if err := s.setFocus(w, r, steamid); err != nil {
		log.Printf("library focus: %v", err)
		http.Redirect(w, r, "/?err=lookup", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/sift", http.StatusSeeOther)
}

func (s *Server) handleSift(w http.ResponseWriter, r *http.Request) {
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
	view := s.siftFromPayload(payload)
	view.SteamID = steamid
	view.ShareURL = steam.PlayerURL(steamid)
	view.Analytics = analyticsHTML()
	_, view.SignedIn = s.sessionUser(r)
	setHTMLHeaders(w, htmlHeaderOpts{Avatars: true})
	if err := siftTmpl.Execute(w, view); err != nil {
		log.Printf("sift template: %v", err)
	}
}

func (s *Server) handleLeave(w http.ResponseWriter, r *http.Request) {
	s.clearFocusCookie(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) setFocus(w http.ResponseWriter, r *http.Request, steamid string) error {
	token, err := s.DB.CreateLibraryFocus(steamid)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     focusCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(24 * 60 * 60),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cookieSecure(r),
	})
	return nil
}

func (s *Server) clearFocusCookie(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(focusCookie); err == nil {
		s.DB.DeleteLibraryFocus(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     focusCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cookieSecure(r),
	})
}

func (s *Server) focusSteamID(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(focusCookie)
	if err != nil || cookie.Value == "" {
		return "", false
	}
	return s.DB.LibraryFocusSteamID(cookie.Value)
}

func (s *Server) activeSteamID(r *http.Request) (string, bool) {
	if id, ok := s.focusSteamID(r); ok {
		return id, true
	}
	if user, ok := s.sessionUser(r); ok && store.ValidSteamID(user.SteamID) {
		return user.SteamID, true
	}
	return "", false
}

func lookupCode(err error) string {
	if err == nil {
		return "lookup"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "public"):
		return "private"
	case strings.Contains(msg, "too many"):
		return "ratelimit"
	case strings.Contains(msg, "vanity"), strings.Contains(msg, "resolve"), strings.Contains(msg, "not found"):
		return "notfound"
	default:
		return "lookup"
	}
}

func (s *Server) siftFromPayload(payload map[string]any) siftView {
	view := siftView{PlayerName: "Steam player"}
	if player, _ := payload["player"].(map[string]any); player != nil {
		if name, _ := player["name"].(string); strings.TrimSpace(name) != "" {
			view.PlayerName = name
		}
		if av, _ := player["avatar"].(string); strings.HasPrefix(av, "https://") {
			view.Avatar = av
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
	view.Recs = s.siftRecs(games, intsFromPayload(payload["wishlist"]))
	return view
}

func toSiftGames(games []libraryGame) []siftGame {
	out := make([]siftGame, 0, len(games))
	for _, game := range games {
		out = append(out, siftGame{
			Name:     game.Name,
			Header:   steam.HeaderURL(game.AppID),
			Href:     steam.AppURL(game.AppID),
			SteamURL: game.SteamURL,
			Hours:    formatHours(game.Hours),
			Tags:     tagAttr(game.Tags),
		})
	}
	return out
}

func tagKey(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(name, "-", " ")), " "))
}

func tagAttr(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	parts := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, tag := range tags {
		key := tagKey(tag)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		parts = append(parts, key)
	}
	if len(parts) == 0 {
		return ""
	}
	return "|" + strings.Join(parts, "|") + "|"
}

func (s *Server) siftRecs(games []libraryGame, wishlist []int) []siftRecGroup {
	defer func() { _ = recover() }()
	skip := true
	result := s.score(recInput{
		Games:          games,
		Wishlist:       wishlist,
		SkipShovelware: &skip,
	})
	groups := []struct {
		title string
		cards []recommend.Card
	}{
		{"More like your library", result.Groups.MoreLike},
		{"Nearby experiments", result.Groups.Adjacent},
		{"Wildcards", result.Groups.Wildcard},
	}
	out := make([]siftRecGroup, 0, 3)
	for _, g := range groups {
		if len(g.cards) == 0 {
			continue
		}
		item := siftRecGroup{Title: g.title}
		ids := make([]int, 0, len(g.cards))
		for _, card := range g.cards {
			ids = append(ids, card.AppID)
		}
		players := s.DB.GetPlayerStats(ids)
		for _, card := range g.cards {
			meta := strings.ReplaceAll(strings.Join(card.Clusters, " · "), "-", " ")
			if meta == "" {
				meta = "fit"
			}
			if st, ok := players[card.AppID]; ok {
				if extra := formatPlayers(st); extra != "" {
					meta += " · " + extra
				}
			}
			if extra := formatPrice(card); extra != "" {
				meta += " · " + extra
			}
			item.Cards = append(item.Cards, siftGame{
				Name:     card.Name,
				Header:   steam.HeaderURL(card.AppID),
				Href:     steam.AppURL(card.AppID),
				SteamURL: card.SteamURL,
				Reason:   card.Reason,
				Meta:     meta,
			})
		}
		out = append(out, item)
	}
	return out
}

func gamesFromPayload(payload map[string]any) []libraryGame {
	switch games := payload["games"].(type) {
	case []libraryGame:
		return games
	default:
		return nil
	}
}

func clustersFromPayload(payload map[string]any) []recommend.TasteCluster {
	taste, _ := payload["taste"].(map[string]any)
	if taste == nil {
		return nil
	}
	raw, _ := taste["clusters"].([]recommend.TasteCluster)
	return raw
}

func intsFromPayload(raw any) []int {
	v, _ := raw.([]int)
	return v
}

func formatHours(hours float64) string {
	if hours <= 0 {
		return "Unplayed"
	}
	if hours < 1 {
		return strconv.Itoa(int(hours*60+0.5)) + "m"
	}
	return strconv.FormatFloat(hours, 'f', 0, 64) + "h"
}

func formatPrice(card recommend.Card) string {
	price := strings.TrimSpace(card.Price)
	if price == "" || strings.EqualFold(price, "Free or unlisted") {
		return ""
	}
	if card.AtLow {
		return price + " · Steam low"
	}
	return price
}

func formatPlayers(st store.PlayerStats) string {
	if st.Current <= 0 && st.PeakAll <= 0 {
		return ""
	}
	parts := make([]string, 0, 2)
	if st.Current > 0 {
		parts = append(parts, commaInt(st.Current)+" in-game")
	}
	if st.PeakAll > 0 && st.PeakAll != st.Current {
		parts = append(parts, "peak "+commaInt(st.PeakAll))
	}
	return strings.Join(parts, " · ")
}

func commaInt(n int) string {
	s := strconv.Itoa(n)
	if n < 0 {
		s = strconv.Itoa(-n)
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre == 0 {
		pre = 3
	}
	b.WriteString(s[:pre])
	for i := pre; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	if n < 0 {
		return "-" + b.String()
	}
	return b.String()
}

func (s *Server) putLibCache(steamid string, payload map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.libCache) >= maxLibCache {
		oldest := ""
		oldestAt := time.Now()
		first := true
		for id, entry := range s.libCache {
			if first || entry.at.Before(oldestAt) {
				oldest, oldestAt, first = id, entry.at, false
			}
		}
		delete(s.libCache, oldest)
	}
	s.libCache[steamid] = cachedLibrary{at: time.Now(), payload: payload}
}
