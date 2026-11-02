package httpapi

import (
	"encoding/base64"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"rsc.io/qr"

	"github.com/nethercoded/heron/internal/auth"
	"github.com/nethercoded/heron/internal/store"
)

type ceremony struct {
	session webauthn.SessionData
	userID  int64
}

type passkeyUser struct {
	*store.User
	keys []webauthn.Credential
}

func (u passkeyUser) WebAuthnID() []byte                         { return u.Handle }
func (u passkeyUser) WebAuthnName() string                       { return u.Username }
func (u passkeyUser) WebAuthnDisplayName() string                { return u.Username }
func (u passkeyUser) WebAuthnCredentials() []webauthn.Credential { return u.keys }

type passkeyDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	CreatedAt  stamp  `json:"created_at"`
	LastUsedAt stamp  `json:"last_used_at,omitzero"`
}

func (s *Server) passkeyUser(r *http.Request, u *store.User) (passkeyUser, error) {
	list, err := s.store.Passkeys(r.Context(), u.ID)
	if err != nil {
		return passkeyUser{}, err
	}
	pu := passkeyUser{User: u}
	for _, p := range list {
		var cred webauthn.Credential
		if err := json.Unmarshal(p.Credential, &cred); err != nil {
			return passkeyUser{}, fmt.Errorf("passkey %x: %w", p.ID, err)
		}
		pu.keys = append(pu.keys, cred)
	}
	return pu, nil
}

func (s *Server) relyingParty(r *http.Request) (*webauthn.WebAuthn, error) {
	origin, err := url.Parse(r.Header.Get("Origin"))
	if err != nil || origin.Host == "" || (origin.Scheme != "https" && origin.Hostname() != "localhost") {
		return nil, errorf(http.StatusBadRequest, "insecure_origin", "passkeys need https, open heron through its domain")
	}
	return webauthn.New(&webauthn.Config{
		RPID:          origin.Hostname(),
		RPDisplayName: "heron",
		RPOrigins:     []string{origin.Scheme + "://" + origin.Host},
	})
}

