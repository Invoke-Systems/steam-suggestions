package web

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"steam-suggestions/data"
	"steam-suggestions/internal/jobs"
	"steam-suggestions/internal/recommend"
	"steam-suggestions/internal/steam"
	"steam-suggestions/internal/store"
	"steam-suggestions/public"
)

const (
	libraryCacheTTL = 10 * time.Minute
	libraryLimit    = 20
	recommendLimit  = 40
	rateWindow      = time.Hour
)

type Server struct {
	Root     string
	DB       *store.DB
	Client   *steam.Client
	Catalog  []recommend.CatalogItem
	Fallback map[int]int

	mu          sync.Mutex
	libCache    map[string]cachedLibrary
	rateBuckets map[string][]time.Time
	genreCache  map[int]map[string]any
	genreDirty  bool

	ImageDir      string
	PublicURL     string
	coverMu       sync.Mutex
	coverInflight map[int]*coverCall
	coverMiss     map[int]time.Time
	coverSem      chan struct{}
	storefront    steam.Featured
	storefrontAt  time.Time
	home          store.HomeRails
	homeAt        time.Time
}

type cachedLibrary struct {
	at      time.Time
	payload map[string]any
}

type libraryGame struct {
	AppID          int      `json:"appid"`
	Name           string   `json:"name"`
	Icon           string   `json:"icon"`
	Header         string   `json:"header"`
	Minutes        int      `json:"minutes"`
	Hours          float64  `json:"hours"`
	Minutes2Weeks  int      `json:"minutes2Weeks"`
	Hours2Weeks    float64  `json:"hours2Weeks"`
	LastPlayed     int64    `json:"lastPlayed"`
	RecentlyPlayed bool     `json:"recentlyPlayed"`
	SteamURL       string   `json:"steamUrl"`
	Tags           []string `json:"tags,omitempty"`
}

