package recommend

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"steam-suggestions/internal/store"
)

const (
	tasteHourFloor = 1
	bounceHourCap  = 3
)

var genericTags = map[string]bool{
	"Singleplayer": true, "Multiplayer": true, "Co-op": true, "Online Co-Op": true,
	"Local Co-Op": true, "Co-op Campaign": true, "PvP": true, "Online PvP": true,
	"Shared/Split Screen": true, "Steam Achievements": true, "Steam Cloud": true,
	"Steam Trading Cards": true, "Steam Workshop": true, "Controller": true,
	"Full controller support": true, "Partial Controller Support": true,
	"Family Sharing": true, "Remote Play Together": true, "Remote Play on TV": true,
	"Indie": true, "Action": true, "Adventure": true, "Casual": true,
	"Early Access": true, "Free to Play": true, "Great Soundtrack": true,
	"Replay Value": true, "Atmospheric": true, "2D": true, "3D": true,
	"Pixel Graphics": true, "Cute": true, "Funny": true, "Comedy": true, "Colorful": true,
	"Sci-fi": true, "Futuristic": true, "Story Rich": true, "First-Person": true,
	"Third Person": true, "Third-Person Shooter": true, "Fast-Paced": true,
	"Physics": true, "Physics Based": true, "Sandbox": true, "Building": true,
	"Open World": true, "Fantasy": true, "Dark Fantasy": true, "Horror": true,
	"Difficult": true, "Exploration": true, "Combat": true, "Character Customization": true,
	"Retro": true, "Arcade": true,
}

var broadTags = map[string]bool{
	"RPG": true, "Strategy": true, "Simulation": true, "Sports": true,
	"Racing": true, "Shooter": true, "FPS": true,
}

var junkName = regexp.MustCompile(`(?i)\b(demo|playtest|legacy|goty)\b`)
var colonName = regexp.MustCompile(`[_:]+`)
var spaceName = regexp.MustCompile(`\s+`)
var dashName = regexp.MustCompile(`[\p{Pd}]+`)
var nonName = regexp.MustCompile(`[^\p{L}\p{N}]+`)

type Tag = store.NamedTag

type Game struct {
	AppID          int
	Name           string
	Hours          float64
	RecentlyPlayed bool
	Tags           []Tag
	Related        []string
	Popularity     int
	Reviews        int
	Positive       int
	HasReviews     bool
	Header         string
}

type CatalogItem struct {
	AppID      int      `json:"appid"`
	Name       string   `json:"name"`
	Clusters   []string `json:"clusters"`
	Tags       []string `json:"tags"`
	Related    []string `json:"related"`
	Popularity int      `json:"popularity"`
	Reviews    int      `json:"reviews"`
	Header     string   `json:"header"`
}

type Card struct {
	AppID      int      `json:"appid"`
	Name       string   `json:"name"`
	Clusters   []string `json:"clusters"`
	Tags       []string `json:"tags"`
	Popularity int      `json:"popularity"`
	Reviews    int      `json:"reviews"`
	Positive   int      `json:"positive,omitempty"`
	SteamURL   string   `json:"steamUrl"`
	PageURL    string   `json:"pageUrl"`
	Header     string   `json:"header"`
	Score      int      `json:"score"`
	Fit        float64  `json:"fit"`
	Reason     string   `json:"reason"`
	Because    string   `json:"because,omitempty"`
	Price      string   `json:"price,omitempty"`
	Discount   int      `json:"discount,omitempty"`
	OnSale     bool     `json:"onSale,omitempty"`
	LowPrice   *string  `json:"lowPrice,omitempty"`
	AtLow      bool     `json:"atLow,omitempty"`
	OnWishlist bool     `json:"onWishlist,omitempty"`
	Sparkline  []int    `json:"sparkline,omitempty"`

	familiarRaw     float64
	similarity      float64
	tagPref         float64
	distinctiveHits []string
}

type TasteCluster struct {
	Name    string  `json:"name"`
	Hours   float64 `json:"hours"`
	Percent int     `json:"percent"`
}

type PlayedSummary struct {
	Name  string  `json:"name"`
	Hours float64 `json:"hours"`
}

