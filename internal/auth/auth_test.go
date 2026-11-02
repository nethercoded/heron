package auth

import (
	"strings"
	"testing"
	"time"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Fatalf("unexpected encoding %q", hash)
	}

	if ok, err := Verify(hash, "correct horse battery"); !ok || err != nil {
		t.Errorf("valid password rejected: %v", err)
	}
	if ok, _ := Verify(hash, "wrong horse battery"); ok {
		t.Error("invalid password accepted")
	}

	other, _ := Hash("correct horse battery")
	if other == hash {
		t.Error("salt is not random")
	}
}

func TestVerifyMalformed(t *testing.T) {
	for _, h := range []string{"", "plain", "$argon2i$v=19$m=1,t=1,p=1$c2FsdA$aGFzaA", "$argon2id$v=19$m=x$a$b"} {
		if _, err := Verify(h, "x"); err == nil {
			t.Errorf("Verify(%q) returned no error", h)
		}
	}
}

func TestLimiter(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	l := NewLimiter(3, time.Minute)
	l.now = func() time.Time { return now }

	for i := range 3 {
		if ok, _ := l.Allow("1.2.3.4"); !ok {
			t.Fatalf("attempt %d blocked", i+1)
		}
	}
	ok, retry := l.Allow("1.2.3.4")
	if ok || retry != time.Minute {
		t.Fatalf("fourth attempt: ok=%v retry=%s", ok, retry)
	}
	if ok, _ := l.Allow("5.6.7.8"); !ok {
		t.Error("other key should not be limited")
	}

	now = now.Add(time.Minute)
	if ok, _ := l.Allow("1.2.3.4"); !ok {
		t.Error("window did not reset")
	}
}

func TestTOTP(t *testing.T) {
	secret := secretEncoding.EncodeToString([]byte("12345678901234567890"))
	for at, want := range map[int64]string{59: "287082", 1111111109: "081804", 2000000000: "279037"} {
		if got := TOTP(secret, time.Unix(at, 0)); got != want {
			t.Errorf("TOTP at %d = %s, want %s", at, got, want)
		}
	}

	now := time.Unix(1_700_000_000, 0)
	step, ok := CheckTOTP(secret, TOTP(secret, now.Add(-30*time.Second)), now)
	if !ok || step != now.Unix()/30-1 {
		t.Errorf("previous step rejected: %d %v", step, ok)
	}
	if _, ok := CheckTOTP(secret, TOTP(secret, now.Add(-90*time.Second)), now); ok {
		t.Error("stale code accepted")
	}
}

func TestTickets(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	tickets := NewTickets[int64](time.Minute)
	tickets.now = func() time.Time { return now }

	id := tickets.Issue(7)
	if v, ok := tickets.Peek(id); !ok || v != 7 {
		t.Fatalf("peek = %d %v", v, ok)
	}
	if v, ok := tickets.Take(id); !ok || v != 7 {
		t.Fatalf("take = %d %v", v, ok)
	}
	if _, ok := tickets.Take(id); ok {
		t.Error("ticket used twice")
	}

	id = tickets.Issue(8)
	now = now.Add(2 * time.Minute)
	if _, ok := tickets.Peek(id); ok {
		t.Error("expired ticket accepted")
	}
}
