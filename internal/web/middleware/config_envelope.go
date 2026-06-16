package middleware

import (
	"bytes"
	"crypto/subtle"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/util/wirecodec"
)

// maxDecodedConfigBytes caps a decompressed request body (defense in depth on
// top of wirecodec's own ceiling).
const maxDecodedConfigBytes = 8 << 20

// ConfigEnvelopeMiddleware advertises node envelope support on every response
// and, for requests that opt into the envelope, decompresses (zstd) and verifies
// the X-Config-Sha256 integrity hash before the body reaches the handler. A
// request carrying neither envelope header passes through untouched, so old
// panels and plain calls keep working (mixed-version safe).
func ConfigEnvelopeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header(wirecodec.CapsHeader, wirecodec.CapZstd)

		enc := c.GetHeader("Content-Encoding")
		sum := c.GetHeader(wirecodec.HashHeader)
		if enc != wirecodec.EncodingZstd && sum == "" {
			c.Next()
			return
		}

		// On the envelope path, zstd is the only encoding we understand. Reject any
		// other Content-Encoding rather than hashing/forwarding a still-encoded body
		// the downstream handler can't read.
		if enc != "" && enc != wirecodec.EncodingZstd {
			c.AbortWithStatus(http.StatusUnsupportedMediaType)
			return
		}

		raw, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		_ = c.Request.Body.Close()

		if enc == wirecodec.EncodingZstd {
			decoded, derr := wirecodec.Decompress(raw, maxDecodedConfigBytes)
			if derr != nil {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			raw = decoded
			c.Request.Header.Del("Content-Encoding")
		}

		if sum != "" {
			got := wirecodec.Sha256Hex(raw)
			if subtle.ConstantTimeCompare([]byte(got), []byte(sum)) != 1 {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(raw))
		c.Request.ContentLength = int64(len(raw))
		c.Next()
	}
}