type Result struct {
	Taste struct {
		Heaviest  *string         `json:"heaviest"`
		Clusters  []TasteCluster  `json:"clusters"`
		Recent    []string        `json:"recent"`
		TopPlayed []PlayedSummary `json:"topPlayed"`
	} `json:"taste"`
	Dials struct {
		Popularity     float64     `json:"popularity"`
		Weirdness      float64     `json:"weirdness"`
		OnSale         bool        `json:"onSale"`
		SkipShovelware bool        `json:"skipShovelware"`
		MinReviews     int         `json:"minReviews"`
		MaxReviews     int         `json:"maxReviews"`
		MinPositive    int         `json:"minPositive"`
		IncludeTags    []string    `json:"includeTags"`
		ExcludeTags    []string    `json:"excludeTags"`
		TagWeights     []TagWeight `json:"tagWeights"`
	} `json:"dials"`
	Pool   int `json:"pool"`
	Groups struct {
		MoreLike []Card `json:"more_like"`
		Adjacent []Card `json:"adjacent"`
		Wildcard []Card `json:"wildcard"`
	} `json:"groups"`
}

type Options struct {
	Popularity     float64
	Weirdness      float64
	OnSale         bool
	Prices         map[int]store.PriceView
	ExtraCatalog   []Game
	Hidden         map[int]bool
	Wishlist       map[int]bool
	HideAdult      bool
	HideGore       bool
	SkipUnknown    bool
	SkipShovelware bool
	MinReviews     int
	MaxReviews     int
	MinPositive    int
	IncludeTags    []string
	ExcludeTags    []string
	TagWeights     []TagWeight
}

type TagWeight struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

type Taste struct {
	OwnedIDs       map[int]bool
	OwnedNames     map[string]bool
	OwnedKeys      map[string]bool
	TagHours       map[string]float64
	Vec            map[string]float64
	RankedClusters []TasteCluster
	TopPlayed      []playedGame
	Recent         []playedGame
	Bounced        []playedGame
}

type playedGame struct {
	appid          int
	name           string
	hours          float64
	recentlyPlayed bool
	tags           []Tag
	clusters       []string
	vec            map[string]float64
}

type comparable struct {
	game    playedGame
	value   float64
	nameHit bool
	shared  []string
}

func TagSpecificity(name string) float64 {
	if genericTags[name] {
		return 0.1
	}
	if broadTags[name] {
		return 0.32
	}
	return 1
}

func NormalizeName(name string) string {
	s := strings.ToLower(name)
	s = strings.Map(func(r rune) rune {
		if r == '™' || r == '®' || r == '©' {
			return -1
		}
		return r
	}, s)
	s = colonName.ReplaceAllString(s, " ")
	s = junkName.ReplaceAllString(s, "")
	s = spaceName.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// NameKey is a punctuation-insensitive title used to skip owned games that
// Steam lists under a duplicate appid or slightly different spelling.
func NameKey(name string) string {
	s := NormalizeName(name)
	s = dashName.ReplaceAllString(s, " ")
	s = nonName.ReplaceAllString(s, " ")
	s = spaceName.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func PrimaryTags(tags []Tag, limit int) []string {
	if limit <= 0 {
		limit = 3
	}
	type pair struct {
		name   string
		weight int
	}
	var list []pair
	for _, tag := range tags {
		if TagSpecificity(tag.Name) < 0.5 {
			continue
		}
		list = append(list, pair{tag.Name, tag.Weight})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].weight > list[j].weight })
	if len(list) > limit {
		list = list[:limit]
	}
	out := make([]string, len(list))
	for i, p := range list {
		out[i] = p.name
	}
	return out
}

func FallbackTags(steam []Tag, catalog []string) []Tag {
	if len(steam) > 0 {
		return steam
	}
	out := make([]Tag, 0, len(catalog))
	for i, name := range catalog {
		w := 20 - i
		if w < 1 {
			w = 1
		}
		out = append(out, Tag{Name: name, Weight: w})
	}
	return out
}

func CatalogGame(item CatalogItem, steamTags []Tag, reviews, popularity int, header string) Game {
	tags := FallbackTags(steamTags, item.Tags)
	if header == "" {
		header = item.Header
	}
	if header == "" {
		header = "https://cdn.akamai.steamstatic.com/steam/apps/" + strconv.Itoa(item.AppID) + "/header.jpg"
	}
	pop := popularity
	if pop == 0 {
		pop = item.Popularity
	}
	if pop == 0 {
		pop = 55
	}
	return Game{
		AppID:      item.AppID,
		Name:       item.Name,
		Tags:       tags,
		Related:    item.Related,
		Popularity: pop,
		Reviews:    reviews,
		Positive:   0,
		HasReviews: reviews > 0,
		Header:     header,
	}
}

