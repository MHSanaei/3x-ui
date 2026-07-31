package geodata

import (
	"errors"
	"strings"

	xraygeodata "github.com/xtls/xray-core/common/geodata"
)

// DefaultSiteFile and DefaultIPFile are the databases the geosite: and geoip:
// shorthands expand to.
const (
	DefaultSiteFile = xraygeodata.DefaultGeoSiteDat
	DefaultIPFile   = xraygeodata.DefaultGeoIPDat
)

// ErrInvalidToken reports a routing token that names a database but does not
// resolve to a file and category.
var ErrInvalidToken = errors.New("invalid geodata routing token")

var (
	sitePrefixes = []string{"ext:", "ext-domain:", "ext-site:"}
	ipPrefixes   = []string{"ext:", "ext-ip:"}
)

// Reference is the database file and category a routing token points at.
// An empty File means a plain domain or CIDR, which needs no database.
type Reference struct {
	File       string
	Code       string
	Attributes []string
	Reverse    bool
}

// ParseReference resolves a routing token the way xray-core does, expanding the
// geosite:/geoip: shorthands to their ext: form. The core's own parser is not
// reusable here: it opens the database from disk as part of parsing, which
// would both duplicate this package's cache and fail whenever the file is
// merely absent — exactly the case the panel needs to report.
func ParseReference(token string, kind GeoKind) (Reference, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Reference{}, ErrInvalidToken
	}

	var reference Reference
	if kind == KindIP {
		// The core strips one "!" before the prefix and another before the code,
		// each flipping the match, so "!!geoip:cn" is an ordinary geoip:cn.
		token, reference.Reverse = cutNegation(token)
	}

	shorthand, defaultFile, prefixes := "geosite:", DefaultSiteFile, sitePrefixes
	if kind == KindIP {
		shorthand, defaultFile, prefixes = "geoip:", DefaultIPFile, ipPrefixes
	}
	if rest, found := strings.CutPrefix(token, shorthand); found {
		token = "ext:" + defaultFile + ":" + rest
	}

	rest, matched := "", false
	for _, prefix := range prefixes {
		if trimmed, found := strings.CutPrefix(token, prefix); found {
			rest, matched = trimmed, true
			break
		}
	}
	if !matched {
		return Reference{}, nil
	}
	if rest == "" {
		return Reference{}, ErrInvalidToken
	}

	file, code, found := strings.Cut(rest, ":")
	if !found || file == "" {
		return Reference{}, ErrInvalidToken
	}
	if kind == KindIP {
		var negated bool
		code, negated = cutNegation(code)
		reference.Reverse = reference.Reverse != negated
	}

	reference.File = file
	// Attribute filters exist for domain rules only; in an ip rule the core
	// treats "@" as part of the category code and fails to resolve it, so the
	// panel must not quietly strip it either.
	// Whitespace inside the token is significant: the core matches the code and
	// the attributes verbatim, so "geosite: cn" and "geosite:cn@ ads" are its
	// problems to report, not ours to silently repair. Only the case is folded,
	// which the core does too.
	if kind == KindSite {
		parts := strings.Split(code, "@")
		code = parts[0]
		for _, attribute := range parts[1:] {
			// The core rejects an empty attribute outright ("geosite:cn@"),
			// so accepting it here would hide a config it will not start with.
			if attribute == "" {
				return Reference{}, ErrInvalidToken
			}
			reference.Attributes = append(reference.Attributes, strings.ToLower(attribute))
		}
	}
	reference.Code = strings.ToLower(code)
	if reference.Code == "" {
		return Reference{}, ErrInvalidToken
	}
	return reference, nil
}

// cutNegation strips leading "!" markers, reporting whether an odd number of
// them was present — the core folds a double negation back into a plain match.
func cutNegation(value string) (string, bool) {
	negated := false
	for {
		rest, found := strings.CutPrefix(value, "!")
		if !found {
			return value, negated
		}
		value = rest
		negated = !negated
	}
}
