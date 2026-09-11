package web

import (
	"log"
	"net/http"
	"os"
	"strings"

	"steam-suggestions/internal/steam"
	"steam-suggestions/internal/store"
)

const sessionCookie = "steam_suggestions_session"

func (s *Server) publicOrigin(r *http.Request) string {
	if origin := strings.TrimRight(strings.TrimSpace(s.PublicURL), "/"); origin != "" {
		return origin
	}
	proto := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		proto = "https"
	}
	host := r.Host
	if host == "" {
		host = "localhost:" + envOr("PORT", "3847")
	}
	return proto + "://" + host
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func (s *Server) cookieSecure(r *http.Request) bool {
	return strings.HasPrefix(s.publicOrigin(r), "https://")
}

func (s *Server) sessionUser(r *http.Request) (store.User, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return store.User{}, false
	}
	return s.DB.SessionUser(cookie.Value)
}

func (s *Server) publicUser(user store.User) map[string]any {
	return map[string]any{
		"steamid":     user.SteamID,
		"name":        user.Name,
		"avatar":      user.Avatar,
		"createdAt":   user.CreatedAt,
		"lastLoginAt": user.LastLoginAt,
		"profileUrl":  "https://steamcommunity.com/profiles/" + user.SteamID,
	}
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int((30 * 24 * 60 * 60)),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cookieSecure(r),
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cookieSecure(r),
	})
}

func (s *Server) handleSteamLogin(w http.ResponseWriter, r *http.Request) {
	origin := s.publicOrigin(r)
	http.Redirect(w, r, steam.OpenIDAuthURL(steam.OpenIDReturnTo(origin), steam.OpenIDRealm(origin)), http.StatusFound)
}

func (s *Server) handleSteamCallback(w http.ResponseWriter, r *http.Request) {
	origin := s.publicOrigin(r)
	returnTo := steam.OpenIDReturnTo(origin)
	client := s.Client.HTTP
	steamid, err := steam.ValidateOpenID(r.URL.Query(), returnTo, client)
	if err != nil {
		log.Printf("steam login failed: %v", err)
		http.Redirect(w, r, origin+"/?login=error", http.StatusFound)
		return
	}
	user := store.User{SteamID: steamid}
	if key := s.envKey(); key != "" {
		summary, sumErr := s.clientFor(key).PlayerSummary(steamid)
		if sumErr == nil && summary != nil {
			user.Name = summary.PersonaName
			user.Avatar = summary.AvatarFull
			if user.Avatar == "" {
				user.Avatar = summary.AvatarMedium
			}
		}
	}
	if err := s.DB.UpsertUser(user); err != nil {
		log.Printf("steam login user persist: %v", err)
		http.Redirect(w, r, origin+"/?login=error", http.StatusFound)
		return
	}
	token, err := s.DB.CreateSession(steamid)
	if err != nil {
		log.Printf("steam login session: %v", err)
		http.Redirect(w, r, origin+"/?login=error", http.StatusFound)
		return
	}
	s.setSessionCookie(w, r, token)
	// Bind Discover to the account that just signed in (clears any prior guest focus).
	s.clearFocusCookie(w, r)
	if err := s.setFocus(w, r, steamid); err != nil {
		log.Printf("steam login focus: %v", err)
	}
	// Relative redirect keeps the session cookie on this host.
	http.Redirect(w, r, "/sift", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		s.DB.DeleteSession(cookie.Value)
	}
	s.clearSessionCookie(w, r)
	s.clearFocusCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
