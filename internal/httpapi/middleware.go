package httpapi

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"runtime/debug"
	"strings"
	"time"

	"github.com/nethercoded/heron/internal/store"
)

type userKey struct{}

func userFrom(ctx context.Context) *store.User {
	u, _ := ctx.Value(userKey{}).(*store.User)
	return u
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := s.sessions.User(r.Context(), w, r)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				s.fail(w, r, err)
				return
			}
			s.fail(w, r, errorf(http.StatusUnauthorized, "unauthorized", "sign in required"))
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if !user.Role.Allows(store.RoleAdmin) && !readOnly(r.Method) && !strings.HasPrefix(r.URL.Path, "/api/admin/account/") {
			s.fail(w, r, errorf(http.StatusForbidden, "forbidden", "your role can only view"))
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, user)))
	})
}

func (s *Server) role(min store.Role, h handlerFunc) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if !userFrom(r.Context()).Role.Allows(min) {
			return errorf(http.StatusForbidden, "forbidden", "this needs the %s role", min)
		}
		return h(w, r)
	}
}

func readOnly(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

type recorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *recorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

func (r *recorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func (s *Server) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		rec := &recorder{ResponseWriter: w}

		defer func() {
			if v := recover(); v != nil {
				if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(v)
				}
				s.log.Error("panic", "value", v, "path", r.URL.Path, "stack", string(debug.Stack()))
				if rec.status == 0 {
					s.fail(rec, r, errors.New("panic"))
				}
			}
			if strings.HasPrefix(r.URL.Path, "/api/") || rec.status >= 500 {
				s.log.Debug("request",
					"method", r.Method,
					"path", r.URL.Path,
					"status", rec.status,
					"bytes", rec.bytes,
					"duration", time.Since(started).Round(time.Microsecond),
					"ip", s.clientIP(r))
			}
		}()

		next.ServeHTTP(rec, r)
	})
}

func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	remote, err := netip.ParseAddr(host)
	if err != nil || !s.trusted(remote) {
		return host
	}

	hops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		addr, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			break
		}
		if !s.trusted(addr) {
			return addr.String()
		}
	}
	if real, err := netip.ParseAddr(r.Header.Get("X-Real-IP")); err == nil {
		return real.String()
	}
	return host
}

func (s *Server) trusted(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, p := range s.opts.TrustedProxies {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
