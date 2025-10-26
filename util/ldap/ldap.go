package ldaputil

import (
	"crypto/tls"
	"fmt"

	"github.com/go-ldap/ldap/v3"
)

type Config struct {
	Host       string
	Port       int
	UseTLS     bool
	BindDN     string
	Password   string
	BaseDN     string
	UserFilter string
	UserAttr   string
	FlagField  string
	TruthyVals []string
	Invert     bool
}

// FetchVlessFlags returns map[email]enabled
func FetchVlessFlags(cfg Config) (map[string]bool, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	var conn *ldap.Conn
	var err error
	if cfg.UseTLS {
		conn, err = ldap.DialTLS("tcp", addr, &tls.Config{InsecureSkipVerify: false})
	} else {
		conn, err = ldap.Dial("tcp", addr)
	}
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if cfg.BindDN != "" {
		if err := conn.Bind(cfg.BindDN, cfg.Password); err != nil {
			return nil, err
		}
	}

	if cfg.UserFilter == "" {
		cfg.UserFilter = "(objectClass=person)"
	}
	if cfg.UserAttr == "" {
		cfg.UserAttr = "mail"
	}
	// if field not set we fallback to legacy vless_enabled
	if cfg.FlagField == "" {
		cfg.FlagField = "vless_enabled"
	}

	req := ldap.NewSearchRequest(
		cfg.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		cfg.UserFilter,
		[]string{cfg.UserAttr, cfg.FlagField},
		nil,
	)

	res, err := conn.Search(req)
	if err != nil {
		return nil, err
	}

	result := make(map[string]bool, len(res.Entries))
	for _, e := range res.Entries {
		user := e.GetAttributeValue(cfg.UserAttr)
		if user == "" {
			continue
		}
		val := e.GetAttributeValue(cfg.FlagField)
		enabled := false
		for _, t := range cfg.TruthyVals {
			if val == t {
				enabled = true
				break
			}
		}
		if cfg.Invert {
			enabled = !enabled
		}
		result[user] = enabled
	}
	return result, nil
}

// AuthenticateUser searches user by cfg.UserAttr and attempts to bind with provided password.
func AuthenticateUser(cfg Config, username, password string) (bool, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	var conn *ldap.Conn
	var err error
	if cfg.UseTLS {
		conn, err = ldap.DialTLS("tcp", addr, &tls.Config{InsecureSkipVerify: false})
	} else {
		conn, err = ldap.Dial("tcp", addr)
	}
	if err != nil {
		return false, err
	}
	defer conn.Close()

	// Optional initial bind for search
	if cfg.BindDN != "" {
		if err := conn.Bind(cfg.BindDN, cfg.Password); err != nil {
			return false, err
		}
	}

	if cfg.UserFilter == "" {
		cfg.UserFilter = "(objectClass=person)"
	}
	if cfg.UserAttr == "" {
		cfg.UserAttr = "uid"
	}

	// Build filter to find specific user
	filter := fmt.Sprintf("(&%s(%s=%s))", cfg.UserFilter, cfg.UserAttr, ldap.EscapeFilter(username))
	req := ldap.NewSearchRequest(
		cfg.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, 0, false,
		filter,
		[]string{"dn"},
		nil,
	)
	res, err := conn.Search(req)
	if err != nil {
		return false, err
	}
	if len(res.Entries) == 0 {
		return false, nil
	}
	userDN := res.Entries[0].DN
	// Try to bind as the user
	if err := conn.Bind(userDN, password); err != nil {
		return false, nil
	}
	return true, nil
}