func tagVector(tags []Tag) map[string]float64 {
	vec := map[string]float64{}
	maxW := 0
	for _, tag := range tags {
		if tag.Weight > maxW {
			maxW = tag.Weight
		}
	}
	if maxW <= 0 {
		return vec
	}
	for _, tag := range tags {
		scale := TagSpecificity(tag.Name)
		if scale <= 0 {
			continue
		}
		value := (float64(tag.Weight) / float64(maxW)) * scale
		if value > 0 {
			vec[tag.Name] += value
		}
	}
	return vec
}

func addVector(target, vec map[string]float64, scale float64) {
	for k, v := range vec {
		target[k] += v * scale
	}
}

func dot(a, b map[string]float64) float64 {
	sum := 0.0
	if len(a) > len(b) {
		a, b = b, a
	}
	for k, v := range a {
		if w, ok := b[k]; ok {
			sum += v * w
		}
	}
	return sum
}

func norm(vec map[string]float64) float64 {
	sum := 0.0
	for _, v := range vec {
		sum += v * v
	}
	return math.Sqrt(sum)
}

func cosine(a, b map[string]float64) float64 {
	n := norm(a) * norm(b)
	if n == 0 {
		return 0
	}
	return dot(a, b) / n
}

func gameHasTag(tags []Tag, clusters []string, name string) bool {
	for _, tag := range tags {
		if tag.Name == name {
			return true
		}
	}
	for _, c := range clusters {
		if c == name {
			return true
		}
	}
	return false
}

func BuildTaste(games []Game) Taste {
	t := Taste{
		OwnedIDs:   map[int]bool{},
		OwnedNames: map[string]bool{},
		OwnedKeys:  map[string]bool{},
		TagHours:   map[string]float64{},
		Vec:        map[string]float64{},
	}
	for _, game := range games {
		t.OwnedIDs[game.AppID] = true
		t.OwnedNames[NormalizeName(game.Name)] = true
		if key := NameKey(game.Name); key != "" {
			t.OwnedKeys[key] = true
		}
		hours := game.Hours
		clusters := PrimaryTags(game.Tags, 3)
		pg := playedGame{
			appid:          game.AppID,
			name:           game.Name,
			hours:          hours,
			recentlyPlayed: game.RecentlyPlayed,
			tags:           game.Tags,
			clusters:       clusters,
			vec:            tagVector(game.Tags),
		}
		if hours > 0 && hours < bounceHourCap {
			t.Bounced = append(t.Bounced, pg)
		}
		if hours < tasteHourFloor {
			continue
		}
		weight := hours
		if game.RecentlyPlayed {
			weight *= 2.4
		}
		addVector(t.Vec, pg.vec, weight)
		maxW := 0
		for _, tag := range game.Tags {
			if tag.Weight > maxW {
				maxW = tag.Weight
			}
		}
		for _, tag := range game.Tags {
			if TagSpecificity(tag.Name) < 0.5 {
				continue
			}
			share := 0.0
			if maxW > 0 {
				share = float64(tag.Weight) / float64(maxW)
			}
			t.TagHours[tag.Name] += weight * share
		}
		t.TopPlayed = append(t.TopPlayed, pg)
	}
	sort.Slice(t.TopPlayed, func(i, j int) bool { return t.TopPlayed[i].hours > t.TopPlayed[j].hours })
	for _, g := range t.TopPlayed {
		if g.recentlyPlayed {
			t.Recent = append(t.Recent, g)
		}
	}
	if len(t.TopPlayed) > 20 {
		t.TopPlayed = t.TopPlayed[:20]
	}
	type kh struct {
		n string
		h float64
	}
	var ranked []kh
	for n, h := range t.TagHours {
		ranked = append(ranked, kh{n, h})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].h > ranked[j].h })
	total := 0.0
	for _, r := range ranked {
		total += r.h
	}
	for _, r := range ranked {
		pct := 0
		if total > 0 {
			pct = int(math.Round(100 * r.h / total))
		}
		t.RankedClusters = append(t.RankedClusters, TasteCluster{Name: r.n, Hours: r.h, Percent: pct})
	}
	return t
}

