package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"steam-suggestions/internal/steam"
	"steam-suggestions/internal/store"
)

func (s *Server) requireUser(r *http.Request) (store.User, error) {
	user, ok := s.sessionUser(r)
	if !ok {
		return store.User{}, steam.Status(http.StatusUnauthorized, "Sign in through Steam first.")
	}
	return user, nil
}

func (s *Server) handleListSearches(w http.ResponseWriter, r *http.Request) {
	user, err := s.requireUser(r)
	if err != nil {
		panic(err)
	}
	items, err := s.DB.ListSavedSearches(user.SteamID)
	if err != nil {
		panic(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"searches": items})
}

func (s *Server) handleCreateSearch(w http.ResponseWriter, r *http.Request) {
	user, err := s.requireUser(r)
	if err != nil {
		panic(err)
	}
	var body struct {
		Name    string          `json:"name"`
		Terms   string          `json:"terms"`
		Kind    string          `json:"kind"`
		Filters json.RawMessage `json:"filters"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		panic(steam.Status(http.StatusBadRequest, "Invalid JSON body."))
	}
	item, err := s.DB.CreateSavedSearch(user.SteamID, body.Name, body.Terms, body.Kind, body.Filters)
	if err != nil {
		if store.IsSearchError(err) {
			panic(steam.Status(http.StatusBadRequest, err.Error()))
		}
		panic(err)
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) handleDeleteSearch(w http.ResponseWriter, r *http.Request) {
	user, err := s.requireUser(r)
	if err != nil {
		panic(err)
	}
	id, convErr := strconv.Atoi(r.PathValue("id"))
	if convErr != nil || id <= 0 {
		panic(steam.Status(http.StatusBadRequest, "Unknown saved search."))
	}
	if !s.DB.DeleteSavedSearch(user.SteamID, id) {
		panic(steam.Status(http.StatusNotFound, "Saved search not found."))
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
