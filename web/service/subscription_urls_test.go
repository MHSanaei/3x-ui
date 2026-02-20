package service

import "testing"

func TestBuildSubscriptionURLsConfiguredURIs(t *testing.T) {
	sub, subJSON, err := BuildSubscriptionURLs(SubscriptionURLInput{
		SubID:                "abc123",
		ConfiguredSubURI:     "https://sub.example.com/s/",
		ConfiguredSubJSONURI: "https://sub.example.com/j",
		JSONEnabled:          true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub != "https://sub.example.com/s/abc123" {
		t.Fatalf("unexpected sub url: %s", sub)
	}
	if subJSON != "https://sub.example.com/j/abc123" {
		t.Fatalf("unexpected sub json url: %s", subJSON)
	}
}

func TestBuildSubscriptionURLsDerivedFromSubDomain(t *testing.T) {
	sub, subJSON, err := BuildSubscriptionURLs(SubscriptionURLInput{
		SubID:       "sid",
		SubDomain:   "sub.example.com",
		SubPort:     443,
		SubPath:     "sub",
		SubJSONPath: "/json/",
		SubKeyFile:  "/tmp/key.pem",
		SubCertFile: "/tmp/cert.pem",
		JSONEnabled: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub != "https://sub.example.com/sub/sid" {
		t.Fatalf("unexpected sub url: %s", sub)
	}
	if subJSON != "https://sub.example.com/json/sid" {
		t.Fatalf("unexpected sub json url: %s", subJSON)
	}
}

func TestBuildSubscriptionURLsFallsBackToRequestHost(t *testing.T) {
	sub, subJSON, err := BuildSubscriptionURLs(SubscriptionURLInput{
		SubID:               "sid",
		RequestScheme:       "https",
		RequestHostWithPort: "panel.example.com:8443",
		SubPath:             "/sub/",
		SubJSONPath:         "/json/",
		JSONEnabled:         false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub != "https://panel.example.com:8443/sub/sid" {
		t.Fatalf("unexpected sub url: %s", sub)
	}
	if subJSON != "" {
		t.Fatalf("expected empty json url when disabled, got: %s", subJSON)
	}
}
