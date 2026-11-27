package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
)

// rawClientHeader captures only hot-path fields when scanning bystander client
// entries inside inbounds.settings without materializing map[string]any.
type rawClientHeader struct {
	Email  string `json:"email"`
	Enable bool   `json:"enable"`
	Flow   string `json:"flow"`
}

// rawSettingsDoc holds an inbound settings JSON object with non-"clients" keys
// and bystander client entries preserved as raw json.RawMessage slices.
type rawSettingsDoc struct {
	top        map[string]json.RawMessage
	rawClients []json.RawMessage
	hasClients bool
}

func parseRawSettingsDoc(raw string) (*rawSettingsDoc, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("empty inbound settings")
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &top); err != nil {
		return nil, err
	}
	if top == nil {
		return nil, errors.New("invalid inbound settings object")
	}
	doc := &rawSettingsDoc{top: top}
	rawArr, ok := top["clients"]
	if !ok {
		return doc, nil
	}
	doc.hasClients = true
	trimmedArr := bytes.TrimSpace(rawArr)
	if len(trimmedArr) == 0 || bytes.Equal(trimmedArr, []byte("null")) {
		return doc, nil
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(trimmedArr, &entries); err != nil {
		return nil, err
	}
	doc.rawClients = entries
	return doc, nil
}

