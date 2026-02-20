package service

import (
	"fmt"
	"strings"
)

// SubscriptionURLInput contains all required inputs for URL generation.
type SubscriptionURLInput struct {
	SubID string

	ConfiguredSubURI     string
	ConfiguredSubJSONURI string

	SubDomain   string
	SubPort     int
	SubPath     string
	SubJSONPath string

	SubKeyFile  string
	SubCertFile string

	RequestScheme       string
	RequestHostWithPort string

	JSONEnabled bool
}

func normalizeSubscriptionPath(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}

func normalizeConfiguredURI(uri string) string {
	if uri == "" {
		return ""
	}
	if strings.HasSuffix(uri, "/") {
		return uri
	}
	return uri + "/"
}

func resolveBaseSchemeHost(in SubscriptionURLInput) (scheme string, host string) {
	if in.SubDomain != "" {
		scheme = "http"
		if in.SubKeyFile != "" && in.SubCertFile != "" {
			scheme = "https"
		}
		host = in.SubDomain
		if !((in.SubPort == 443 && scheme == "https") || (in.SubPort == 80 && scheme == "http")) {
			host = fmt.Sprintf("%s:%d", in.SubDomain, in.SubPort)
		}
		return scheme, host
	}

	scheme = in.RequestScheme
	if scheme == "" {
		scheme = "http"
	}
	host = in.RequestHostWithPort
	if host == "" {
		host = "localhost"
	}
	return scheme, host
}

// BuildSubscriptionURLs computes canonical subscription URLs used by both sub and tgbot flows.
func BuildSubscriptionURLs(in SubscriptionURLInput) (subURL string, subJSONURL string, err error) {
	if in.SubID == "" {
		return "", "", fmt.Errorf("sub id is required")
	}

	if uri := normalizeConfiguredURI(in.ConfiguredSubURI); uri != "" {
		subURL = uri + in.SubID
	} else {
		scheme, host := resolveBaseSchemeHost(in)
		subPath := normalizeSubscriptionPath(in.SubPath)
		subURL = fmt.Sprintf("%s://%s%s%s", scheme, host, subPath, in.SubID)
	}

	if !in.JSONEnabled {
		return subURL, "", nil
	}

	if uri := normalizeConfiguredURI(in.ConfiguredSubJSONURI); uri != "" {
		subJSONURL = uri + in.SubID
	} else {
		scheme, host := resolveBaseSchemeHost(in)
		subJSONPath := normalizeSubscriptionPath(in.SubJSONPath)
		subJSONURL = fmt.Sprintf("%s://%s%s%s", scheme, host, subJSONPath, in.SubID)
	}

	return subURL, subJSONURL, nil
}
