package router

import (
	"encoding/gob"
	"errors"
	"io"
	"runtime"

	"github.com/xtls/xray-core/common/strmatcher"
)

type geoSiteListGob struct {
	Sites map[string][]byte
	Deps  map[string][]string
	Hosts map[string][]string
}

func SerializeGeoSiteList(sites []*GeoSite, deps map[string][]string, hosts map[string][]string, w io.Writer) error {
	data := geoSiteListGob{
		Sites: make(map[string][]byte),
		Deps:  deps,
		Hosts: hosts,
	}

	for _, site := range sites {
		if site == nil {
			continue
		}
		var buf bytesWriter
		if err := SerializeDomainMatcher(site.Domain, &buf); err != nil {
			return err
		}
		data.Sites[site.CountryCode] = buf.Bytes()
	}

	return gob.NewEncoder(w).Encode(data)
}

type bytesWriter struct {
	data []byte
}

func (w *bytesWriter) Write(p []byte) (n int, err error) {
	w.data = append(w.data, p...)
	return len(p), nil
}

func (w *bytesWriter) Bytes() []byte {
	return w.data
}

func LoadGeoSiteMatcher(r io.Reader, countryCode string) (strmatcher.IndexMatcher, error) {
	var data geoSiteListGob
	if err := gob.NewDecoder(r).Decode(&data); err != nil {
		return nil, err
	}

	return loadWithDeps(&data, countryCode, make(map[string]bool))
}

func loadWithDeps(data *geoSiteListGob, code string, visited map[string]bool) (strmatcher.IndexMatcher, error) {
	if visited[code] {
		return nil, errors.New("cyclic dependency")
	}
	visited[code] = true

	var matchers []strmatcher.IndexMatcher

	if siteData, ok := data.Sites[code]; ok {
		m, err := NewDomainMatcherFromBuffer(siteData)
		if err == nil {
			matchers = append(matchers, m)
		}
	}

	if deps, ok := data.Deps[code]; ok {
		for _, dep := range deps {
			m, err := loadWithDeps(data, dep, visited)
			if err == nil {
				matchers = append(matchers, m)
			}
		}
	}

	if len(matchers) == 0 {
		return nil, errors.New("matcher not found for: " + code)
	}
	if len(matchers) == 1 {
		return matchers[0], nil
	}
	runtime.GC()
	return &strmatcher.IndexMatcherGroup{Matchers: matchers}, nil
}
func LoadGeoSiteHosts(r io.Reader) (map[string][]string, error) {
	var data geoSiteListGob
	if err := gob.NewDecoder(r).Decode(&data); err != nil {
		return nil, err
	}
	return data.Hosts, nil
}
