package steam

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestParseClaimedSteamID(t *testing.T) {
	id, err := ParseClaimedSteamID("https://steamcommunity.com/openid/id/76561198000000000")
	if err != nil || id != "76561198000000000" {
		t.Fatalf("got %q %v", id, err)
	}
	if _, err := ParseClaimedSteamID("https://evil.example/openid/id/76561198000000000"); err == nil {
		t.Fatal("foreign host should fail")
	}
}

func TestValidateOpenIDRejectsReturnToMismatch(t *testing.T) {
	q := url.Values{}
	q.Set("openid.ns", openIDNS)
	q.Set("openid.mode", "id_res")
	q.Set("openid.return_to", "https://other.example/auth/steam/callback")
	q.Set("openid.op_endpoint", OpenIDEndpoint)
	q.Set("openid.claimed_id", "https://steamcommunity.com/openid/id/76561198000000000")
	if _, err := ValidateOpenID(q, "https://mine.example/auth/steam/callback", http.DefaultClient); err == nil {
		t.Fatal("expected return_to mismatch")
	}
}

func TestValidateOpenIDChecksSteam(t *testing.T) {
	var gotMode string
	steamSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotMode = r.Form.Get("openid.mode")
		_, _ = w.Write([]byte("ns:" + openIDNS + "\nis_valid:true\n"))
	}))
	t.Cleanup(steamSrv.Close)

	q := url.Values{}
	q.Set("openid.ns", openIDNS)
	q.Set("openid.mode", "id_res")
	returnTo := "http://localhost:3847/auth/steam/callback"
	q.Set("openid.return_to", returnTo)
	q.Set("openid.op_endpoint", OpenIDEndpoint)
	q.Set("openid.claimed_id", "https://steamcommunity.com/openid/id/76561198000000000")
	q.Set("openid.identity", "https://steamcommunity.com/openid/id/76561198000000000")

	client := &http.Client{Transport: rewriteHost{base: steamSrv.URL, next: http.DefaultTransport}}
	id, err := ValidateOpenID(q, returnTo, client)
	if err != nil {
		t.Fatal(err)
	}
	if id != "76561198000000000" {
		t.Fatal(id)
	}
	if gotMode != "check_authentication" {
		t.Fatalf("mode %q", gotMode)
	}
}

func TestSteamOpenIDValid(t *testing.T) {
	if !SteamOpenIDValid("ns:http://specs.openid.net/auth/2.0\nis_valid:true\n") {
		t.Fatal("expected valid")
	}
	if SteamOpenIDValid("is_valid:false") {
		t.Fatal("false should fail")
	}
}

func TestOpenIDAuthURL(t *testing.T) {
	raw := OpenIDAuthURL("http://localhost:3847/auth/steam/callback", "http://localhost:3847")
	if !strings.HasPrefix(raw, OpenIDEndpoint+"?") {
		t.Fatal(raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("openid.mode") != "checkid_setup" || q.Get("openid.return_to") == "" {
		t.Fatal(q)
	}
}
