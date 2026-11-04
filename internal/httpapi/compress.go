package httpapi

import (
	"compress/gzip"
	"net/http"
	"strings"
	"sync"
)

var gzipWriters = sync.Pool{New: func() any { return gzip.NewWriter(nil) }}

var compressible = []string{"application/json", "image/svg+xml", "text/plain"}

func compress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		dynamic := strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/badge") || path == "/metrics"
		if !dynamic || path == "/api/stream" || r.Method == http.MethodHead || !acceptsGzip(r) {
			next.ServeHTTP(w, r)
			return
		}
		gw := &gzipResponse{ResponseWriter: w}
		defer gw.close()
		next.ServeHTTP(gw, r)
	})
}

type gzipResponse struct {
	http.ResponseWriter
	zw      *gzip.Writer
	decided bool
}

func (w *gzipResponse) decide(code int) {
	w.decided = true
	h := w.Header()
	if code < 200 || code == http.StatusNoContent || code == http.StatusNotModified || h.Get("Content-Encoding") != "" {
		return
	}
	kind := h.Get("Content-Type")
	if !matches(kind) {
		return
	}
	h.Del("Content-Length")
	h.Set("Content-Encoding", "gzip")
	h.Add("Vary", "Accept-Encoding")
	w.zw = gzipWriters.Get().(*gzip.Writer)
	w.zw.Reset(w.ResponseWriter)
}

func matches(kind string) bool {
	for _, prefix := range compressible {
		if strings.HasPrefix(kind, prefix) {
			return true
		}
	}
	return false
}

func (w *gzipResponse) WriteHeader(code int) {
	if !w.decided {
		w.decide(code)
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipResponse) Write(b []byte) (int, error) {
	if !w.decided {
		w.WriteHeader(http.StatusOK)
	}
	if w.zw != nil {
		return w.zw.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

func (w *gzipResponse) close() {
	if w.zw == nil {
		return
	}
	_ = w.zw.Close()
	w.zw.Reset(nil)
	gzipWriters.Put(w.zw)
	w.zw = nil
}

func (w *gzipResponse) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
