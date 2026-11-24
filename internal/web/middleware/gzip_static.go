package middleware

import (
	"bytes"
	"compress/gzip"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// GzipStatic serves a static route's "*filepath" from a gzip copy made once per
// file, so a stalled download holds shared bytes instead of its own compressor.
func GzipStatic(files fs.FS) gin.HandlerFunc {
	var compressed sync.Map // name -> func() []byte
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			return
		}
		c.Writer.Header().Add("Vary", "Accept-Encoding")
		if !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+c.Param("filepath")), "/")
		// Only names that exist get an entry, so requests cannot grow the map.
		info, err := fs.Stat(files, name)
		if err != nil || info.IsDir() {
			return
		}
		once, _ := compressed.LoadOrStore(name, sync.OnceValue(func() []byte {
			return gzipFile(files, name)
		}))
		body := once.(func() []byte)()
		if body == nil {
			return
		}
		c.Header("Content-Encoding", "gzip")
		// ServeContent leaves Content-Length unset once Content-Encoding is present.
		c.Header("Content-Length", strconv.Itoa(len(body)))
		http.ServeContent(c.Writer, c.Request, name, info.ModTime(), bytes.NewReader(body))
		c.Abort()
	}
}

// gzipFile returns the file compressed, or nil to leave it to the static route.
func gzipFile(files fs.FS, name string) []byte {
	raw, err := fs.ReadFile(files, name)
	if err != nil {
		return nil
	}
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if _, err := zw.Write(raw); err != nil {
		return nil
	}
	if err := zw.Close(); err != nil {
		return nil
	}
	return buf.Bytes()
}
