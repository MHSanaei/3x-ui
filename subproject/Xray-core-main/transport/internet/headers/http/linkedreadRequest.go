package http

import (
	"bufio"
	"net/http"
	// required to use go:linkname
	_ "unsafe"
)

//go:linkname readRequest net/http.readRequest
func readRequest(b *bufio.Reader) (req *http.Request, err error)
