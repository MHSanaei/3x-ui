package service

import (
	"errors"
	"os"
	"strings"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/xray/geodata"
)

// GeodataTokenIssue reports a routing token the running core would reject,
// or would silently match nothing against.
type GeodataTokenIssue struct {
	Token  string `json:"token" example:"geosite:blabla"`
	Reason string `json:"reason" example:"categoryMissing"`
	File   string `json:"file,omitempty" example:"geosite.dat"`
	Code   string `json:"code,omitempty" example:"blabla"`
}

const (
	geodataReasonSyntax           = "syntax"
	geodataReasonFileMissing      = "fileMissing"
	geodataReasonCategoryMissing  = "categoryMissing"
	geodataReasonAttributeMissing = "attributeMissing"
	geodataReasonWrongKind        = "wrongKind"
)

// geodataStores keys the cache by asset directory rather than holding a single
// store, so a changed XUI_BIN_FOLDER is picked up instead of being pinned to
// whatever the first call saw.
var geodataStores sync.Map

func assetStore() *geodata.Store {
	dir := assetDir()
	if cached, ok := geodataStores.Load(dir); ok {
		return cached.(*geodata.Store)
	}
	store, _ := geodataStores.LoadOrStore(dir, geodata.NewStore(dir))
	return store.(*geodata.Store)
}

// assetDir resolves the folder the running core reads its databases from,
// with the same precedence the core itself uses (see ensureXrayAssetLocation
// in internal/xray). An install that points XRAY_LOCATION_ASSET at a shared
// asset directory would otherwise have the panel browsing an empty bin folder
// and reporting perfectly valid geosite:/geoip: tokens as missing.
func assetDir() string {
	for _, key := range [...]string{"XRAY_LOCATION_ASSET", "xray.location.asset"} {
		if dir := os.Getenv(key); dir != "" {
			return dir
		}
	}
	return config.GetBinFolderPath()
}

// GeodataService browses the geosite/geoip databases Xray resolves its
// geosite:/geoip: routing tokens against.
type GeodataService struct{}

// Files lists the databases available in the Xray asset folder.
func (s *GeodataService) Files() ([]geodata.GeoFile, error) {
	return assetStore().ListFiles()
}

// Categories returns one page of a database's categories.
func (s *GeodataService) Categories(file, query string, offset, limit int) (geodata.GeoCategoryPage, error) {
	return assetStore().Categories(file, query, offset, limit)
}

// Entries returns one page of the rules inside a category.
func (s *GeodataService) Entries(file, code, query string, offset, limit int) (geodata.GeoEntryPage, error) {
	return assetStore().Entries(file, code, query, offset, limit)
}

// Validate reports which of the given routing tokens do not resolve against the
// databases on disk. Plain domains and CIDRs are left alone — only tokens that
// name a database are looked up.
func (s *GeodataService) Validate(isIP bool, tokens []string) []GeodataTokenIssue {
	kind := geodata.KindSite
	if isIP {
		kind = geodata.KindIP
	}
	issues := make([]GeodataTokenIssue, 0)
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		reference, err := geodata.ParseReference(token, kind)
		if err != nil {
			reason := geodataReasonSyntax
			if errors.Is(err, geodata.ErrWrongKind) {
				reason = geodataReasonWrongKind
			}
			issues = append(issues, GeodataTokenIssue{Token: token, Reason: reason})
			continue
		}
		if reference.File == "" {
			continue
		}
		category, err := assetStore().Lookup(reference.File, reference.Code)
		if err != nil {
			issues = append(issues, GeodataTokenIssue{
				Token:  token,
				Reason: geodataIssueReason(err),
				File:   reference.File,
				Code:   reference.Code,
			})
			continue
		}
		if missing := unknownAttributes(category, reference.Attributes); missing != "" {
			issues = append(issues, GeodataTokenIssue{
				Token:  token,
				Reason: geodataReasonAttributeMissing,
				File:   reference.File,
				Code:   missing,
			})
		}
	}
	return issues
}

// unknownAttributes returns the first attribute the category does not carry.
// The core accepts such a token, but no domain can satisfy the filter, so the
// rule silently matches nothing — worth reporting even though Xray will start.
// A leading "!" is accepted either way: some databases ship the negated key
// verbatim, and the panel must not guess which convention a database follows.
func unknownAttributes(category geodata.GeoCategory, wanted []string) string {
	if len(wanted) == 0 {
		return ""
	}
	present := make(map[string]struct{}, len(category.Attributes))
	for _, attribute := range category.Attributes {
		present[attribute] = struct{}{}
		present[strings.TrimPrefix(attribute, "!")] = struct{}{}
	}
	for _, attribute := range wanted {
		if _, ok := present[attribute]; ok {
			continue
		}
		if _, ok := present[strings.TrimPrefix(attribute, "!")]; ok {
			continue
		}
		return attribute
	}
	return ""
}

func geodataIssueReason(err error) string {
	if errors.Is(err, geodata.ErrUnknownCategory) {
		return geodataReasonCategoryMissing
	}
	return geodataReasonFileMissing
}
