package web

import (
	"strings"
	"testing"
)

func TestValidWebsiteID(t *testing.T) {
	if !validWebsiteID("a1b2c3d4-e5f6-7890-abcd-ef1234567890") {
		t.Fatal("uuid-like id should pass")
	}
	if validWebsiteID("") || validWebsiteID("bad id") || validWebsiteID("<script>") {
		t.Fatal("invalid ids must fail")
	}
}

func TestValidAnalyticsURL(t *testing.T) {
	if !validAnalyticsURL("https://cloud.umami.is/script.js") {
		t.Fatal("cloud url")
	}
	if !validAnalyticsURL("https://stats.example.com/umami.js") {
		t.Fatal("self-host url")
	}
	if validAnalyticsURL("http://insecure.example/script.js") || validAnalyticsURL("javascript:alert(1)") {
		t.Fatal("bad urls must fail")
	}
}

func TestContentSecurityPolicyIncludesUmami(t *testing.T) {
	t.Setenv("UMAMI_WEBSITE_ID", "d3d7edbb-3eca-4831-ad97-866f1e3dfca8")
	t.Setenv("UMAMI_SCRIPT_URL", "https://analytics.invoke.systems/recorder.js")
	t.Setenv("UMAMI_SAMPLE_RATE", "0.15")
	t.Setenv("UMAMI_MASK_LEVEL", "moderate")
	t.Setenv("UMAMI_MAX_DURATION", "300000")
	csp := contentSecurityPolicy(htmlHeaderOpts{})
	if !strings.Contains(csp, "https://analytics.invoke.systems") {
		t.Fatal(csp)
	}
	snippet := string(analyticsHTML())
	want := []string{
		`src="https://analytics.invoke.systems/recorder.js"`,
		`data-website-id="d3d7edbb-3eca-4831-ad97-866f1e3dfca8"`,
		`data-sample-rate="0.15"`,
		`data-mask-level="moderate"`,
		`data-max-duration="300000"`,
		`defer`,
	}
	for _, part := range want {
		if !strings.Contains(snippet, part) {
			t.Fatalf("missing %q in %s", part, snippet)
		}
	}
}

func TestAnalyticsDisabledWithoutID(t *testing.T) {
	t.Setenv("UMAMI_WEBSITE_ID", "")
	t.Setenv("UMAMI_SCRIPT_URL", "https://cloud.umami.is/script.js")
	if analyticsHTML() != "" {
		t.Fatal("expected empty snippet")
	}
	if strings.Contains(contentSecurityPolicy(htmlHeaderOpts{}), "umami.is") {
		t.Fatal("csp should omit umami when disabled")
	}
}
