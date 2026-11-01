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
