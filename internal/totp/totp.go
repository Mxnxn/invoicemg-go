// Package totp ports Helpers/Totp.js - RFC 6238 codes over base32 secrets, with the exact defaults
// the authenticator apps and the Node helper agree on (SHA1, 6 digits, 30s step, ±1 window). Pure
// stdlib, no dependency; the clock is injectable for tests.
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	stepSeconds = 30
	digits      = 6
	issuer      = "InvoiceMG"
)

var (
	b32       = base32.StdEncoding.WithPadding(base32.NoPadding)
	sixDigits = regexp.MustCompile(`^\d{6}$`)
)

func decode(secret string) ([]byte, error) {
	clean := strings.ToUpper(strings.TrimRight(strings.Join(strings.Fields(secret), ""), "="))
	if clean == "" {
		return nil, fmt.Errorf("empty secret")
	}
	return b32.DecodeString(clean)
}

// hotp is RFC 4226 (TOTP is HOTP with a clock-derived counter).
func hotp(secret []byte, counter uint64) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, secret)
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	bin := (uint32(sum[off]&0x7f) << 24) | (uint32(sum[off+1]) << 16) | (uint32(sum[off+2]) << 8) | uint32(sum[off+3])
	return fmt.Sprintf("%0*d", digits, bin%1_000_000)
}

// Verify checks a 6-digit code against the base32 secret within ±1 step of now.
func Verify(secret, code string, now time.Time) bool {
	ok, _ := VerifyStep(secret, code, now)
	return ok
}

// VerifyStep is Verify plus the matching step, so a caller (the dev-token route) can burn a step to
// stop one observed code minting more than once. step is -1 when invalid.
func VerifyStep(secret, code string, now time.Time) (bool, int64) {
	candidate := strings.TrimSpace(code)
	if !sixDigits.MatchString(candidate) {
		return false, -1
	}
	buf, err := decode(secret)
	if err != nil {
		return false, -1
	}
	current := now.Unix() / stepSeconds
	for offset := int64(-1); offset <= 1; offset++ {
		step := current + offset
		if step < 0 {
			continue
		}
		want := hotp(buf, uint64(step))
		if subtle.ConstantTimeCompare([]byte(want), []byte(candidate)) == 1 {
			return true, step
		}
	}
	return false, -1
}

// GenerateSecret returns a fresh base32 secret (20 random bytes, no padding).
func GenerateSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return b32.EncodeToString(b), nil
}

// OtpauthURI is the provisioning URI an authenticator app expects (QR or typed), with every
// default spelled out so a code is not right on one phone and wrong on another.
func OtpauthURI(secret, account string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{
		"secret":    {secret},
		"issuer":    {issuer},
		"algorithm": {"SHA1"},
		"digits":    {fmt.Sprintf("%d", digits)},
		"period":    {fmt.Sprintf("%d", stepSeconds)},
	}
	return "otpauth://totp/" + label + "?" + q.Encode()
}
