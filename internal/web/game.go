package web

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"steam-suggestions/internal/steam"
)

var gameTmpl = template.Must(template.ParseFS(tmplFS, "templates/game.html"))

type gameView struct {
	AppID        int
	Name         string
	Header       string
	SteamURL     string
	Type         string
	ReleaseDate  string
	Developers   string
	Publishers   string
	Description  string
	Genres       []string
	Tags         []string
	Players      string
	Peak         string
	Reviews      string
	Price        string
	PriceNote    string
	PriceSVG     template.HTML
	PlayerSVG    template.HTML
	Comments     []gameComment
	News         []gameNews
	Achievements []gameAchievement
	SignedIn     bool
	HasData      bool
	Analytics    template.HTML
}

type gameComment struct {
	Text    string
	URL     string
	VotedUp bool
	Hours   string
	Date    string
}

type gameNews struct {
	Title string
	URL   string
	Date  string
	Body  string
}

type gameAchievement struct {
	Name    string
	Percent string
	Width   int
}

func (s *Server) handleGame(w http.ResponseWriter, r *http.Request) {
	appid, err := strconv.Atoi(r.PathValue("appid"))
	if err != nil || appid <= 0 {
		http.NotFound(w, r)
		return
	}
	view := s.gameFromDB(appid)
	view.Comments = s.gameComments(appid)
	view.Analytics = analyticsHTML()
	_, view.SignedIn = s.sessionUser(r)
	setHTMLHeaders(w, htmlHeaderOpts{CacheControl: "public, max-age=120"})
	if err := gameTmpl.Execute(w, view); err != nil {
		log.Printf("game template: %v", err)
	}
}

func (s *Server) gameFromDB(appid int) gameView {
	view := gameView{
		AppID:    appid,
		Name:     "Steam app " + strconv.Itoa(appid),
		Header:   steam.HeaderURL(appid),
		SteamURL: steam.StoreURL(appid),
	}
	if game, ok := s.DB.GetGame(appid); ok && strings.TrimSpace(game.Name) != "" {
		view.Name = game.Name
		view.HasData = true
	}
	if details, ok := s.DB.GetAppDetails([]int{appid})[appid]; ok && !details.Missing {
		view.HasData = true
		if details.Type != "" {
			view.Type = details.Type
		}
		view.ReleaseDate = details.ReleaseDate
		view.Developers = strings.Join(details.Developers, ", ")
		view.Publishers = strings.Join(details.Publishers, ", ")
		view.Description = details.ShortDescription
		view.Genres = details.Genres
	}
	tags := s.DB.GetTags([]int{appid})[appid]
	for i, tag := range tags {
		if i >= 12 {
			break
		}
		view.Tags = append(view.Tags, tag.Name)
	}
	if len(view.Tags) > 0 {
		view.HasData = true
	}
	if st, ok := s.DB.GetPlayerStats([]int{appid})[appid]; ok {
		view.HasData = true
		if st.Current > 0 {
			view.Players = commaInt(st.Current)
		}
		if st.PeakAll > 0 {
			view.Peak = commaInt(st.PeakAll)
		}
	}
	if rev, ok := s.DB.GetReviews([]int{appid})[appid]; ok && rev.Total > 0 {
		view.HasData = true
		view.Reviews = strconv.Itoa(rev.Positive) + "% · " + commaInt(rev.Total)
	}
	if price, ok := s.DB.GetPrices([]int{appid})[appid]; ok {
		view.HasData = true
		view.Price = price.Formatted
		if price.AtLow && price.Low != nil {
			view.PriceNote = "at all-time Steam low (" + *price.Low + ")"
		} else if price.OnSale && price.Discount > 0 {
			view.PriceNote = strconv.Itoa(price.Discount) + "% off"
		} else if price.Low != nil {
			view.PriceNote = "Steam low " + *price.Low
		}
	}
	priceHist := s.DB.GetPriceHistory(appid, 730)
	if len(priceHist) >= 2 {
		vals := make([]int, len(priceHist))
		for i, p := range priceHist {
			vals[i] = p.Final
		}
		view.PriceSVG = sparklineHTML(vals, 520, 64)
	}
	playerHist := s.DB.GetPlayerHistory(appid, 14)
	if len(playerHist) >= 2 {
		vals := make([]int, len(playerHist))
		for i, p := range playerHist {
			vals[i] = p.Current
		}
		view.PlayerSVG = sparklineHTML(vals, 520, 64)
	}
	for _, item := range s.DB.GetAppNews(appid, 6) {
		view.News = append(view.News, gameNews{
			Title: item.Title,
			URL:   item.URL,
			Date:  formatUnixDate(item.Date),
			Body:  item.Contents,
		})
	}
	if ach, ok := s.DB.GetAchievements(appid); ok && len(ach) > 0 {
		view.HasData = true
		sort.Slice(ach, func(i, j int) bool { return ach[i].Percent < ach[j].Percent })
		n := len(ach)
		if n > 8 {
			n = 8
		}
		for _, a := range ach[:n] {
			w := int(a.Percent + 0.5)
			if w < 0 {
				w = 0
			}
			if w > 100 {
				w = 100
			}
			view.Achievements = append(view.Achievements, gameAchievement{
				Name:    a.Name,
				Percent: fmt.Sprintf("%.1f%%", a.Percent),
				Width:   w,
			})
		}
	}
	return view
}

func (s *Server) gameComments(appid int) []gameComment {
	if s.Client == nil || s.DB == nil {
		return nil
	}
	items := s.Client.EnsureReviewSnippets(s.DB, appid, 5)
	if len(items) == 0 {
		return nil
	}
	out := make([]gameComment, 0, len(items))
	for _, item := range items {
		c := gameComment{
			Text:    item.Text,
			URL:     item.URL,
			VotedUp: item.VotedUp,
			Date:    formatUnixDate(item.CreatedAt),
		}
		if item.Hours > 0 {
			c.Hours = commaInt(item.Hours)
		}
		out = append(out, c)
	}
	return out
}

func formatUnixDate(sec int64) string {
	if sec <= 0 {
		return ""
	}
	return time.Unix(sec, 0).UTC().Format("Jan 2, 2006")
}

func sparklineHTML(values []int, width, height int) template.HTML {
	if len(values) < 2 || width <= 0 || height <= 0 {
		return ""
	}
	min, max := values[0], values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	span := max - min
	if span < 1 {
		span = 1
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<svg class="sparkline chart" viewBox="0 0 %d %d" preserveAspectRatio="none" aria-hidden="true"><polyline fill="none" stroke="currentColor" stroke-width="2" points="`, width, height))
	for i, v := range values {
		x := float64(i)/float64(len(values)-1)*float64(width-2) + 1
		y := float64(height-2) - (float64(v-min)/float64(span))*float64(height-4)
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%.1f,%.1f", x, y)
	}
	b.WriteString(`"/></svg>`)
	return template.HTML(b.String())
}