func sharedTagNames(left, right []Tag, limit int) []string {
	rightSet := map[string]bool{}
	for _, tag := range right {
		rightSet[tag.Name] = true
	}
	type pair struct {
		name   string
		weight int
	}
	var hits []pair
	for _, tag := range left {
		if rightSet[tag.Name] && TagSpecificity(tag.Name) >= 1 {
			hits = append(hits, pair{tag.Name, tag.Weight})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].weight > hits[j].weight })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.name
	}
	return out
}

func bestComparable(item Game, itemVec map[string]float64, t Taste) *comparable {
	related := map[string]bool{}
	for _, name := range item.Related {
		related[NormalizeName(name)] = true
	}
	var best *comparable
	for _, game := range t.TopPlayed {
		if game.appid == item.AppID {
			continue
		}
		nameHit := related[NormalizeName(game.name)]
		sim := cosine(itemVec, game.vec)
		if !nameHit && sim < 0.32 {
			continue
		}
		bonus := 1 + sim
		if nameHit {
			bonus = 2.6
		}
		value := game.hours * bonus * sim
		if game.recentlyPlayed {
			value *= 1.4
		}
		if best == nil || value > best.value {
			c := comparable{
				game:    game,
				value:   value,
				nameHit: nameHit,
				shared:  sharedTagNames(item.Tags, game.tags, 3),
			}
			best = &c
		}
	}
	return best
}

func bouncePenalty(item Game, t Taste) float64 {
	tags := PrimaryTags(item.Tags, 6)
	penalty := 0.0
	for _, tag := range tags {
		loved := t.TagHours[tag]
		bounced := 0
		for _, g := range t.Bounced {
			if g.appid == item.AppID {
				continue
			}
			if gameHasTag(g.tags, g.clusters, tag) {
				bounced++
			}
		}
		if loved < 8 && bounced >= 2 {
			penalty += 40
		}
		if loved < 3 && bounced >= 1 {
			penalty += 15
		}
	}
	return penalty
}

func formatHours(hours float64) string {
	if hours >= 10 {
		return strconv.Itoa(int(math.Round(hours))) + "h"
	}
	return strconv.FormatFloat(hours, 'f', 1, 64) + "h"
}

func formatTagList(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	default:
		return strings.Join(names[:len(names)-1], ", ") + ", and " + names[len(names)-1]
	}
}

func reasonFor(item Game, comp *comparable, t Taste) string {
	if comp != nil && len(comp.shared) > 0 {
		tags := formatTagList(comp.shared)
		if comp.game.recentlyPlayed {
			return "Like " + comp.game.name + " (" + tags + ") — in your recent play."
		}
		return "Like " + comp.game.name + " · " + tags + " · " + formatHours(comp.game.hours)
	}
	if comp != nil && comp.nameHit {
		return "Related to " + comp.game.name + " (" + formatHours(comp.game.hours) + ")"
	}
	if len(t.Recent) > 0 {
		shared := sharedTagNames(item.Tags, t.Recent[0].tags, 2)
		if len(shared) > 0 {
			return "Close to your recent play · " + t.Recent[0].name
		}
	}
	if len(t.RankedClusters) > 0 && gameHasTag(item.Tags, PrimaryTags(item.Tags, 3), t.RankedClusters[0].Name) {
		top := t.RankedClusters[0]
		return "Hits your " + top.Name + " streak (" + formatHours(top.Hours) + ")"
	}
	return "Fits your library tags"
}

