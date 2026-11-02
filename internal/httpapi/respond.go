package httpapi

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/store"
)

const maxRequestBody = 1 << 20

type Error struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitzero"`
}

func (e *Error) Error() string {
	return e.Message
}

func errorf(status int, code, format string, args ...any) *Error {
	return &Error{Status: status, Code: code, Message: fmt.Sprintf(format, args...)}
}

func badRequest(format string, args ...any) *Error {
	return errorf(http.StatusBadRequest, "bad_request", format, args...)
}

func notFound(what string) *Error {
	return errorf(http.StatusNotFound, "not_found", "%s not found", what)
}

func invalid(err error) *Error {
	out := &Error{Status: http.StatusUnprocessableEntity, Code: "invalid", Message: "validation failed", Fields: map[string]string{}}
	for _, e := range flatten(err) {
		if fe, ok := errors.AsType[*monitor.FieldError](e); ok {
			out.Fields[fe.Field] = fe.Message
			continue
		}
		out.Message = e.Error()
	}
	return out
}

func flatten(err error) []error {
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		var out []error
		for _, e := range joined.Unwrap() {
			out = append(out, flatten(e)...)
		}
		return out
	}
	return []error{err}
}

type handlerFunc func(w http.ResponseWriter, r *http.Request) error

func (s *Server) handle(h handlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			s.fail(w, r, err)
		}
	})
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	apiErr, ok := errors.AsType[*Error](err)
	switch {
	case ok:
	case errors.Is(err, store.ErrNotFound):
		apiErr = notFound("resource")
	case errors.Is(err, store.ErrConflict):
		apiErr = errorf(http.StatusConflict, "conflict", "resource already exists")
	default:
		if _, isField := errors.AsType[*monitor.FieldError](err); isField {
			apiErr = invalid(err)
			break
		}
		if r.Context().Err() == nil {
			s.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		}
		apiErr = errorf(http.StatusInternalServerError, "internal", "internal server error")
	}
	if err := writeJSON(w, apiErr.Status, apiErr); err != nil {
		s.log.Debug("write error response", "err", err)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	return json.MarshalWrite(w, v)
}

func ok(w http.ResponseWriter, v any) error {
	return writeJSON(w, http.StatusOK, v)
}

func created(w http.ResponseWriter, v any) error {
	return writeJSON(w, http.StatusCreated, v)
}

func noContent(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	if err := json.UnmarshalRead(r.Body, v, json.RejectUnknownMembers(true)); err != nil {
		if _, tooLarge := errors.AsType[*http.MaxBytesError](err); tooLarge {
			return errorf(http.StatusRequestEntityTooLarge, "too_large", "request body is too large")
		}
		return badRequest("invalid json: %v", err)
	}
	return nil
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, notFound("resource")
	}
	return id, nil
}