func (d *rawSettingsDoc) topString(key string) string {
	raw, ok := d.top[key]
	if !ok || len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

func (d *rawSettingsDoc) clientHeaderAt(idx int) rawClientHeader {
	var hdr rawClientHeader
	_ = json.Unmarshal(d.rawClients[idx], &hdr)
	return hdr
}

func (d *rawSettingsDoc) clientMapAt(idx int) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(d.rawClients[idx], &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (d *rawSettingsDoc) appendMaps(entries []map[string]any) error {
	d.hasClients = true
	for _, entry := range entries {
		b, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		d.rawClients = append(d.rawClients, json.RawMessage(b))
	}
	return nil
}

func (d *rawSettingsDoc) replaceAt(idx int, entry any) error {
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	d.rawClients[idx] = json.RawMessage(b)
	return nil
}

func (d *rawSettingsDoc) encodeCompact() (string, error) {
	if d.hasClients || len(d.rawClients) > 0 {
		totalLen := 2
		for _, r := range d.rawClients {
			totalLen += len(r) + 1
		}
		var buf bytes.Buffer
		buf.Grow(totalLen)
		buf.WriteByte('[')
		for i, r := range d.rawClients {
			if i > 0 {
				buf.WriteByte(',')
			}
			trimmed := bytes.TrimSpace(r)
			if len(trimmed) == 0 {
				buf.WriteString("null")
			} else {
				buf.Write(trimmed)
			}
		}
		buf.WriteByte(']')
		d.top["clients"] = json.RawMessage(buf.Bytes())
	}
	out, err := json.Marshal(d.top)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// applyShadowsocksMethodToClientMaps sets or strips the legacy "method" field on
// raw client maps matching applyShadowsocksClientMethod semantics.
func applyShadowsocksMethodToClientMaps(clients []map[string]any, method string) {
	is2022 := strings.HasPrefix(method, "2022-blake3-")
	for _, cm := range clients {
		if cm == nil {
			continue
		}
		if is2022 {
			delete(cm, "method")
			continue
		}
		if method == "" {
			continue
		}
		if existing, _ := cm["method"].(string); existing != "" {
			continue
		}
		cm["method"] = method
	}
}

type clientSpan struct {
	start  int
	end    int
	email  string
	enable bool
	flow   string
}

type clientReplacement struct {
	pos     int
	encoded []byte
	span    clientSpan
}

// fastSettingsIndex stores byte offsets of client entries in inbounds.settings
// so add/edit/remove operations splice JSON bytes directly without full re-parse.
type fastSettingsIndex struct {
	raw           string
	spans         []clientSpan
	byEmail       map[string]int
	dupEmails     map[string][]int
	indexedCount  int
	arrStart      int
	arrEnd        int
	clientsFound  bool
	clientsIsNull bool
	topMethod     string
	hasVlessEnc   bool
	hasTestseed   bool
	visionCnt     int
}

func isJSONSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

func skipJSONSpaces(b []byte, i int) int {
	for i < len(b) && isJSONSpace(b[i]) {
		i++
	}
	return i
}

func skipJSONString(b []byte, start int) (int, bool) {
	if start >= len(b) || b[start] != '"' {
		return 0, false
	}
	pos := start + 1
	for {
		rel := bytes.IndexByte(b[pos:], '"')
		if rel < 0 {
			return 0, false
		}
		q := pos + rel
		bs := 0
		for k := q - 1; k >= start+1 && b[k] == '\\'; k-- {
			bs++
		}
		if bs&1 == 0 {
			return q + 1, true
		}
		pos = q + 1
	}
}

func scanJSONString(b []byte, start int) (string, int, bool) {
	end, ok := skipJSONString(b, start)
	if !ok {
		return "", 0, false
	}
	if bytes.IndexByte(b[start+1:end-1], '\\') < 0 {
		return string(b[start+1 : end-1]), end, true
	}
	var s string
	if err := json.Unmarshal(b[start:end], &s); err != nil {
		return "", 0, false
	}
	return s, end, true
}

func skipJSONValue(b []byte, i int) (int, bool) {
	n := len(b)
	if i >= n {
		return 0, false
	}
	switch b[i] {
	case '"':
		return skipJSONString(b, i)
	case '{', '[':
		depth := 1
		i++
		for i < n && depth > 0 {
			switch b[i] {
			case '"':
				var ok bool
				i, ok = skipJSONString(b, i)
				if !ok {
					return 0, false
				}
			case '{', '[':
				depth++
				i++
			case '}', ']':
				depth--
				i++
			default:
				i++
			}
		}
		if depth != 0 {
			return 0, false
		}
		return i, true
	default:
		start := i
		for i < n {
			c := b[i]
			if isJSONSpace(c) || c == ',' || c == '}' || c == ']' {
				break
			}
			i++
		}
		if i == start {
			return 0, false
		}
		return i, true
	}
}

func scanClientObject(b []byte, start int) (clientSpan, int, bool) {
	var sp clientSpan
	n := len(b)
	if start >= n || b[start] != '{' {
		return sp, 0, false
	}
	i := skipJSONSpaces(b, start+1)
	if i < n && b[i] == '}' {
		sp.start = start
		sp.end = i + 1
		return sp, i + 1, true
	}
	for {
		i = skipJSONSpaces(b, i)
		if i >= n || b[i] != '"' {
			return sp, 0, false
		}
		kEnd, ok := skipJSONString(b, i)
		if !ok {
			return sp, 0, false
		}
		var keyEscaped string
		hasBackslash := bytes.IndexByte(b[i+1:kEnd-1], '\\') >= 0
		if hasBackslash {
			if err := json.Unmarshal(b[i:kEnd], &keyEscaped); err != nil {
				return sp, 0, false
			}
		}
		keyBytes := b[i+1 : kEnd-1]
		i = skipJSONSpaces(b, kEnd)
		if i >= n || b[i] != ':' {
			return sp, 0, false
		}
		i = skipJSONSpaces(b, i+1)
		if i >= n {
			return sp, 0, false
		}

		var keyName string
		if hasBackslash {
			keyName = keyEscaped
		} else {
			keyName = string(keyBytes)
		}

		switch keyName {
		case "email":
			if b[i] == '"' {
				val, nextI, ok := scanJSONString(b, i)
				if !ok {
					return sp, 0, false
				}
				sp.email = val
				i = nextI
			} else {
				nextI, ok := skipJSONValue(b, i)
				if !ok {
					return sp, 0, false
				}
				sp.email = ""
				i = nextI
			}
		case "enable":
			if bytes.HasPrefix(b[i:], []byte("true")) && (i+4 == n || isJSONSpace(b[i+4]) || b[i+4] == ',' || b[i+4] == '}') {
				sp.enable = true
				i += 4
			} else if bytes.HasPrefix(b[i:], []byte("false")) && (i+5 == n || isJSONSpace(b[i+5]) || b[i+5] == ',' || b[i+5] == '}') {
				sp.enable = false
				i += 5
			} else {
				nextI, ok := skipJSONValue(b, i)
				if !ok {
					return sp, 0, false
				}
				sp.enable = false
				i = nextI
			}
		case "flow":
			if b[i] == '"' {
				val, nextI, ok := scanJSONString(b, i)
				if !ok {
					return sp, 0, false
				}
				sp.flow = val
				i = nextI
			} else {
				nextI, ok := skipJSONValue(b, i)
				if !ok {
					return sp, 0, false
				}
				sp.flow = ""
				i = nextI
			}
		default:
			nextI, ok := skipJSONValue(b, i)
			if !ok {
				return sp, 0, false
			}
			i = nextI
		}

		i = skipJSONSpaces(b, i)
		if i < n && b[i] == ',' {
			i++
			continue
		}
		if i < n && b[i] == '}' {
			sp.start = start
			sp.end = i + 1
			return sp, i + 1, true
		}
		return sp, 0, false
	}
}

func parseFastSettingsIndex(raw string) (*fastSettingsIndex, bool) {
	b := []byte(raw)
	n := len(b)
	i := skipJSONSpaces(b, 0)
	if i >= n || b[i] != '{' {
		return nil, false
	}
	i = skipJSONSpaces(b, i+1)

	idx := &fastSettingsIndex{
		raw:      raw,
		arrStart: -1,
		arrEnd:   -1,
	}
	if i < n && b[i] == '}' {
		i = skipJSONSpaces(b, i+1)
		if i != n {
			return nil, false
		}
		return idx, true
	}

	for {
		i = skipJSONSpaces(b, i)
		if i >= n || b[i] != '"' {
			return nil, false
		}
		key, kEnd, ok := scanJSONString(b, i)
		if !ok {
			return nil, false
		}
		i = skipJSONSpaces(b, kEnd)
		if i >= n || b[i] != ':' {
			return nil, false
		}
		i = skipJSONSpaces(b, i+1)
		if i >= n {
			return nil, false
		}

		switch key {
		case "clients":
			idx.clientsFound = true
			if bytes.HasPrefix(b[i:], []byte("null")) && (i+4 == n || isJSONSpace(b[i+4]) || b[i+4] == ',' || b[i+4] == '}') {
				idx.clientsIsNull = true
				i += 4
			} else if b[i] == '[' {
				idx.arrStart = i
				i = skipJSONSpaces(b, i+1)
				if i < n && b[i] == ']' {
					idx.arrEnd = i + 1
					i++
				} else {
					for {
						i = skipJSONSpaces(b, i)
						if i >= n || b[i] != '{' {
							return nil, false
						}
						sp, nextI, ok := scanClientObject(b, i)
						if !ok {
							return nil, false
						}
						pos := len(idx.spans)
						idx.spans = append(idx.spans, sp)
						if sp.flow == "xtls-rprx-vision" {
							idx.visionCnt++
						}
						if sp.email != "" {
							if idx.byEmail == nil {
								idx.byEmail = make(map[string]int, 16)
							}
							recordSpanEmail(idx.byEmail, &idx.dupEmails, sp.email, pos)
						}
						i = skipJSONSpaces(b, nextI)
						if i < n && b[i] == ',' {
							i++
							continue
						}
						if i < n && b[i] == ']' {
							idx.arrEnd = i + 1
							i++
							break
						}
						return nil, false
					}
				}
			} else {
				return nil, false
			}
		case "method":
			if b[i] == '"' {
				val, nextI, ok := scanJSONString(b, i)
				if !ok {
					return nil, false
				}
				idx.topMethod = val
				i = nextI
			} else {
				nextI, ok := skipJSONValue(b, i)
				if !ok {
					return nil, false
				}
				i = nextI
			}
		case "encryption", "decryption":
			if b[i] == '"' {
				val, nextI, ok := scanJSONString(b, i)
				if !ok {
					return nil, false
				}
				if vlessEncValueSet(val) {
					idx.hasVlessEnc = true
				}
				i = nextI
			} else {
				nextI, ok := skipJSONValue(b, i)
				if !ok {
					return nil, false
				}
				i = nextI
			}
		case "testseed":
			idx.hasTestseed = true
			nextI, ok := skipJSONValue(b, i)
			if !ok {
				return nil, false
			}
			i = nextI
		default:
			nextI, ok := skipJSONValue(b, i)
			if !ok {
				return nil, false
			}
			i = nextI
		}

		i = skipJSONSpaces(b, i)
		if i < n && b[i] == ',' {
			i++
			continue
		}
		if i < n && b[i] == '}' {
			i = skipJSONSpaces(b, i+1)
			if i != n {
				return nil, false
			}
			idx.indexedCount = len(idx.spans)
			return idx, true
		}
		return nil, false
	}
}

// scanTopLevelStringFields extracts up to two top-level string values without
// materializing the clients array. Returns ok=false if input is malformed JSON.
func scanTopLevelStringFields(raw string, want1, want2 string) (val1, val2 string, ok bool) {
	b := []byte(raw)
	n := len(b)
	i := skipJSONSpaces(b, 0)
	if i >= n || b[i] != '{' {
		return "", "", false
	}
	i = skipJSONSpaces(b, i+1)
	if i < n && b[i] == '}' {
		return "", "", skipJSONSpaces(b, i+1) == n
	}
	for {
		i = skipJSONSpaces(b, i)
		if i >= n || b[i] != '"' {
			return "", "", false
		}
		key, kEnd, kOK := scanJSONString(b, i)
		if !kOK {
			return "", "", false
		}
		i = skipJSONSpaces(b, kEnd)
		if i >= n || b[i] != ':' {
			return "", "", false
		}
		i = skipJSONSpaces(b, i+1)
		if i >= n {
			return "", "", false
		}
		if key == want1 || (want2 != "" && key == want2) {
			if b[i] != '"' {
				return "", "", false
			}
			v, vEnd, vOK := scanJSONString(b, i)
			if !vOK {
				return "", "", false
			}
			if key == want1 {
				val1 = v
			} else {
				val2 = v
			}
			i = vEnd
		} else {
			vEnd, vOK := skipJSONValue(b, i)
			if !vOK {
				return "", "", false
			}
			i = vEnd
		}
		i = skipJSONSpaces(b, i)
		if i < n && b[i] == ',' {
			i++
			continue
		}
		if i < n && b[i] == '}' {
			return val1, val2, skipJSONSpaces(b, i+1) == n
		}
		return "", "", false
	}
}

func (idx *fastSettingsIndex) canSplice() bool {
	return idx != nil && idx.clientsFound && !idx.clientsIsNull && idx.arrStart >= 0 && idx.arrEnd > idx.arrStart
}

func (idx *fastSettingsIndex) findFoldIndex(email string) (int, bool) {
	if idx == nil || email == "" {
		return -1, false
	}
	if idx.byEmail != nil {
		if pos, ok := idx.byEmail[strings.ToLower(email)]; ok {
			return pos, true
		}
	}
	for p := idx.indexedCount; p < len(idx.spans); p++ {
		if strings.EqualFold(idx.spans[p].email, email) {
			return p, true
		}
	}
	return -1, false
}

func (idx *fastSettingsIndex) hasFoldEmail(email string) bool {
	_, ok := idx.findFoldIndex(email)
	return ok
}

func (idx *fastSettingsIndex) findExactPositions(email string) []int {
	if idx == nil || email == "" {
		return nil
	}
	var out []int
	if idx.byEmail != nil {
		lower := strings.ToLower(email)
		if first, ok := idx.byEmail[lower]; ok {
			if idx.dupEmails != nil {
				if positions, hasDups := idx.dupEmails[lower]; hasDups {
					for _, p := range positions {
						if idx.spans[p].email == email {
							out = append(out, p)
						}
					}
				} else if idx.spans[first].email == email {
					out = append(out, first)
				}
			} else if idx.spans[first].email == email {
				out = append(out, first)
			}
		}
	}
	for p := idx.indexedCount; p < len(idx.spans); p++ {
		if idx.spans[p].email == email {
			out = append(out, p)
		}
	}
	return out
}

func recordSpanEmail(byEmail map[string]int, dupEmails *map[string][]int, email string, pos int) {
	if email == "" {
		return
	}
	lower := strings.ToLower(email)
	if prev, exists := byEmail[lower]; exists {
		if *dupEmails == nil {
			*dupEmails = make(map[string][]int)
		}
		if _, has := (*dupEmails)[lower]; !has {
			(*dupEmails)[lower] = []int{prev}
		}
		(*dupEmails)[lower] = append((*dupEmails)[lower], pos)
	} else {
		byEmail[lower] = pos
	}
}

func rebuildEmailIndex(spans []clientSpan) (map[string]int, map[string][]int) {
	var by map[string]int
	var dup map[string][]int
	if len(spans) > 0 {
		by = make(map[string]int, len(spans))
	}
	for i := range spans {
		if spans[i].email != "" {
			recordSpanEmail(by, &dup, spans[i].email, i)
		}
	}
	return by, dup
}

func (idx *fastSettingsIndex) spliceAppend(encodedClients [][]byte, addedSpans []clientSpan) (string, *fastSettingsIndex) {
	if len(encodedClients) == 0 {
		return idx.raw, idx
	}
	extraBytes := len(encodedClients)
	for _, b := range encodedClients {
		extraBytes += len(b)
	}
	var sb strings.Builder
	sb.Grow(len(idx.raw) + extraBytes)

	var tailStart int
	if len(idx.spans) == 0 {
		sb.WriteString(idx.raw[:idx.arrStart+1])
		tailStart = idx.arrEnd - 1
	} else {
		lastEnd := idx.spans[len(idx.spans)-1].end
		sb.WriteString(idx.raw[:lastEnd])
		tailStart = lastEnd
	}

	newSpans := make([]clientSpan, len(idx.spans), len(idx.spans)+len(addedSpans))
	copy(newSpans, idx.spans)
	newVision := idx.visionCnt

	for i, enc := range encodedClients {
		if len(idx.spans) > 0 || i > 0 {
			sb.WriteByte(',')
		}
		sp := addedSpans[i]
		sp.start = sb.Len()
		sb.Write(enc)
		sp.end = sb.Len()
		newSpans = append(newSpans, sp)
		if sp.flow == "xtls-rprx-vision" {
			newVision++
		}
	}
	sb.WriteString(idx.raw[tailStart:])
	newRaw := sb.String()

	const maxUnindexedTail = 32
	newBy := idx.byEmail
	newDup := idx.dupEmails
	newIndexed := idx.indexedCount
	if len(newSpans)-newIndexed >= maxUnindexedTail {
		newBy, newDup = rebuildEmailIndex(newSpans)
		newIndexed = len(newSpans)
	}

	newIdx := &fastSettingsIndex{
		raw:           newRaw,
		spans:         newSpans,
		byEmail:       newBy,
		dupEmails:     newDup,
		indexedCount:  newIndexed,
		arrStart:      idx.arrStart,
		arrEnd:        idx.arrEnd + (len(newRaw) - len(idx.raw)),
		clientsFound:  true,
		clientsIsNull: false,
		topMethod:     idx.topMethod,
		hasVlessEnc:   idx.hasVlessEnc,
		hasTestseed:   idx.hasTestseed,
		visionCnt:     newVision,
	}
	return newRaw, newIdx
}

func (idx *fastSettingsIndex) spliceReplaceOne(pos int, encodedClient []byte, newSpan clientSpan, dropTestseedIfNoVision bool) (string, *fastSettingsIndex, bool) {
	oldSp := idx.spans[pos]
	newVision := idx.visionCnt
	if oldSp.flow == "xtls-rprx-vision" {
		newVision--
	}
	if newSpan.flow == "xtls-rprx-vision" {
		newVision++
	}
	if dropTestseedIfNoVision && idx.hasTestseed && newVision == 0 {
		return "", nil, false
	}

	delta := len(encodedClient) - (oldSp.end - oldSp.start)
	var sb strings.Builder
	sb.Grow(len(idx.raw) + delta)
	sb.WriteString(idx.raw[:oldSp.start])
	sb.Write(encodedClient)
	sb.WriteString(idx.raw[oldSp.end:])
	newRaw := sb.String()

	newSpans := make([]clientSpan, len(idx.spans))
	copy(newSpans, idx.spans)
	newSpan.start = oldSp.start
	newSpan.end = oldSp.start + len(encodedClient)
	newSpans[pos] = newSpan
	if delta != 0 {
		for k := pos + 1; k < len(newSpans); k++ {
			newSpans[k].start += delta
			newSpans[k].end += delta
		}
	}

	newBy := idx.byEmail
	newDup := idx.dupEmails
	newIndexed := idx.indexedCount
	if pos < newIndexed && !strings.EqualFold(oldSp.email, newSpan.email) {
		newBy, newDup = rebuildEmailIndex(newSpans)
		newIndexed = len(newSpans)
	}

	newIdx := &fastSettingsIndex{
		raw:           newRaw,
		spans:         newSpans,
		byEmail:       newBy,
		dupEmails:     newDup,
		indexedCount:  newIndexed,
		arrStart:      idx.arrStart,
		arrEnd:        idx.arrEnd + delta,
		clientsFound:  true,
		clientsIsNull: false,
		topMethod:     idx.topMethod,
		hasVlessEnc:   idx.hasVlessEnc,
		hasTestseed:   idx.hasTestseed,
		visionCnt:     newVision,
	}
	return newRaw, newIdx, true
}

func (idx *fastSettingsIndex) spliceReplaceMany(replacements []clientReplacement, dropTestseedIfNoVision bool) (string, *fastSettingsIndex, bool) {
	if len(replacements) == 0 {
		return idx.raw, idx, true
	}
	if len(replacements) == 1 {
		r := replacements[0]
		return idx.spliceReplaceOne(r.pos, r.encoded, r.span, dropTestseedIfNoVision)
	}
	sorted := make([]clientReplacement, len(replacements))
	copy(sorted, replacements)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].pos < sorted[j].pos })

	newVision := idx.visionCnt
	emailChanged := false
	estDelta := 0
	for _, r := range sorted {
		oldSp := idx.spans[r.pos]
		if oldSp.flow == "xtls-rprx-vision" {
			newVision--
		}
		if r.span.flow == "xtls-rprx-vision" {
			newVision++
		}
		if r.pos < idx.indexedCount && !strings.EqualFold(oldSp.email, r.span.email) {
			emailChanged = true
		}
		estDelta += len(r.encoded) - (oldSp.end - oldSp.start)
	}
	if dropTestseedIfNoVision && idx.hasTestseed && newVision == 0 {
		return "", nil, false
	}

	var sb strings.Builder
	sb.Grow(len(idx.raw) + estDelta)
	newSpans := make([]clientSpan, len(idx.spans))
	copy(newSpans, idx.spans)

	cursor := 0
	totalDelta := 0
	nextSpanIdx := 0
	for _, r := range sorted {
		for k := nextSpanIdx; k < r.pos; k++ {
			newSpans[k].start += totalDelta
			newSpans[k].end += totalDelta
		}
		oldSp := idx.spans[r.pos]
		sb.WriteString(idx.raw[cursor:oldSp.start])
		newStart := sb.Len()
		sb.Write(r.encoded)
		newEnd := sb.Len()
		sp := r.span
		sp.start = newStart
		sp.end = newEnd
		newSpans[r.pos] = sp
		totalDelta += len(r.encoded) - (oldSp.end - oldSp.start)
		cursor = oldSp.end
		nextSpanIdx = r.pos + 1
	}
	for k := nextSpanIdx; k < len(newSpans); k++ {
		newSpans[k].start += totalDelta
		newSpans[k].end += totalDelta
	}
	sb.WriteString(idx.raw[cursor:])
	newRaw := sb.String()

	newBy := idx.byEmail
	newDup := idx.dupEmails
	newIndexed := idx.indexedCount
	if emailChanged {
		newBy, newDup = rebuildEmailIndex(newSpans)
		newIndexed = len(newSpans)
	}

	newIdx := &fastSettingsIndex{
		raw:           newRaw,
		spans:         newSpans,
		byEmail:       newBy,
		dupEmails:     newDup,
		indexedCount:  newIndexed,
		arrStart:      idx.arrStart,
		arrEnd:        idx.arrEnd + totalDelta,
		clientsFound:  true,
		clientsIsNull: false,
		topMethod:     idx.topMethod,
		hasVlessEnc:   idx.hasVlessEnc,
		hasTestseed:   idx.hasTestseed,
		visionCnt:     newVision,
	}
	return newRaw, newIdx, true
}