func scoreItem(item Game, t Taste, options Options) Card {
	itemVec := tagVector(item.Tags)
	comp := bestComparable(item, itemVec, t)
	var allTags []Tag
	for _, game := range t.TopPlayed {
		allTags = append(allTags, game.tags...)
	}
	hits := sharedTagNames(item.Tags, allTags, 8)
	bounce := bouncePenalty(item, t)
	compPart := 0.0
	if comp != nil {
		compPart = math.Min(1, comp.value/260) * 0.28
	}
	familiarRaw := math.Max(0, cosine(t.Vec, itemVec)*0.72+compPart-bounce/500)
	onWishlist := options.Wishlist[item.AppID]
	if onWishlist {
		familiarRaw = math.Min(1, familiarRaw+0.18)
	}
	tagNames := make([]string, 0, 12)
	for i, tag := range item.Tags {
		if i >= 12 {
			break
		}
		if tag.Name != "" {
			tagNames = append(tagNames, tag.Name)
		}
	}
	header := item.Header
	if header == "" || !strings.Contains(header, strconv.Itoa(item.AppID)) {
		header = "https://cdn.akamai.steamstatic.com/steam/apps/" + strconv.Itoa(item.AppID) + "/header.jpg"
	}
	pop := item.Popularity
	if pop == 0 {
		pop = 50
	}
	because := ""
	if comp != nil {
		because = comp.game.name
	}
	reason := reasonFor(item, comp, t)
	if onWishlist {
		reason = "On your Steam wishlist. " + reason
	}
	return Card{
		AppID:           item.AppID,
		Name:            item.Name,
		Clusters:        PrimaryTags(item.Tags, 3),
		Tags:            tagNames,
		Popularity:      pop,
		Reviews:         item.Reviews,
		Positive:        item.Positive,
		SteamURL:        "https://store.steampowered.com/app/" + strconv.Itoa(item.AppID) + "/",
		PageURL:         "/app/" + strconv.Itoa(item.AppID),
		Header:          header,
		Fit:             familiarRaw,
		Reason:          reason,
		Because:         because,
		OnWishlist:      onWishlist,
		familiarRaw:     familiarRaw,
		similarity:      cosine(t.Vec, itemVec),
		tagPref:         tagPrefScore(item.Tags, options.TagWeights),
		distinctiveHits: hits,
	}
}

