package httpapi

import (
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

var encodings = []struct {
	name, ext string
}{
	{"br", ".br"},
	{"gzip", ".gz"},
}

func spa(assets fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}

		if info, err := fs.Stat(assets, name); err != nil || info.IsDir() {
			if strings.HasPrefix(name, "_app/") || path.Ext(name) != "" {
				http.NotFound(w, r)
				return
			}
			name = "index.html"
		}

		h := w.Header()
		if strings.HasPrefix(name, "_app/immutable/") {
			h.Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			h.Set("Cache-Control", "no-cache")
		}
		if ctype := mime.TypeByExtension(path.Ext(name)); ctype != "" {
			h.Set("Content-Type", ctype)
		}
		h.Add("Vary", "Accept-Encoding")

		accept := r.Header.Get("Accept-Encoding")
		for _, enc := range encodings {
			if !strings.Contains(accept, enc.name) {
				continue
			}
			if f, err := assets.Open(name + enc.ext); err == nil {
				defer f.Close()
				h.Set("Content-Encoding", enc.name)
				serve(w, r, name, f)
				return
			}
		}

		f, err := assets.Open(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		serve(w, r, name, f)
	})
}

func serve(w http.ResponseWriter, r *http.Request, name string, f fs.File) {
	if rs, ok := f.(io.ReadSeeker); ok {
		http.ServeContent(w, r, name, time.Time{}, rs)
		return
	}
	io.Copy(w, f)
}
