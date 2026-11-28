package sub

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

// loonSubscription keeps existing URI entries and renders eligible REALITY TCP nodes in Loon's native format.
func loonSubscription(links []string) string {
	var out strings.Builder
	out.WriteString("[Proxy]\n")
	usedNames := make(map[string]bool)
	for _, linksForClient := range links {
		for line := range strings.SplitSeq(linksForClient, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			name, node, ok := loonRealityNode(line)
			if ok {
				base := name
				for suffix := 2; usedNames[name]; suffix++ {
					name = base + " (" + strconv.Itoa(suffix) + ")"
				}
				usedNames[name] = true
				line = name + " = " + node
			}
			out.WriteString(line)
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func loonRealityNode(raw string) (name, node string, ok bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "vless" || u.User == nil {
		return "", "", false
	}
	q := u.Query()
	switch q.Get("type") {
	case "", "raw", "tcp":
	default:
		return "", "", false
	}
	if q.Get("security") != "reality" || q.Get("support-x25519mlkem768") != "true" ||
		q.Get("pqv") != "" || (q.Get("encryption") != "" && q.Get("encryption") != "none") ||
		(q.Get("headerType") != "" && q.Get("headerType") != "none") {
		return "", "", false
	}
	host, key, serverName, shortID, flow := u.Hostname(), q.Get("pbk"), q.Get("sni"), q.Get("sid"), q.Get("flow")
	port, portErr := strconv.ParseUint(u.Port(), 10, 16)
	id, idErr := uuid.Parse(u.User.Username())
	if portErr != nil || port == 0 || idErr != nil || host == "" || key == "" ||
		(flow != "" && flow != "xtls-rprx-vision" && flow != "xtls-rprx-vision-udp443") {
		return "", "", false
	}
	for _, value := range []string{host, key, serverName, shortID} {
		if strings.ContainsAny(value, ",=\"\\[]#") || strings.ContainsFunc(value, func(r rune) bool {
			return unicode.IsControl(r) || unicode.IsSpace(r)
		}) {
			return "", "", false
		}
	}
	name = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune("=,[]#\"", r) {
			return ' '
		}
		return r
	}, u.Fragment))
	if name == "" {
		name = host
	}
	node = fmt.Sprintf("VLESS,%s,%d,%q,transport=tcp,over-tls=true,public-key=%q,tls-profile=chrome147", host, port, id.String(), key)
	if shortID != "" {
		node += ",short-id=" + shortID
	}
	if serverName != "" {
		node += ",sni=" + serverName
	}
	if flow != "" {
		node += ",flow=" + flow
	}
	return name, node, true
}
