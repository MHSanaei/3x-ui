package tls

import _ "unsafe"

//go:linkname errNoCertificates crypto/tls.errNoCertificates
var errNoCertificates error
