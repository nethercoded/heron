package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nethercoded/heron/internal/monitor"
	"github.com/nethercoded/heron/internal/notify"
	"github.com/nethercoded/heron/internal/store"
)

func (s *Server) listChannels(w http.ResponseWriter, r *http.Request) error {
	list, err := s.store.Channels(r.Context())
	if err != nil {
		return err
	}
	out := make([]channelDTO, 0, len(list))
	for _, c := range list {
		out = append(out, toChannel(c))
	}
	return ok(w, out)
}

func (s *Server) createChannel(w http.ResponseWriter, r *http.Request) error {
	var in channelInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	c := &notify.Channel{Name: strings.TrimSpace(in.Name), Kind: in.Kind, Enabled: in.Enabled, Config: in.Config}
	if err := c.Validate(); err != nil {
		return invalid(err)
	}
	if err := s.store.CreateChannel(r.Context(), c); err != nil {
		return err
	}
	describe(r, c.Name)
	return created(w, toChannel(c))
}

func (s *Server) updateChannel(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	var in channelInput
	if err := decode(w, r, &in); err != nil {
		return err
	}
	c := &notify.Channel{ID: id, Name: strings.TrimSpace(in.Name), Kind: in.Kind, Enabled: in.Enabled, Config: in.Config}
	if err := c.Validate(); err != nil {
		return invalid(err)
	}
	if err := s.store.UpdateChannel(r.Context(), c); err != nil {
		return err
	}
	describe(r, c.Name)
	fresh, err := s.store.Channel(r.Context(), id)
	if err != nil {
		return err
	}
	return ok(w, toChannel(fresh))
}

func (s *Server) deleteChannel(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if c, err := s.store.Channel(r.Context(), id); err == nil {
		describe(r, c.Name)
	}
	if err := s.store.DeleteChannel(r.Context(), id); err != nil {
		return err
	}
	return noContent(w)
}

func (s *Server) testChannel(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	c, err := s.store.Channel(r.Context(), id)
	if err != nil {
		return err
	}

	msg := notify.Message{
		Event:   notify.EventTest,
		Monitor: &monitor.Monitor{Name: "heron", Slug: "test", Kind: monitor.KindHTTP},
		Status:  monitor.StatusUp,
		Reason:  "If you can read this, notifications from heron reach this channel.",
		At:      time.Now(),
		Link:    s.engine.Link(nil),
	}
	if err := s.sender.Send(r.Context(), *c, msg); err != nil {
		return errorf(http.StatusBadGateway, "delivery_failed", "%s", deliveryError(err))
	}
	return noContent(w)
}

func deliveryError(err error) string {
	if ue, ok := errors.AsType[*url.Error](err); ok {
		return ue.Op + " " + redactURL(ue.URL) + ": " + ue.Err.Error()
	}
	return err.Error()
}

func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "request"
	}
	return u.Scheme + "://" + u.Host
}

func isConflict(err error) bool {
	return errors.Is(err, store.ErrConflict)
}