func (s *Server) loginWithCode(w http.ResponseWriter, r *http.Request) error {
	ip := s.clientIP(r)
	if allowed, _ := s.limiter.Allow(ip); !allowed {
		return errorf(http.StatusTooManyRequests, "rate_limited", "too many attempts, try again later")
	}
	var in struct {
		Ticket string `json:"ticket"`
		Code   string `json:"code"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	id, found := s.secondStep.Peek(in.Ticket)
	if !found {
		return errorf(http.StatusUnauthorized, "ticket_expired", "the sign in took too long, start again")
	}
	user, err := s.store.User(r.Context(), id)
	if err != nil {
		return err
	}
	if err := s.checkCode(r, user, in.Code); err != nil {
		s.record(r, user, "auth.code_failed", "")
		return err
	}
	s.secondStep.Take(in.Ticket)
	s.limiter.Reset(ip)
	if err := s.sessions.Start(r.Context(), w, r, user); err != nil {
		return err
	}
	s.record(r, user, "auth.login", "password + code")
	return ok(w, loginDTO{User: toUser(user)})
}

func (s *Server) checkCode(r *http.Request, user *store.User, code string) error {
	step, valid := auth.CheckTOTP(user.TOTPSecret, code, time.Now())
	if valid {
		fresh, err := s.store.UseTOTPStep(r.Context(), user.ID, step)
		if err != nil {
			return err
		}
		valid = fresh
	}
	if !valid {
		return &Error{Status: http.StatusUnprocessableEntity, Code: "invalid", Message: "validation failed", Fields: map[string]string{"code": "is wrong or was already used"}}
	}
	return nil
}

func (s *Server) startTOTP(w http.ResponseWriter, r *http.Request) error {
	user := userFrom(r.Context())
	if user.TwoFactor() {
		return errorf(http.StatusConflict, "already_enabled", "two-factor authentication is already on")
	}
	secret := auth.NewTOTPSecret()
	uri := auth.TOTPURI("heron", user.Username, secret)
	svg, err := qrSVG(uri)
	if err != nil {
		return err
	}
	return ok(w, map[string]string{"secret": secret, "uri": uri, "qr": svg})
}

func (s *Server) enableTOTP(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Secret string `json:"secret"`
		Code   string `json:"code"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	user := userFrom(r.Context())
	if user.TwoFactor() {
		return errorf(http.StatusConflict, "already_enabled", "two-factor authentication is already on")
	}
	if !auth.ValidTOTPSecret(in.Secret) {
		return badRequest("invalid secret")
	}
	step, valid := auth.CheckTOTP(in.Secret, in.Code, time.Now())
	if !valid {
		return &Error{Status: http.StatusUnprocessableEntity, Code: "invalid", Message: "validation failed", Fields: map[string]string{"code": "does not match, check the time on your phone"}}
	}
	if err := s.store.SetTOTP(r.Context(), user.ID, strings.ToUpper(in.Secret)); err != nil {
		return err
	}
	if _, err := s.store.UseTOTPStep(r.Context(), user.ID, step); err != nil {
		return err
	}
	return noContent(w)
}

func (s *Server) disableTOTP(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Code string `json:"code"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	user := userFrom(r.Context())
	if !user.TwoFactor() {
		return noContent(w)
	}
	if err := s.checkCode(r, user, in.Code); err != nil {
		return err
	}
	if err := s.store.SetTOTP(r.Context(), user.ID, ""); err != nil {
		return err
	}
	return noContent(w)
}

func (s *Server) listPasskeys(w http.ResponseWriter, r *http.Request) error {
	list, err := s.store.Passkeys(r.Context(), userFrom(r.Context()).ID)
	if err != nil {
		return err
	}
	out := make([]passkeyDTO, 0, len(list))
	for _, p := range list {
		out = append(out, passkeyDTO{ID: base64.RawURLEncoding.EncodeToString(p.ID), Name: p.Name, CreatedAt: stamp(p.CreatedAt), LastUsedAt: stamp(p.LastUsedAt)})
	}
	return ok(w, out)
}

func (s *Server) beginPasskey(w http.ResponseWriter, r *http.Request) error {
	rp, err := s.relyingParty(r)
	if err != nil {
		return err
	}
	user, err := s.passkeyUser(r, userFrom(r.Context()))
	if err != nil {
		return err
	}
	options, session, err := rp.BeginRegistration(user,
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		webauthn.WithExclusions(webauthn.Credentials(user.keys).CredentialDescriptors()))
	if err != nil {
		return err
	}
	ticket := s.ceremonies.Issue(ceremony{session: *session, userID: user.ID})
	return ok(w, map[string]any{"ticket": ticket, "options": options.Response})
}

func (s *Server) finishPasskey(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Ticket     string         `json:"ticket"`
		Name       string         `json:"name"`
		Credential jsontext.Value `json:"credential"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	c, found := s.ceremonies.Take(in.Ticket)
	current := userFrom(r.Context())
	if !found || c.userID != current.ID {
		return errorf(http.StatusBadRequest, "ticket_expired", "the passkey prompt took too long, try again")
	}
	rp, err := s.relyingParty(r)
	if err != nil {
		return err
	}
	user, err := s.passkeyUser(r, current)
	if err != nil {
		return err
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(in.Credential)
	if err != nil {
		return badRequest("invalid passkey response")
	}
	cred, err := rp.CreateCredential(user, c.session, parsed)
	if err != nil {
		s.log.Warn("passkey registration rejected", "user", current.Username, "err", err)
		return errorf(http.StatusBadRequest, "passkey_rejected", "the passkey could not be verified")
	}
	raw, err := json.Marshal(cred)
	if err != nil {
		return err
	}

	name := strings.TrimSpace(in.Name)
	if name == "" || len(name) > 60 {
		name = "Passkey"
	}
	p := &store.Passkey{ID: cred.ID, UserID: current.ID, Name: name, Credential: raw}
	if err := s.store.AddPasskey(r.Context(), p); err != nil {
		return err
	}
	describe(r, name)
	return created(w, passkeyDTO{ID: base64.RawURLEncoding.EncodeToString(p.ID), Name: p.Name, CreatedAt: stamp(p.CreatedAt)})
}

func (s *Server) deletePasskey(w http.ResponseWriter, r *http.Request) error {
	id, err := base64.RawURLEncoding.DecodeString(r.PathValue("id"))
	if err != nil {
		return notFound("passkey")
	}
	if err := s.store.DeletePasskey(r.Context(), userFrom(r.Context()).ID, id); err != nil {
		return err
	}
	return noContent(w)
}

func (s *Server) beginPasskeyLogin(w http.ResponseWriter, r *http.Request) error {
	if allowed, _ := s.limiter.Allow(s.clientIP(r)); !allowed {
		return errorf(http.StatusTooManyRequests, "rate_limited", "too many attempts, try again later")
	}
	rp, err := s.relyingParty(r)
	if err != nil {
		return err
	}
	options, session, err := rp.BeginDiscoverableLogin()
	if err != nil {
		return err
	}
	ticket := s.ceremonies.Issue(ceremony{session: *session})
	return ok(w, map[string]any{"ticket": ticket, "options": options.Response})
}

func (s *Server) finishPasskeyLogin(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Ticket     string         `json:"ticket"`
		Credential jsontext.Value `json:"credential"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	c, found := s.ceremonies.Take(in.Ticket)
	if !found || c.userID != 0 {
		return errorf(http.StatusBadRequest, "ticket_expired", "the passkey prompt took too long, try again")
	}
	rp, err := s.relyingParty(r)
	if err != nil {
		return err
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(in.Credential)
	if err != nil {
		return badRequest("invalid passkey response")
	}

	account, cred, err := rp.ValidatePasskeyLogin(func(_, handle []byte) (webauthn.User, error) {
		u, err := s.store.UserByHandle(r.Context(), handle)
		if err != nil {
			return nil, err
		}
		return s.passkeyUser(r, u)
	}, c.session, parsed)
	if err != nil {
		s.log.Warn("passkey sign in rejected", "ip", s.clientIP(r), "err", err)
		return errorf(http.StatusUnauthorized, "passkey_rejected", "this passkey is not registered here")
	}
	user, valid := account.(passkeyUser)
	if !valid {
		return errors.New("unexpected passkey user type")
	}

	raw, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	if err := s.store.UsePasskey(r.Context(), cred.ID, raw); err != nil {
		return err
	}
	if err := s.sessions.Start(r.Context(), w, r, user.User); err != nil {
		return err
	}
	s.record(r, user.User, "auth.login", "passkey")
	return ok(w, loginDTO{User: toUser(user.User)})
}

func qrSVG(text string) (string, error) {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return "", err
	}
	const quiet = 2
	var path strings.Builder
	for y := range code.Size {
		for x := range code.Size {
			if code.Black(x, y) {
				fmt.Fprintf(&path, "M%d %dh1v1h-1z", x+quiet, y+quiet)
			}
		}
	}
	size := code.Size + 2*quiet
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges"><rect width="%d" height="%d" fill="#fff"/><path fill="#000" d="%s"/></svg>`,
		size, size, size, size, path.String()), nil
}
