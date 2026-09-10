package web

import (
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"steam-suggestions/internal/recommend"
	"steam-suggestions/internal/steam"
)

const (
	coverMissTTL  = 24 * time.Hour
	coverFetchers = 8
)

var errCoverMissing = errors.New("cover not found")

type coverCall struct {
	done chan struct{}
	data []byte
	err  error
}

func localizeCovers(result *recommend.Result) {
	if result == nil {
		return
	}
	fix := func(cards []recommend.Card) {
		for i := range cards {
			if cards[i].AppID > 0 {
				cards[i].Header = steam.CachedCoverURL(cards[i].AppID)
			}
		}
	}
	fix(result.Groups.MoreLike)
	fix(result.Groups.Adjacent)
	fix(result.Groups.Wildcard)
}

func (s *Server) coverPath(appid int) string {
	return filepath.Join(s.ImageDir, strconv.Itoa(appid)+".jpg")
}

func (s *Server) handleCover(w http.ResponseWriter, r *http.Request) {
	appid, ok := steam.ParseCoverAppID(r.PathValue("appid"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	raw, err := s.getCover(appid)
	if err != nil {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	if steam.LooksLikeImage(raw) && raw[0] == 0x89 {
		w.Header().Set("Content-Type", "image/png")
	}
	w.Header().Set("Cache-Control", "public, max-age=2592000")
	_, _ = w.Write(raw)
}

func (s *Server) getCover(appid int) ([]byte, error) {
	path := s.coverPath(appid)
	if raw, err := os.ReadFile(path); err == nil && steam.LooksLikeImage(raw) {
		return raw, nil
	}

	s.coverMu.Lock()
	if t, ok := s.coverMiss[appid]; ok && time.Since(t) < coverMissTTL {
		s.coverMu.Unlock()
		return nil, errCoverMissing
	}
	if call, ok := s.coverInflight[appid]; ok {
		s.coverMu.Unlock()
		<-call.done
		return call.data, call.err
	}
	call := &coverCall{done: make(chan struct{})}
	s.coverInflight[appid] = call
	s.coverMu.Unlock()

	s.coverSem <- struct{}{}
	raw, err := s.Client.FetchCover(appid)
	<-s.coverSem

	if err != nil {
		s.coverMu.Lock()
		s.coverMiss[appid] = time.Now()
		s.coverMu.Unlock()
	} else if writeErr := s.writeCover(appid, raw); writeErr != nil {
		log.Printf("cover cache write %d: %v", appid, writeErr)
	}

	call.data, call.err = raw, err
	close(call.done)
	s.coverMu.Lock()
	delete(s.coverInflight, appid)
	s.coverMu.Unlock()
	if err != nil {
		return nil, errCoverMissing
	}
	return raw, nil
}

func (s *Server) writeCover(appid int, raw []byte) error {
	if err := os.MkdirAll(s.ImageDir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.ImageDir, strconv.Itoa(appid)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.coverPath(appid))
}
