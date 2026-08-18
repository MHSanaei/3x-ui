package network

import (
	"errors"
	"net"
	"net/http"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// ServeHTTP runs a panel HTTP server and records unexpected listener failures.
// A normal Shutdown returns http.ErrServerClosed and is intentionally silent.
func ServeHTTP(server *http.Server, listener net.Listener, name string) {
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error(name, " stopped unexpectedly: ", err)
	}
}