func (idx *fastSettingsIndex) spliceRemove(removePositions map[int]struct{}) (string, *fastSettingsIndex) {
	if len(removePositions) == 0 {
		return idx.raw, idx
	}
	var sb strings.Builder
	sb.Grow(len(idx.raw))
	sb.WriteString(idx.raw[:idx.arrStart+1])

	surviving := make([]clientSpan, 0, len(idx.spans)-len(removePositions))
	var newBy map[string]int
	if len(idx.spans) > len(removePositions) {
		newBy = make(map[string]int, len(idx.spans)-len(removePositions))
	}
	var newDup map[string][]int
	newVision := 0

	for i, sp := range idx.spans {
		if _, drop := removePositions[i]; drop {
			continue
		}
		if len(surviving) > 0 {
			sb.WriteByte(',')
		}
		newStart := sb.Len()
		sb.WriteString(idx.raw[sp.start:sp.end])
		newEnd := sb.Len()
		newSp := clientSpan{
			start:  newStart,
			end:    newEnd,
			email:  sp.email,
			enable: sp.enable,
			flow:   sp.flow,
		}
		pos := len(surviving)
		surviving = append(surviving, newSp)
		if newSp.email != "" {
			recordSpanEmail(newBy, &newDup, newSp.email, pos)
		}
		if newSp.flow == "xtls-rprx-vision" {
			newVision++
		}
	}
	sb.WriteString(idx.raw[idx.arrEnd-1:])
	newRaw := sb.String()
	newIdx := &fastSettingsIndex{
		raw:           newRaw,
		spans:         surviving,
		byEmail:       newBy,
		dupEmails:     newDup,
		indexedCount:  len(surviving),
		arrStart:      idx.arrStart,
		arrEnd:        idx.arrEnd + (len(newRaw) - len(idx.raw)),
		clientsFound:  true,
		clientsIsNull: false,
		topMethod:     idx.topMethod,
		hasVlessEnc:   idx.hasVlessEnc,
		hasTestseed:   idx.hasTestseed,
		visionCnt:     newVision,
	}
	return newRaw, newIdx
}

