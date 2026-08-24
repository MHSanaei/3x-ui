package geodata

import "strings"

// Categories returns the database's categories, filtered by a case-insensitive
// substring of the category code. A non-positive limit returns all of them:
// the category index is small even for the largest databases, and the panel
// filters it client-side so typing in the search box costs no requests.
func (s *Store) Categories(name, query string, offset, limit int) (GeoCategoryPage, error) {
	idx, err := s.index(name)
	if err != nil {
		return GeoCategoryPage{}, err
	}
	matched := idx.categories
	if query = strings.ToLower(strings.TrimSpace(query)); query != "" {
		matched = make([]GeoCategory, 0, len(idx.categories))
		for _, category := range idx.categories {
			if strings.Contains(category.Code, query) {
				matched = append(matched, category)
			}
		}
	}
	page := GeoCategoryPage{Total: len(matched), Items: []GeoCategory{}}
	from, to := categoryBounds(len(matched), offset, limit)
	page.Items = append(page.Items, matched[from:to]...)
	return page, nil
}

// Entries returns one page of a category's rules, filtered by a
// case-insensitive substring of the rule value. The page is scanned out of the
// file on each call: a category such as geosite's category-ads-all holds well
// over a hundred thousand rules, and holding those in memory to serve one
// screenful of them is what the panel cannot afford.
func (s *Store) Entries(name, code, query string, offset, limit int) (GeoEntryPage, error) {
	idx, err := s.index(name)
	if err != nil {
		return GeoEntryPage{}, err
	}
	code = strings.ToLower(strings.TrimSpace(code))
	if _, ok := idx.byCode[code]; !ok {
		return GeoEntryPage{}, ErrUnknownCategory
	}
	if _, err := s.resolve(name); err != nil {
		return GeoEntryPage{}, err
	}
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > MaxPageSize {
		limit = MaxPageSize
	}

	spans := idx.spans[code]
	if len(spans) == 0 {
		return GeoEntryPage{}, ErrUnknownCategory
	}

	s.scan.Lock()
	defer s.scan.Unlock()
	records, err := s.recordsLocked(name, code, spans)
	if err != nil {
		return GeoEntryPage{}, err
	}
	return scanEntries(records, idx.kind, code, strings.ToLower(strings.TrimSpace(query)), offset, limit)
}

// Lookup reports whether a category exists in the database, without paying for
// the entry data. The code is matched verbatim apart from case: this backs the
// routing-token validator, and the core does not forgive a stray space either,
// so trimming one here would hide the very typo the validator exists to find.
func (s *Store) Lookup(name, code string) (GeoCategory, error) {
	idx, err := s.index(name)
	if err != nil {
		return GeoCategory{}, err
	}
	category, ok := idx.byCode[strings.ToLower(code)]
	if !ok {
		return GeoCategory{}, ErrUnknownCategory
	}
	return category, nil
}

func categoryBounds(total, offset, limit int) (int, int) {
	if limit <= 0 {
		if offset < 0 {
			offset = 0
		}
		if offset > total {
			offset = total
		}
		return offset, total
	}
	return sliceBounds(total, offset, limit)
}

func sliceBounds(total, offset, limit int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	if limit <= 0 || limit > MaxPageSize {
		limit = MaxPageSize
	}
	to := min(offset+limit, total)
	return offset, to
}

func (s *Store) recordsLocked(name, code string, spans []byteSpan) ([][]byte, error) {
	info, err := s.resolve(name)
	if err != nil {
		return nil, err
	}
	key := fileKey{name: name, size: info.Size(), modTime: info.ModTime().UnixNano()}
	if s.hot.key == key && s.hot.code == code {
		return s.hot.records, nil
	}
	records, err := readSpans(s.dir, name, spans)
	if err != nil {
		return nil, err
	}
	s.hot = hotRecord{key: key, code: code, records: records}
	return records, nil
}
