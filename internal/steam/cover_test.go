package steam

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCachedCoverURL(t *testing.T) {
	if CachedCoverURL(730) != "/images/730.jpg" {
		t.Fatal(CachedCoverURL(730))
	}
	if CachedCoverURL(0) != "" {
		t.Fatal("zero appid should be empty")
	}
}

func TestCoverURLsPreferHeader(t *testing.T) {
	urls := CoverURLs(2420510)
	if len(urls) < 3 || !strings.Contains(urls[0], "/2420510/header.jpg") {
		t.Fatalf("expected header first, got %v", urls)
	}
}

func TestFetchCoverFallsBackPastMissingHeader(t *testing.T) {
	jpeg := append([]byte{0xff, 0xd8, 0xff, 0xe0}, bytesOf('x', 32)...)
	var hits []string
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/header.jpg") {
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/capsule_616x353.jpg") {
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(jpeg)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(cdn.Close)

	client := New("", "US", "USD")
	client.HTTP = &http.Client{Transport: rewriteHost{base: cdn.URL, next: cdn.Client().Transport}}
	raw, err := client.FetchCover(9)
	if err != nil {
		t.Fatal(err)
	}
	if !LooksLikeImage(raw) {
		t.Fatal("expected jpeg")
	}
	joined := strings.Join(hits, " ")
	if !strings.Contains(joined, "header.jpg") || !strings.Contains(joined, "capsule_616x353.jpg") {
		t.Fatalf("expected header then capsule, got %v", hits)
	}
}

func TestParseCoverAppID(t *testing.T) {
	id, ok := ParseCoverAppID("730.jpg")
	if !ok || id != 730 {
		t.Fatalf("got %d %v", id, ok)
	}
	if _, ok := ParseCoverAppID("nope"); ok {
		t.Fatal("invalid id")
	}
}

type rewriteHost struct {
	base string
	next http.RoundTripper
}

func (r rewriteHost) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	u := strings.TrimSuffix(r.base, "/")
	clone.URL.Scheme = "http"
	if strings.HasPrefix(u, "https://") {
		clone.URL.Scheme = "https"
		u = strings.TrimPrefix(u, "https://")
	} else {
		u = strings.TrimPrefix(u, "http://")
	}
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