type ssClientWire struct {
	model.Client
	Method string `json:"method,omitempty"`
}

func marshalClientWire(c model.Client, ssMethod string, defaultNowMs int64) ([]byte, clientSpan, error) {
	c.AllowedIPsByInbound = nil
	if c.ForwardedPortsSpec() == "" {
		c.ForwardedPorts = nil
	}
	if c.CreatedAt == 0 {
		c.CreatedAt = defaultNowMs
	}
	if c.UpdatedAt == 0 {
		c.UpdatedAt = defaultNowMs
	}
	if strings.TrimSpace(c.SubID) == "" {
		c.SubID = random.NumLower(16)
	}
	legacySSMethod := ""
	if ssMethod != "" && !strings.HasPrefix(ssMethod, "2022-blake3-") {
		legacySSMethod = ssMethod
	}
	b, err := json.Marshal(ssClientWire{Client: c, Method: legacySSMethod})
	if err != nil {
		return nil, clientSpan{}, err
	}
	sp := clientSpan{
		email:  c.Email,
		enable: c.Enable,
		flow:   c.Flow,
	}
	return b, sp, nil
}

// patchClientSlice unmarshals a single client JSON slice, runs mutate(entry),
// and only sets updated_at and re-marshals when mutate reports a change.
func patchClientSlice(rawSlice string, nowMs int64, mutate func(entry map[string]any) (changed bool, err error)) ([]byte, model.Client, model.Client, clientSpan, bool, error) {
	var oldClient model.Client
	if err := json.Unmarshal([]byte(rawSlice), &oldClient); err != nil {
		return nil, model.Client{}, model.Client{}, clientSpan{}, false, err
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(rawSlice), &entry); err != nil {
		return nil, model.Client{}, model.Client{}, clientSpan{}, false, err
	}
	changed, err := mutate(entry)
	if err != nil {
		return nil, model.Client{}, model.Client{}, clientSpan{}, false, err
	}
	if !changed {
		return nil, oldClient, oldClient, clientSpan{
			email:  oldClient.Email,
			enable: oldClient.Enable,
			flow:   oldClient.Flow,
		}, false, nil
	}
	entry["updated_at"] = nowMs
	encoded, err := json.Marshal(entry)
	if err != nil {
		return nil, model.Client{}, model.Client{}, clientSpan{}, false, err
	}
	var newClient model.Client
	if err := json.Unmarshal(encoded, &newClient); err != nil {
		return nil, model.Client{}, model.Client{}, clientSpan{}, false, err
	}
	return encoded, oldClient, newClient, clientSpan{
		email:  newClient.Email,
		enable: newClient.Enable,
		flow:   newClient.Flow,
	}, true, nil
}