func New(root string, db *store.DB, client *steam.Client, catalog []recommend.CatalogItem, fallback map[int]int) *Server {
	if fallback == nil {
		fallback = map[int]int{}
	}
	s := &Server{
		Root:          root,
		DB:            db,
		Client:        client,
		Catalog:       catalog,
		Fallback:      fallback,
		libCache:      map[string]cachedLibrary{},
		rateBuckets:   map[string][]time.Time{},
		genreCache:    map[int]map[string]any{},
		ImageDir:      filepath.Join(root, "data", "images"),
		coverInflight: map[int]*coverCall{},
		coverMiss:     map[int]time.Time{},
		coverSem:      make(chan struct{}, coverFetchers),
	}
	s.loadGenreCache()
	return s
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/config", s.handleConfig)
	mux.HandleFunc("GET /auth/steam", s.handleSteamLogin)
	mux.HandleFunc("GET /auth/steam/callback", s.handleSteamCallback)
	mux.HandleFunc("POST /auth/logout", s.handleLogout)
	mux.HandleFunc("POST /library", s.handleConnectLibrary)
	mux.HandleFunc("GET /sift", s.handleSift)
	mux.HandleFunc("GET /picks", s.handleLibraryPicks)
	mux.HandleFunc("GET /app/{appid}", s.handleGame)
	mux.HandleFunc("GET /search", s.handleSearchPage)
	mux.HandleFunc("GET /api/games", s.handleSearchGamesAPI)
	mux.HandleFunc("GET /leave", s.handleLeave)
	mux.HandleFunc("POST /api/library", s.handleLibrary)
	mux.HandleFunc("GET /api/searches", s.handleListSearches)
	mux.HandleFunc("POST /api/searches", s.handleCreateSearch)
	mux.HandleFunc("DELETE /api/searches/{id}", s.handleDeleteSearch)
	mux.HandleFunc("POST /api/recommend", s.handleRecommend)
	mux.HandleFunc("GET /api/tags", s.handleTags)
	mux.HandleFunc("GET /api/storefront", s.handleStorefront)
	mux.HandleFunc("GET /api/home", s.handleHome)
	mux.HandleFunc("POST /api/genres", s.handleGenres)
	mux.HandleFunc("GET /images/{appid}", s.handleCover)
	mux.HandleFunc("GET /u/{id}", s.handlePlayer)
	mux.HandleFunc("GET /u/{id}/", s.handlePlayer)
	mux.HandleFunc("GET /{$}", s.serveIndex)
	mux.HandleFunc("GET /index.html", s.serveIndex)
	mux.Handle("/", noStore(http.FileServer(public.FileSystem())))
	return withJSONErrors(limitBody(mux, 1<<20))
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	raw, err := fs.ReadFile(public.FS(), "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	html := strings.Replace(string(raw), "<!--analytics-->", string(analyticsHTML()), 1)
	setHTMLHeaders(w, htmlHeaderOpts{Avatars: true})
	_, _ = w.Write([]byte(html))
}

func (s *Server) Warmup() {
	go func() {
		if err := jobs.RefreshTagDict(s.DB, s.Client); err != nil {
			log.Printf("tag dictionary refresh skipped: %v", err)
		}
		ids := make([]int, 0, len(s.Catalog))
		names := map[int]string{}
		for _, item := range s.Catalog {
			ids = append(ids, item.AppID)
			names[item.AppID] = item.Name
		}
		if _, err := s.Client.EnsureTags(s.DB, ids, names); err != nil {
			log.Printf("catalog tags skipped: %v", err)
		}
		_ = s.Client.EnsureReviews(s.DB, ids)
		if !shouldIngestAppList(s.DB) {
			if stats, err := s.DB.Stats(); err == nil {
				log.Printf("Steam SQLite: %d apps, %d tagged, %d priced", stats.Games, stats.Tagged, stats.Priced)
			}
			return
		}
		if err := jobs.IngestAppList(s.DB, s.Client); err != nil {
			log.Printf("Steam catalog ingest skipped: %v", err)
			return
		}
		if stats, err := s.DB.Stats(); err == nil {
			log.Printf("Steam SQLite: %d apps, %d tagged, %d priced", stats.Games, stats.Tagged, stats.Priced)
		}
	}()
}

func shouldIngestAppList(db *store.DB) bool {
	stats, err := db.Stats()
	if err != nil || stats.Games < 1000 {
		return true
	}
	at := db.Meta("applist_at")
	if at == "" {
		_ = db.SetMeta("applist_at", strconv.FormatInt(time.Now().UnixMilli(), 10))
		return false
	}
	ms, err := strconv.ParseInt(at, 10, 64)
	if err != nil {
		return false
	}
	return time.Since(time.UnixMilli(ms)) > 24*time.Hour
}

func (s *Server) envKey() string {
	return strings.TrimSpace(os.Getenv("STEAM_API_KEY"))
}

func (s *Server) acceptClientKey() bool {
	if s.envKey() != "" {
		return false
	}
	flag := strings.ToLower(strings.TrimSpace(os.Getenv("ALLOW_CLIENT_API_KEY")))
	return flag != "0" && flag != "false"
}

func (s *Server) resolveKey(bodyKey string) (string, error) {
	if server := s.envKey(); server != "" {
		return server, nil
	}
	client := strings.TrimSpace(bodyKey)
	if s.acceptClientKey() && client != "" {
		return client, nil
	}
	if s.acceptClientKey() {
		return "", steam.Status(http.StatusBadRequest, "Add STEAM_API_KEY to .env, or paste a Steam Web API key for local use.")
	}
	return "", steam.Status(http.StatusServiceUnavailable, "This instance is not configured with a Steam API key.")
}

func (s *Server) clientFor(key string) *steam.Client {
	return steam.New(key, s.Client.Country, s.Client.Currency)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	stats, err := s.DB.Stats()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "steam": map[string]int{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"steam": map[string]int{
			"games":    stats.Games,
			"tagged":   stats.Tagged,
			"priced":   stats.Priced,
			"tagNames": stats.TagNames,
		},
	})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	payload := map[string]any{
		"hasServerKey":    s.envKey() != "",
		"acceptClientKey": s.acceptClientKey(),
		"recommend":       true,
		"region":          strings.ToUpper(s.Client.Country),
		"currency":        strings.ToUpper(s.Client.Currency),
		"user":            nil,
	}
	if user, ok := s.sessionUser(r); ok {
		payload["user"] = s.publicUser(user)
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleLibrary(w http.ResponseWriter, r *http.Request) {
	if err := s.rateLimit(r, libraryLimit); err != nil {
		panic(err)
	}
	var body struct {
		APIKey     string `json:"apiKey"`
		Identifier string `json:"identifier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		panic(steam.Status(http.StatusBadRequest, "Invalid JSON body."))
	}
	_, signedIn := s.sessionUser(r)
	log.Printf("library request identifier=%q signedIn=%t", strings.TrimSpace(body.Identifier), signedIn)
	key, err := s.resolveKey(body.APIKey)
	if err != nil {
		panic(err)
	}
	client := s.clientFor(key)
	var steamid string
	if strings.TrimSpace(body.Identifier) == "" {
		id, ok := s.activeSteamID(r)
		if !ok {
			panic(steam.Status(http.StatusUnauthorized, "Sign in through Steam, or paste a public profile."))
		}
		steamid = id
	} else {
		parsed, parseErr := steam.ParseSteamInput(body.Identifier)
		if parseErr != nil {
			panic(parseErr)
		}
		resolved, resolveErr := client.ResolveSteamID(parsed)
		if resolveErr != nil {
			panic(mapSteamErr(resolveErr))
		}
		steamid = resolved
		_ = s.setFocus(w, r, steamid)
	}

	payload, err := s.libraryFor(client, steamid)
	if err != nil {
		panic(mapSteamErr(err))
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) libraryFor(client *steam.Client, steamid string) (map[string]any, error) {
	if !store.ValidSteamID(steamid) {
		return nil, steam.Status(http.StatusBadRequest, "Invalid SteamID.")
	}
	s.mu.Lock()
	cached, ok := s.libCache[steamid]
	s.mu.Unlock()
	if ok && time.Since(cached.at) < libraryCacheTTL {
		return cached.payload, nil
	}

	var (
		owned    []steam.OwnedGame
		recent   []steam.OwnedGame
		summary  *steam.PlayerSummary
		ownedErr error
	)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		owned, ownedErr = client.OwnedGames(steamid)
	}()
	go func() {
		defer wg.Done()
		recent, _ = client.RecentlyPlayed(steamid)
	}()
	go func() {
		defer wg.Done()
		summary, _ = client.PlayerSummary(steamid)
	}()
	wishCh := make(chan []int, 1)
	go func() { wishCh <- client.Wishlist(steamid) }()
	wg.Wait()
	if ownedErr != nil {
		return nil, ownedErr
	}
	wishlist := []int{}
	select {
	case wishlist = <-wishCh:
	case <-time.After(800 * time.Millisecond):
	}

	recentIDs := map[int]bool{}
	for _, game := range recent {
		recentIDs[game.AppID] = true
	}
	games := make([]libraryGame, 0, len(owned))
	for _, game := range owned {
		games = append(games, mapOwned(game, recentIDs))
	}
	sort.Slice(games, func(i, j int) bool {
		if games[i].Minutes != games[j].Minutes {
			return games[i].Minutes > games[j].Minutes
		}
		return games[i].Name < games[j].Name
	})
	recentlyPlayed := make([]libraryGame, 0, len(recent))
	for _, game := range recent {
		recentlyPlayed = append(recentlyPlayed, mapOwned(game, recentIDs))
	}

	player := map[string]any{"steamid": steamid, "name": "Steam player", "avatar": "", "profileUrl": ""}
	if summary != nil {
		avatar := summary.AvatarFull
		if avatar == "" {
			avatar = summary.AvatarMedium
		}
		player = map[string]any{
			"steamid":    summary.SteamID,
			"name":       summary.PersonaName,
			"avatar":     avatar,
			"profileUrl": summary.ProfileURL,
		}
	}

	playedIDs := make([]int, 0, len(games))
	for _, game := range games {
		if game.Hours >= 1 {
			playedIDs = append(playedIDs, game.AppID)
		}
	}
	tagMap := s.DB.GetTags(playedIDs)
	played := make([]recommend.Game, 0, len(playedIDs))
	for _, game := range games {
		if game.Hours < 1 {
			continue
		}
		played = append(played, recommend.Game{
			AppID:          game.AppID,
			Name:           game.Name,
			Hours:          game.Hours,
			RecentlyPlayed: game.RecentlyPlayed,
			Tags:           tagMap[game.AppID],
		})
	}
	taste := recommend.BuildTaste(played)
	clusters := taste.RankedClusters
	if len(clusters) > 6 {
		clusters = clusters[:6]
	}
	if clusters == nil {
		clusters = []recommend.TasteCluster{}
	}
	for i, game := range games {
		if tags := tagMap[game.AppID]; len(tags) > 0 {
			games[i].Tags = recommend.PrimaryTags(tags, 12)
		}
	}

	payload := map[string]any{
		"player":         player,
		"games":          games,
		"recentlyPlayed": recentlyPlayed,
		"wishlist":       wishlist,
		"taste":          map[string]any{"clusters": clusters},
	}
	s.putLibCache(steamid, payload)

	if len(played) > 0 {
		go func() {
			limit := 60
			if len(played) < limit {
				limit = len(played)
			}
			ids := make([]int, 0, limit)
			names := map[int]string{}
			for _, game := range played[:limit] {
				ids = append(ids, game.AppID)
				names[game.AppID] = game.Name
			}
			_, _ = s.Client.EnsureTags(s.DB, ids, names)
		}()
	}
	return payload, nil
}

type recInput struct {
	Games          []libraryGame         `json:"games"`
	Popularity     *float64              `json:"popularity"`
	Weirdness      *float64              `json:"weirdness"`
	OnSale         bool                  `json:"onSale"`
	HideAdult      bool                  `json:"hideAdult"`
	HideGore       bool                  `json:"hideGore"`
	SkipShovelware *bool                 `json:"skipShovelware"`
	MinReviews     *int                  `json:"minReviews"`
	MaxReviews     *int                  `json:"maxReviews"`
	MinPositive    *int                  `json:"minPositive"`
	IncludeTags    []string              `json:"includeTags"`
	ExcludeTags    []string              `json:"excludeTags"`
	TagWeights     []recommend.TagWeight `json:"tagWeights"`
	Hidden         []int                 `json:"hidden"`
	Wishlist       []int                 `json:"wishlist"`
}

func (s *Server) handleRecommend(w http.ResponseWriter, r *http.Request) {
	if err := s.rateLimit(r, recommendLimit); err != nil {
		panic(err)
	}
	var body recInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		panic(steam.Status(http.StatusBadRequest, "Invalid JSON body."))
	}
	writeJSON(w, http.StatusOK, s.score(body))
}

func (s *Server) score(body recInput) recommend.Result {
	if len(body.Games) > 8000 {
		panic(steam.Status(http.StatusBadRequest, "Library is too large to score."))
	}

	skipShovel := true
	if body.SkipShovelware != nil {
		skipShovel = *body.SkipShovelware
	}
	minRevDefault, minPosDefault := 0, 0
	if skipShovel {
		minRevDefault, minPosDefault = 500, 70
	}
	minRev := clampInt(intOr(body.MinReviews, minRevDefault), 0, 5_000_000)
	maxRev := clampInt(intOr(body.MaxReviews, 0), 0, 5_000_000)
	minPos := clampInt(intOr(body.MinPositive, minPosDefault), 0, 100)
	includeTags := cleanTagNames(body.IncludeTags)
	excludeTags := cleanTagNames(body.ExcludeTags)
	tagWeights := recommend.NormalizeTagWeights(body.TagWeights)
	includeIDs := s.DB.TagIDsByNames(includeTags)
	excludeIDs := s.DB.TagIDsByNames(excludeTags)
	var preferNames []string
	for _, item := range tagWeights {
		if item.Weight > 0 {
			preferNames = append(preferNames, item.Name)
		}
	}
	preferIDs := s.DB.TagIDsByNames(preferNames)
	needReviews := skipShovel || minRev > 0 || minPos > 0 || maxRev > 0

	slim := make([]recommend.Game, 0, len(body.Games))
	names := map[int]string{}
	playedIDs := make([]int, 0, len(body.Games))
	for _, game := range body.Games {
		slim = append(slim, recommend.Game{
			AppID:          game.AppID,
			Name:           game.Name,
			Hours:          game.Hours,
			RecentlyPlayed: game.RecentlyPlayed,
		})
		if game.Name != "" {
			names[game.AppID] = game.Name
		}
		if game.Hours >= 1 {
			playedIDs = append(playedIDs, game.AppID)
		}
	}

	ensureIDs := topPlayedIDs(body.Games, 16)
	tagMap := s.DB.GetTags(playedIDs)
	var missingTags []int
	for _, id := range ensureIDs {
		if len(tagMap[id]) == 0 {
			missingTags = append(missingTags, id)
		}
	}
	if len(missingTags) > 0 {
		fetched, err := s.Client.EnsureTags(s.DB, missingTags, names)
		if err == nil {
			for id, tags := range fetched {
				tagMap[id] = tags
			}
		}
	}
	taggedGames := make([]recommend.Game, 0, len(slim))
	for _, game := range slim {
		game.Tags = recommend.FallbackTags(tagMap[game.AppID], nil)
		taggedGames = append(taggedGames, game)
	}

	tasteIDs := recommend.DistinctiveTagIDs(taggedGames, 10)
	tasteIDs = append(tasteIDs, preferIDs...)
	tasteIDs = append(tasteIDs, includeIDs...)
	if len(tasteIDs) == 0 {
		panic(steam.Status(http.StatusBadRequest, "Add a prefer or include tag, or load a public library."))
	}
	candOpts := store.CandidateOpts{
		OnSale:     body.OnSale,
		PerTag:     700,
		MaxTotal:   4000,
		IncludeIDs: includeIDs,
		ExcludeIDs: excludeIDs,
	}
	cands := s.DB.RecommendCandidates(tasteIDs, candOpts)
	if needReviews {
		tight := candOpts
		tight.MinReviews = minRev
		tight.MaxReviews = maxRev
		tight.MinPositive = minPos
		tight.RequireReview = true
		filtered := s.DB.RecommendCandidates(tasteIDs, tight)
		if len(filtered) < 24 {
			ids := make([]int, 0, len(cands))
			for _, c := range cands {
				ids = append(ids, c.AppID)
			}
			s.Client.FillReviews(s.DB, ids, 40)
			filtered = s.DB.RecommendCandidates(tasteIDs, tight)
		}
		if len(filtered) > 0 {
			cands = filtered
		}
	}
	candIDs := make([]int, 0, len(cands))
	for _, c := range cands {
		candIDs = append(candIDs, c.AppID)
		if c.Name != "" {
			names[c.AppID] = c.Name
		}
	}
	candTags := s.DB.GetTags(candIDs)
	reviews := s.DB.GetReviews(candIDs)
	prices := map[int]store.PriceView{}
	if body.OnSale {
		prices = s.DB.GetPrices(candIDs)
	}

	scored := make([]recommend.Game, 0, len(cands))
	for _, c := range cands {
		tags := candTags[c.AppID]
		rev, hasRev := reviews[c.AppID]
		pop := steam.ReviewsToPopularity(recommend.MaxTagWeight(tags))
		reviewCount := 0
		positive := 0
		if hasRev {
			reviewCount = rev.Total
			positive = rev.Positive
			pop = rev.Popularity
		} else if fallback := s.Fallback[c.AppID]; fallback > 0 {
			pop = fallback
		}
		scored = append(scored, recommend.Game{
			AppID:      c.AppID,
			Name:       c.Name,
			Tags:       tags,
			Popularity: pop,
			Reviews:    reviewCount,
			Positive:   positive,
			HasReviews: hasRev,
			Header:     steam.CachedCoverURL(c.AppID),
		})
	}
	if len(scored) < 24 && len(slim) > 0 {
		for _, item := range s.Catalog {
			tags := s.DB.GetTags([]int{item.AppID})[item.AppID]
			rev := s.DB.GetReviews([]int{item.AppID})[item.AppID]
			game := recommend.CatalogGame(item, tags, rev.Total, s.Fallback[item.AppID], steam.CachedCoverURL(item.AppID))
			game.Positive = rev.Positive
			game.HasReviews = rev.AppID != 0
			scored = append(scored, game)
		}
	}

	pop := 0.5
	if body.Popularity != nil {
		pop = recommend.Clamp01(*body.Popularity)
	}
	weird := 0.25
	if body.Weirdness != nil {
		weird = recommend.Clamp01(*body.Weirdness)
	}
	result := recommend.Recommend(taggedGames, scored, recommend.Options{
		Popularity:     pop,
		Weirdness:      weird,
		OnSale:         body.OnSale,
		Prices:         prices,
		Hidden:         idSet(body.Hidden),
		Wishlist:       idSet(body.Wishlist),
		HideAdult:      body.HideAdult,
		HideGore:       body.HideGore,
		SkipUnknown:    needReviews,
		SkipShovelware: skipShovel,
		MinReviews:     minRev,
		MaxReviews:     maxRev,
		MinPositive:    minPos,
		IncludeTags:    includeTags,
		ExcludeTags:    excludeTags,
		TagWeights:     tagWeights,
	})

	shown := shownCards(result)
	shownIDs := make([]int, 0, len(shown))
	for _, item := range shown {
		shownIDs = append(shownIDs, item.AppID)
		if item.Name != "" {
			names[item.AppID] = item.Name
		}
	}
	if len(shownIDs) > 0 {
		have := s.DB.GetPrices(shownIDs)
		recommend.StampPrices(&result, have)
		var missing []int
		for _, id := range shownIDs {
			if _, ok := have[id]; !ok {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 {
			recommend.StampPrices(&result, s.Client.EnsurePrices(s.DB, missing, names))
		}
		recommend.StampSparklines(&result, s.DB.PriceSparklines(shownIDs, 30))
		// Refresh titles from Steam so covers (keyed by appid) match the name we show.
		if items, err := s.Client.GetItems(shownIDs); err == nil {
			fresh := map[int]string{}
			for _, item := range items {
				if item.AppID > 0 && item.Name != "" {
					fresh[item.AppID] = item.Name
					_ = s.DB.UpsertGame(item.AppID, item.Name)
				}
			}
			recommend.StampNames(&result, fresh)
		}
	}
	localizeCovers(&result)
	return result
}

func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	tags := s.DB.SearchTags(q, 12)
	if tags == nil {
		tags = []store.DictTag{}
	}
	writeJSON(w, http.StatusOK, tags)
}

func intOr(p *int, fallback int) int {
	if p == nil {
		return fallback
	}
	return *p
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func cleanTagNames(names []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
		if len(out) >= 12 {
			break
		}
	}
	return out
}

func shownCards(result recommend.Result) []recommend.Card {
	out := make([]recommend.Card, 0, len(result.Groups.MoreLike)+len(result.Groups.Adjacent)+len(result.Groups.Wildcard))
	out = append(out, result.Groups.MoreLike...)
	out = append(out, result.Groups.Adjacent...)
	out = append(out, result.Groups.Wildcard...)
	return out
}

func idSet(ids []int) map[int]bool {
	out := map[int]bool{}
	for _, id := range ids {
		if id != 0 {
			out[id] = true
		}
	}
	return out
}

func topPlayedIDs(games []libraryGame, limit int) []int {
	type row struct {
		id    int
		hours float64
	}
	var list []row
	seen := map[int]bool{}
	for _, game := range games {
		if game.AppID == 0 || game.Hours < 1 || seen[game.AppID] {
			continue
		}
		seen[game.AppID] = true
		list = append(list, row{game.AppID, game.Hours})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].hours > list[j].hours })
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	out := make([]int, len(list))
	for i, r := range list {
		out[i] = r.id
	}
	return out
}

func (s *Server) handleGenres(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AppIDs []int `json:"appids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		panic(steam.Status(http.StatusBadRequest, "Invalid JSON body."))
	}
	seen := map[int]bool{}
	details := map[int]map[string]any{}
	var missing []int
	s.mu.Lock()
	for _, id := range body.AppIDs {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		if cached, ok := s.genreCache[id]; ok {
			details[id] = cached
			continue
		}
		missing = append(missing, id)
	}
	s.mu.Unlock()

	type job struct{ id int }
	jobsCh := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	workers := 3
	if len(missing) < workers {
		workers = len(missing)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobsCh {
				time.Sleep(120 * time.Millisecond)
				info := s.fetchStoreDetails(id)
				mu.Lock()
				details[id] = info
				mu.Unlock()
			}
		}()
	}
	for _, id := range missing {
		jobsCh <- id
	}
	close(jobsCh)
	wg.Wait()

	if len(missing) > 0 {
		s.mu.Lock()
		for _, id := range missing {
			if info, ok := details[id]; ok {
				s.genreCache[id] = info
			}
		}
		s.genreDirty = true
		s.mu.Unlock()
		go s.saveGenreCache()
	}
	writeJSON(w, http.StatusOK, map[string]any{"details": details})
}

func (s *Server) fetchStoreDetails(appid int) map[string]any {
	for attempt := 0; attempt < 4; attempt++ {
		info, err := s.Client.StoreDetails(appid)
		if err != nil {
			var rate *steam.RateError
			if errors.As(err, &rate) && attempt < 3 {
				time.Sleep(time.Duration(1200*(attempt+1)) * time.Millisecond)
				continue
			}
			return map[string]any{"appid": appid, "missing": true, "genres": []string{}}
		}
		return info
	}
	return map[string]any{"appid": appid, "missing": true, "genres": []string{}}
}

func (s *Server) rateLimit(r *http.Request, max int) error {
	ip := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ip = strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	key := r.URL.Path + ":" + ip
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	recent := s.rateBuckets[key]
	kept := recent[:0]
	for _, t := range recent {
		if now.Sub(t) < rateWindow {
			kept = append(kept, t)
		}
	}
	if len(kept) >= max {
		return steam.Status(http.StatusTooManyRequests, "Too many requests. Try again in a few minutes.")
	}
	s.rateBuckets[key] = append(kept, now)
	return nil
}

func (s *Server) loadGenreCache() {
	raw, err := os.ReadFile(filepath.Join(s.Root, "data", "genre-cache.json"))
	if err != nil {
		return
	}
	var parsed map[string]map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return
	}
	for key, value := range parsed {
		id, err := strconv.Atoi(key)
		if err != nil {
			continue
		}
		s.genreCache[id] = value
	}
}

