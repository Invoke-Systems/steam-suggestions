package web

import (
	"html"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"steam-suggestions/internal/env"
)

var (
	sampleRateRe  = regexp.MustCompile(`^(0(\.\d+)?|1(\.0+)?)$`)
	maskLevelRe   = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,32}$`)
	maxDurationRe = regexp.MustCompile(`^\d{1,10}$`)
)

func analyticsHTML() template.HTML {
	id := strings.TrimSpace(env.Get("UMAMI_WEBSITE_ID", ""))
	if id == "" {
		return ""
	}
	src := strings.TrimSpace(env.Get("UMAMI_SCRIPT_URL", "https://cloud.umami.is/script.js"))
	if !validAnalyticsURL(src) || !validWebsiteID(id) {
		return ""
	}
	attrs := []string{
		`defer`,
		`src="` + html.EscapeString(src) + `"`,
		`data-website-id="` + html.EscapeString(id) + `"`,
	}
	if rate := strings.TrimSpace(env.Get("UMAMI_SAMPLE_RATE", "")); rate != "" {
		if !sampleRateRe.MatchString(rate) {
			return ""
		}
		attrs = append(attrs, `data-sample-rate="`+html.EscapeString(rate)+`"`)
	}
	if level := strings.TrimSpace(env.Get("UMAMI_MASK_LEVEL", "")); level != "" {
		if !maskLevelRe.MatchString(level) {
			return ""
		}
		attrs = append(attrs, `data-mask-level="`+html.EscapeString(level)+`"`)
	}
	if dur := strings.TrimSpace(env.Get("UMAMI_MAX_DURATION", "")); dur != "" {
		if !maxDurationRe.MatchString(dur) {
			return ""
		}
		attrs = append(attrs, `data-max-duration="`+html.EscapeString(dur)+`"`)
	}
	return template.HTML("<script " + strings.Join(attrs, " ") + "></script>")
}

func validWebsiteID(id string) bool {
	if len(id) < 8 || len(id) > 80 {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	return true
}

func validAnalyticsURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return false
	}
	if strings.Contains(u.Host, "/") || strings.Contains(raw, " ") {
		return false
	}
	return true
}

func analyticsOrigin() string {
	src := strings.TrimSpace(env.Get("UMAMI_SCRIPT_URL", "https://cloud.umami.is/script.js"))
	if strings.TrimSpace(env.Get("UMAMI_WEBSITE_ID", "")) == "" || !validAnalyticsURL(src) {
		return ""
	}
	u, err := url.Parse(src)
	if err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func setHTMLHeaders(w http.ResponseWriter, opts htmlHeaderOpts) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if opts.CacheControl != "" {
		w.Header().Set("Cache-Control", opts.CacheControl)
	} else {
		w.Header().Set("Cache-Control", "no-store")
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", contentSecurityPolicy(opts))
}

type htmlHeaderOpts struct {
	CacheControl string
	Avatars      bool
}

func contentSecurityPolicy(opts htmlHeaderOpts) string {
	img := []string{
		"'self'",
		"https://shared.akamai.steamstatic.com",
		"https://cdn.akamai.steamstatic.com",
		"https://media.steampowered.com",
		"data:",
	}
	if opts.Avatars {
		img = append(img, "https://avatars.steamstatic.com")
	}
	script := []string{"'self'"}
	connect := []string{"'self'"}
	if origin := analyticsOrigin(); origin != "" {
		script = append(script, origin)
		connect = append(connect, origin)
	}
	return strings.Join([]string{
		"default-src 'self'",
		"img-src " + strings.Join(img, " "),
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
		"font-src https://fonts.gstatic.com",
		"script-src " + strings.Join(script, " "),
		"connect-src " + strings.Join(connect, " "),
		"object-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
	}, "; ")
}