func validateClientRequiredCredentials(protocol model.Protocol, client model.Client) error {
	if strings.TrimSpace(client.Email) == "" {
		return common.NewError("client email is required")
	}
	switch protocol {
	case model.Trojan, model.MASQUE:
		if client.Password == "" {
			return common.NewError("empty client ID")
		}
	case model.Shadowsocks:
		if client.Email == "" {
			return common.NewError("empty client ID")
		}
	case model.Hysteria:
		if client.Auth == "" {
			return common.NewError("empty client ID")
		}
	case model.WireGuard, model.AmneziaWG:
		if client.PublicKey == "" {
			return common.NewError("wireguard client requires a key")
		}
	case model.MTProto:
		if client.Secret == "" {
			return common.NewError("mtproto client requires a secret")
		}
		if client.AdTag != "" && !model.ValidMtprotoAdTag(client.AdTag) {
			return common.NewError("mtproto client ad tag must be 32 hex characters")
		}
	case model.TUIC:
		if client.ID == "" {
			return common.NewError("empty client ID")
		}
		if client.Password == "" {
			return common.NewError("tuic client requires a password")
		}
	default:
		if client.ID == "" {
			return common.NewError("empty client ID")
		}
	}
	return nil
}

func clientWithInboundFlowIdx(c model.Client, ib *model.Inbound, idx *fastSettingsIndex) model.Client {
	if ib.DisableFlow || !inboundCanEnableTlsFlowIdx(string(ib.Protocol), ib.StreamSettings, ib.Settings, idx) {
		c.Flow = ""
	}
	return c
}
