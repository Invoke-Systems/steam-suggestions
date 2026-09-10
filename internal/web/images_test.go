package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"steam-suggestions/internal/recommend"
	"steam-suggestions/internal/steam"
)

func TestHandleCoverCachesToDisk(t *testing.T) {
	jpeg := append([]byte{0xff, 0xd8, 0xff, 0xe0}, bytesOf('x', 48)...)
	var hits int
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(jpeg)
	}))
	t.Cleanup(cdn.Close)

	dir := t.TempDir()
	client := steam.New("", "US", "USD")
	client.HTTP = &http.Client{Transport: rewriteHost{base: cdn.URL, next: http.DefaultTransport}}
	s := &Server{
		Client:        client,
		ImageDir:      dir,
		coverInflight: map[int]*coverCall{},
		coverMiss:     map[int]time.Time{},
		coverSem:      make(chan struct{}, coverFetchers),
	}

	req := httptest.NewRequest(http.MethodGet, "/images/730.jpg", nil)
	req.SetPathValue("appid", "730.jpg")
	rec := httptest.NewRecorder()
	s.handleCover(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(dir, "730.jpg")); err != nil {
		t.Fatal(err)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/images/730.jpg", nil)
	req2.SetPathValue("appid", "730.jpg")
	rec2 := httptest.NewRecorder()
	s.handleCover(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("second status %d", rec2.Code)
	}
	if hits != 1 {
		t.Fatalf("expected one Steam fetch, got %d", hits)
	}
}

func TestHandleCoverRemembersMisses(t *testing.T) {
	var hits int
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.NotFound(w, r)
	}))
	t.Cleanup(cdn.Close)

	client := steam.New("", "US", "USD")
	client.HTTP = &http.Client{Transport: rewriteHost{base: cdn.URL, next: http.DefaultTransport}}
	s := &Server{
		Client:        client,
		ImageDir:      t.TempDir(),
		coverInflight: map[int]*coverCall{},
		coverMiss:     map[int]time.Time{},
		coverSem:      make(chan struct{}, coverFetchers),
	}
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/images/1.jpg", nil)
		req.SetPathValue("appid", "1.jpg")
		rec := httptest.NewRecorder()
		s.handleCover(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status %d", rec.Code)
		}
	}
	if hits < 1 {
		t.Fatal("expected a Steam fetch")
	}
	if hits > len(steam.CoverURLs(1)) {
		t.Fatalf("negative cache should stop repeats, got %d hits", hits)
	}
}

func TestLocalizeCovers(t *testing.T) {
	result := recommend.Result{}
	result.Groups.MoreLike = []recommend.Card{{AppID: 730, Header: "https://example"}}
	localizeCovers(&result)
	if result.Groups.MoreLike[0].Header != "/images/730.jpg" {
		t.Fatal(result.Groups.MoreLike[0].Header)
	}
}

type rewriteHost struct {
	base string
	next http.RoundTripper
}

func (r rewriteHost) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	u := strings.TrimPrefix(strings.TrimPrefix(r.base, "https://"), "http://")
	clone.URL.Scheme = "http"
	clone.URL.Host = u
	clone.Host = u
	next := r.next
	if next == nil {
		next = http.DefaultTransport
	}
	return next.RoundTrip(clone)
}

func bytesOf(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}
