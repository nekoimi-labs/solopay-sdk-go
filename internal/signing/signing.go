package signing

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strconv"
	"strings"
)

func BodySHA256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func Canonical(method, path string, timestamp int64, nonce string, body []byte) string {
	return strings.Join([]string{
		strings.ToUpper(strings.TrimSpace(method)),
		strings.TrimSpace(path),
		strconv.FormatInt(timestamp, 10),
		strings.TrimSpace(nonce),
		BodySHA256Hex(body),
	}, "\n")
}

func SignHex(secret []byte, canonical string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifyHex(secret []byte, canonical, signature string) bool {
	signature = strings.ToLower(strings.TrimSpace(signature))
	if len(signature) != sha256.Size*2 {
		return false
	}
	want := SignHex(secret, canonical)
	return subtle.ConstantTimeCompare([]byte(want), []byte(signature)) == 1
}
