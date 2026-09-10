package steam

import (
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	OpenIDEndpoint = "https://steamcommunity.com/openid/login"
	openIDNS       = "http://specs.openid.net/auth/2.0"
	openIDSelect   = "http://specs.openid.net/auth/2.0/identifier_select"
)

var claimedIDRE = regexp.MustCompile(`(?i)^https?://steamcommunity\.com/openid/id/(\d{17})$`)

func OpenIDAuthURL(returnTo, realm string) string {
	q := url.Values{}
	q.Set("openid.ns", openIDNS)
	q.Set("openid.mode", "checkid_setup")
	q.Set("openid.return_to", returnTo)
	q.Set("openid.realm", realm)
	q.Set("openid.identity", openIDSelect)
	q.Set("openid.claimed_id", openIDSelect)
	return OpenIDEndpoint + "?" + q.Encode()
}

func ParseClaimedSteamID(claimed string) (string, error) {
	m := claimedIDRE.FindStringSubmatch(strings.TrimSpace(claimed))
	if len(m) != 2 {
		return "", Status(http.StatusUnauthorized, "Steam login did not return a SteamID.")
	}
	return m[1], nil
}

func SteamOpenIDValid(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(key), "is_valid") {
			return strings.EqualFold(strings.TrimSpace(value), "true")
		}
	}
	return false
}

func ValidateOpenID(params url.Values, expectedReturnTo string, httpClient *http.Client) (string, error) {
	if params.Get("openid.mode") != "id_res" {
		return "", Status(http.StatusUnauthorized, "Steam login was cancelled.")
	}
	if params.Get("openid.ns") != openIDNS {
		return "", Status(http.StatusUnauthorized, "Steam login used an unexpected protocol.")
	}
	if params.Get("openid.return_to") != expectedReturnTo {
		return "", Status(http.StatusUnauthorized, "Steam login return address did not match.")
	}
	if params.Get("openid.op_endpoint") != OpenIDEndpoint {
		return "", Status(http.StatusUnauthorized, "Steam login came from an unexpected provider.")
	}
	claimed := params.Get("openid.claimed_id")
	steamid, err := ParseClaimedSteamID(claimed)
	if err != nil {
		return "", err
	}
	identity := params.Get("openid.identity")
	if identity != "" && identity != claimed {
		other, idErr := ParseClaimedSteamID(identity)
		if idErr != nil || other != steamid {
			return "", Status(http.StatusUnauthorized, "Steam login identity did not match.")
		}
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	form := url.Values{}
	for key, values := range params {
		if !strings.HasPrefix(key, "openid.") {
			continue
		}
		for _, value := range values {
			form.Add(key, value)
		}
	}
	form.Set("openid.mode", "check_authentication")
	req, err := http.NewRequest(http.MethodPost, OpenIDEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", UserAgent)
	res, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<10))
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK || !SteamOpenIDValid(string(body)) {
		return "", Status(http.StatusUnauthorized, "Steam could not verify this login.")
	}
	return steamid, nil
}

func OpenIDReturnTo(origin string) string {
	return strings.TrimRight(origin, "/") + "/auth/steam/callback"
}

func OpenIDRealm(origin string) string {
	return strings.TrimRight(origin, "/")
}
