package xray

import (
	"strconv"
	"strings"
)

// statEmailSep separates the inbound id from the logical client email in the
// per-attachment accounting identity Xray meters. The id is numeric, so splitting
// on the FIRST separator is unambiguous even when the email itself contains it.
const statEmailSep = "::"

// EncodeStatEmail builds the per-attachment accounting identity
// ("<inboundId>::<email>") written into the Xray config and live
// AddUser/RemoveUser calls, so Xray meters each (client, inbound) pair separately
// for the Traffic Multiplier feature. It is reversed by DecodeStatEmail everywhere
// outside the Xray boundary (stats, access log, online API, IP-limit, sub).
func EncodeStatEmail(inboundId int, email string) string {
	return strconv.Itoa(inboundId) + statEmailSep + email
}

// DecodeStatEmail reverses EncodeStatEmail. It returns the inbound id and the
// logical email when s is an encoded identity, or (0, s, false) when s carries no
// numeric "<id>::" prefix — i.e. a legacy/un-encoded email (the brief pre-rewrite
// window on upgrade), which is passed through unchanged so mixed state keeps
// working. Splitting on the FIRST separator keeps an email that itself contains
// "::" intact, since encode always prepends "<id>::".
func DecodeStatEmail(s string) (inboundId int, email string, ok bool) {
	idx := strings.Index(s, statEmailSep)
	if idx <= 0 {
		return 0, s, false
	}
	id, err := strconv.Atoi(s[:idx])
	if err != nil {
		return 0, s, false
	}
	return id, s[idx+len(statEmailSep):], true
}

// DecodeStatEmailValue is the email-only convenience form: it returns the logical
// email whether or not s was encoded.
func DecodeStatEmailValue(s string) string {
	_, email, _ := DecodeStatEmail(s)
	return email
}