func (s *Server) saveGenreCache() {
	s.mu.Lock()
	if !s.genreDirty {
		s.mu.Unlock()
		return
	}
	out := make(map[string]map[string]any, len(s.genreCache))
	for id, value := range s.genreCache {
		out[strconv.Itoa(id)] = value
	}
	s.genreDirty = false
	s.mu.Unlock()
	path := filepath.Join(s.Root, "data", "genre-cache.json")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	raw, err := json.Marshal(out)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, raw, 0o644)
}

func mapOwned(game steam.OwnedGame, recentIDs map[int]bool) libraryGame {
	name := game.Name
	if name == "" {
		name = "App " + strconv.Itoa(game.AppID)
	}
	return libraryGame{
		AppID:          game.AppID,
		Name:           name,
		Icon:           steam.IconURL(game.AppID, game.ImgIconURL),
		Header:         steam.CachedCoverURL(game.AppID),
		Minutes:        game.PlaytimeMinutes(),
		Hours:          steam.MinutesToHours(game.PlaytimeMinutes()),
		Minutes2Weeks:  game.Playtime2Weeks,
		Hours2Weeks:    steam.MinutesToHours(game.Playtime2Weeks),
		LastPlayed:     game.RtimeLastPlayed,
		RecentlyPlayed: recentIDs[game.AppID] || game.PlayedRecently(time.Now().Unix()),
		SteamURL:       steam.StoreURL(game.AppID),
	}
}

