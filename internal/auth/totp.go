package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	totpPeriod = 30
	totpDigits = 6
)

var secretEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

func NewTOTPSecret() string {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	return secretEncoding.EncodeToString(b)
}

func TOTPURI(issuer, account, secret string) string {
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("period", strconv.Itoa(totpPeriod))
	q.Set("digits", strconv.Itoa(totpDigits))
	return "otpauth://totp/" + url.PathEscape(issuer+":"+account) + "?" + q.Encode()
}

func ValidTOTPSecret(secret string) bool {
	key, err := secretEncoding.DecodeString(strings.ToUpper(secret))
	return err == nil && len(key) >= 10
}

func CheckTOTP(secret, code string, at time.Time) (int64, bool) {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(code) != totpDigits {
		return 0, false
	}
	key, err := secretEncoding.DecodeString(strings.ToUpper(secret))
	if err != nil {
		return 0, false
	}
	step := at.Unix() / totpPeriod
	for _, s := range []int64{step, step - 1, step + 1} {
		if subtle.ConstantTimeCompare([]byte(totpCode(key, s)), []byte(code)) == 1 {
			return s, true
		}
	}
	return 0, false
}

func totpCode(key []byte, step int64) string {
	mac := hmac.New(sha1.New, key)
	_ = binary.Write(mac, binary.BigEndian, step)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:]) & 0x7fffffff
	return fmt.Sprintf("%0*d", totpDigits, value%1_000_000)
}

func TOTP(secret string, at time.Time) string {
	key, err := secretEncoding.DecodeString(strings.ToUpper(secret))
	if err != nil {
		return ""
	}
	return totpCode(key, at.Unix()/totpPeriod)
}