func Clamp01(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func inTasteRange(item Card) bool {
	return item.similarity >= 0.22 || len(item.distinctiveHits) >= 2 || item.Because != ""
}

func selectNeighbors(items []Card, min int) []Card {
	var close []Card
	for _, item := range items {
		if inTasteRange(item) {
			close = append(close, item)
		}
	}
	if len(close) >= min {
		return close
	}
	keep := map[int]bool{}
	for _, item := range close {
		keep[item.AppID] = true
	}
	byFit := append([]Card(nil), items...)
	sort.Slice(byFit, func(i, j int) bool {
		if byFit[i].familiarRaw != byFit[j].familiarRaw {
			return byFit[i].familiarRaw > byFit[j].familiarRaw
		}
		return byFit[i].Name < byFit[j].Name
	})
	for _, item := range byFit {
		if len(keep) >= min {
			break
		}
		keep[item.AppID] = true
	}
	var out []Card
	for _, item := range items {
		if keep[item.AppID] {
			out = append(out, item)
		}
	}
	return out
}

func qualityValue(item Card) float64 {
	if item.Positive > 0 {
		return float64(item.Positive)
	}
	return 70
}

func axisScore(items []Card, value func(Card) float64) func(Card) float64 {
	sorted := append([]Card(nil), items...)
	sort.Slice(sorted, func(i, j int) bool {
		vi, vj := value(sorted[i]), value(sorted[j])
		if vi != vj {
			return vi > vj
		}
		return sorted[i].Name < sorted[j].Name
	})
	rank := map[int]int{}
	for i, item := range sorted {
		rank[item.AppID] = i
	}
	last := math.Max(1, float64(len(items)-1))
	return func(item Card) float64 {
		return 1 - float64(rank[item.AppID])/last
	}
}

func AttachPrices(items []Card, prices map[int]store.PriceView) {
	for i := range items {
		price, ok := prices[items[i].AppID]
		if !ok {
			continue
		}
		items[i].Price = price.Formatted
		items[i].Discount = price.Discount
		items[i].OnSale = price.OnSale
		items[i].LowPrice = price.Low
		items[i].AtLow = price.AtLow
	}
}

func StampPrices(result *Result, prices map[int]store.PriceView) {
	if result == nil {
		return
	}
	AttachPrices(result.Groups.MoreLike, prices)
	AttachPrices(result.Groups.Adjacent, prices)
	AttachPrices(result.Groups.Wildcard, prices)
}

func StampSparklines(result *Result, lines map[int][]int) {
	if result == nil {
		return
	}
	stamp := func(items []Card) {
		for i := range items {
			if line, ok := lines[items[i].AppID]; ok {
				items[i].Sparkline = line
			}
		}
	}
	stamp(result.Groups.MoreLike)
	stamp(result.Groups.Adjacent)
	stamp(result.Groups.Wildcard)
}

// StampNames overwrites card titles from a trusted appid→name map (fixes stale catalog names).
func StampNames(result *Result, names map[int]string) {
	if result == nil || len(names) == 0 {
		return
	}
	stamp := func(items []Card) {
		for i := range items {
			if name := names[items[i].AppID]; name != "" {
				items[i].Name = name
			}
		}
	}
	stamp(result.Groups.MoreLike)
	stamp(result.Groups.Adjacent)
	stamp(result.Groups.Wildcard)
}

func ownsTitle(t Taste, item Game) bool {
	if t.OwnedIDs[item.AppID] {
		return true
	}
	if t.OwnedNames[NormalizeName(item.Name)] {
		return true
	}
	key := NameKey(item.Name)
	return key != "" && t.OwnedKeys[key]
}

func skipItem(item Game, t Taste, options Options) bool {
	if item.AppID == 0 {
		return true
	}
	if ownsTitle(t, item) {
		return true
	}
	if IsJunkName(item.Name) {
		return true
	}
	if options.Hidden[item.AppID] {
		return true
	}
	if options.HideAdult && HasAdultTags(item.Tags) {
		return true
	}
	if options.HideGore && HasGoreTags(item.Tags) {
		return true
	}
	if options.SkipUnknown && !item.HasReviews {
		return true
	}
	if options.MinReviews > 0 && item.Reviews < options.MinReviews {
		return true
	}
	if options.MaxReviews > 0 && item.Reviews > options.MaxReviews {
		return true
	}
	if options.MinPositive > 0 && item.Positive < options.MinPositive {
		return true
	}
	if !HasAllTagNames(item.Tags, options.IncludeTags) {
		return true
	}
	if HasAnyTagName(item.Tags, options.ExcludeTags) {
		return true
	}
	return false
}

func publicCard(item Card) Card {
	item.familiarRaw = 0
	item.similarity = 0
	item.tagPref = 0
	item.distinctiveHits = nil
	return item
}

func Recommend(games []Game, catalog []Game, options Options) Result {
	t := BuildTaste(games)
	popularityDial := Clamp01(options.Popularity)
	weirdness := Clamp01(options.Weirdness)
	wMainstream := popularityDial
	wFamiliar := 1 - weirdness
	tagWeights := NormalizeTagWeights(options.TagWeights)
	options.TagWeights = tagWeights

	seen := map[int]bool{}
	var cards []Card
	pool := append(append([]Game{}, catalog...), options.ExtraCatalog...)
	for _, item := range pool {
		if seen[item.AppID] || skipItem(item, t, options) {
			continue
		}
		seen[item.AppID] = true
		cards = append(cards, scoreItem(item, t, options))
	}
	AttachPrices(cards, options.Prices)
	neighbors := cards
	if options.OnSale {
		var onSale []Card
		for _, item := range cards {
			if item.OnSale {
				onSale = append(onSale, item)
			}
		}
		neighbors = onSale
	}
	poolSize := len(neighbors)
	neighbors = selectNeighbors(neighbors, 24)

	mainstreamOf := axisScore(neighbors, func(item Card) float64 { return float64(item.Popularity) })
	familiarOf := axisScore(neighbors, func(item Card) float64 { return item.familiarRaw })
	qualityOf := axisScore(neighbors, qualityValue)
	tagOf := axisScore(neighbors, func(item Card) float64 { return item.tagPref })
	for i := range neighbors {
		mainstream := mainstreamOf(neighbors[i])
		familiar := familiarOf(neighbors[i])
		quality := qualityOf(neighbors[i])
		if len(tagWeights) > 0 {
			neighbors[i].Score = int(math.Round(
				160*quality +
					270*(wMainstream*mainstream+(1-wMainstream)*(1-mainstream)) +
					270*(wFamiliar*familiar+(1-wFamiliar)*(1-familiar)) +
					450*tagOf(neighbors[i]),
			))
		} else {
			neighbors[i].Score = int(math.Round(
				220*quality +
					390*(wMainstream*mainstream+(1-wMainstream)*(1-mainstream)) +
					390*(wFamiliar*familiar+(1-wFamiliar)*(1-familiar)),
			))
		}
	}
	sort.Slice(neighbors, func(i, j int) bool {
		if neighbors[i].Score != neighbors[j].Score {
			return neighbors[i].Score > neighbors[j].Score
		}
		return neighbors[i].Name < neighbors[j].Name
	})

	more := []Card{}
	adjacent := []Card{}
	if len(t.RankedClusters) == 0 {
		cap := 24
		if cap > len(neighbors) {
			cap = len(neighbors)
		}
		for _, item := range neighbors[:cap] {
			more = append(more, publicCard(item))
		}
	} else if len(neighbors) > 0 {
		end := 12
		if end > len(neighbors) {
			end = len(neighbors)
		}
		for _, item := range neighbors[:end] {
			more = append(more, publicCard(item))
		}
		if len(neighbors) > 12 {
			adjEnd := 18
			if adjEnd > len(neighbors) {
				adjEnd = len(neighbors)
			}
			for _, item := range neighbors[12:adjEnd] {
				adjacent = append(adjacent, publicCard(item))
			}
		}
	}
	used := map[int]bool{}
	for _, item := range more {
		used[item.AppID] = true
	}
	for _, item := range adjacent {
		used[item.AppID] = true
	}
	var leftover []Card
	for _, item := range neighbors {
		if !used[item.AppID] {
			leftover = append(leftover, item)
		}
	}
	minWild := leftover
	var inRange []Card
	for _, item := range leftover {
		if item.familiarRaw >= 0.2 {
			inRange = append(inRange, item)
		}
	}
	if len(inRange) > 0 {
		minWild = inRange
	}
	sort.Slice(minWild, func(i, j int) bool {
		if weirdness >= 0.5 {
			if minWild[i].familiarRaw != minWild[j].familiarRaw {
				return minWild[i].familiarRaw > minWild[j].familiarRaw
			}
		} else if minWild[i].familiarRaw != minWild[j].familiarRaw {
			return minWild[i].familiarRaw < minWild[j].familiarRaw
		}
		return minWild[i].Name < minWild[j].Name
	})
	wildcard := []Card{}
	if len(minWild) > 0 {
		wildcard = []Card{publicCard(minWild[0])}
	}

	var result Result
	if len(t.RankedClusters) > 0 {
		name := t.RankedClusters[0].Name
		result.Taste.Heaviest = &name
	}
	clusters := t.RankedClusters
	if len(clusters) > 6 {
		clusters = clusters[:6]
	}
	if clusters == nil {
		clusters = []TasteCluster{}
	}
	result.Taste.Clusters = clusters
	recent := make([]string, 0, len(t.Recent))
	for _, game := range t.Recent {
		recent = append(recent, game.name)
	}
	result.Taste.Recent = recent
	topN := t.TopPlayed
	if len(topN) > 8 {
		topN = topN[:8]
	}
	topPlayed := make([]PlayedSummary, 0, len(topN))
	for _, game := range topN {
		topPlayed = append(topPlayed, PlayedSummary{Name: game.name, Hours: game.hours})
	}
	result.Taste.TopPlayed = topPlayed
	result.Dials.Popularity = popularityDial
	result.Dials.Weirdness = weirdness
	result.Dials.OnSale = options.OnSale
	result.Dials.SkipShovelware = options.SkipShovelware
	result.Dials.MinReviews = options.MinReviews
	result.Dials.MaxReviews = options.MaxReviews
	result.Dials.MinPositive = options.MinPositive
	result.Dials.IncludeTags = options.IncludeTags
	if result.Dials.IncludeTags == nil {
		result.Dials.IncludeTags = []string{}
	}
	result.Dials.ExcludeTags = options.ExcludeTags
	if result.Dials.ExcludeTags == nil {
		result.Dials.ExcludeTags = []string{}
	}
	result.Dials.TagWeights = tagWeights
	if result.Dials.TagWeights == nil {
		result.Dials.TagWeights = []TagWeight{}
	}
	result.Pool = poolSize
	result.Groups.MoreLike = more
	result.Groups.Adjacent = adjacent
	result.Groups.Wildcard = wildcard
	return result
}