func mapSteamErr(err error) error {
	var status *steam.StatusError
	if errors.As(err, &status) {
		return status
	}
	var httpErr *steam.HTTPError
	if errors.As(err, &httpErr) {
		code := http.StatusBadGateway
		if httpErr.Status == http.StatusForbidden {
			code = http.StatusForbidden
		}
		return steam.Status(code, err.Error())
	}
	var rate *steam.RateError
	if errors.As(err, &rate) {
		return steam.Status(http.StatusBadGateway, err.Error())
	}
	return steam.Status(http.StatusBadGateway, err.Error())
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func limitBody(next http.Handler, n int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, n)
		next.ServeHTTP(w, r)
	})
}

func withJSONErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			err, ok := rec.(error)
			if !ok {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Unexpected server error."})
				return
			}
			var status *steam.StatusError
			if errors.As(err, &status) {
				writeJSON(w, status.Code, map[string]string{"error": status.Msg})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}()
		next.ServeHTTP(w, r)
	})
}

func LoadCatalog(root string) ([]recommend.CatalogItem, map[int]int, error) {
	raw, err := readDataFile(root, "catalog.json")
	if err != nil {
		return nil, nil, err
	}
	var catalog []recommend.CatalogItem
	if err := json.Unmarshal(raw, &catalog); err != nil {
		return nil, nil, err
	}
	fallback := map[int]int{}
	popRaw, err := readDataFile(root, "popularity.json")
	if err == nil {
		var parsed map[string]int
		if json.Unmarshal(popRaw, &parsed) == nil {
			for key, value := range parsed {
				id, convErr := strconv.Atoi(key)
				if convErr != nil {
					continue
				}
				fallback[id] = value
			}
		}
	}
	return catalog, fallback, nil
}

func readDataFile(root, name string) ([]byte, error) {
	if root != "" {
		raw, err := os.ReadFile(filepath.Join(root, "data", name))
		if err == nil {
			return raw, nil
		}
	}
	return data.Files.ReadFile(name)
}

func FindRoot() string {
	var candidates []string
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd)
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(exe))
	}
	for _, dir := range candidates {
		if _, err := os.Stat(filepath.Join(dir, "data", "catalog.json")); err == nil {
			return dir
		}
	}
	return "."
}
