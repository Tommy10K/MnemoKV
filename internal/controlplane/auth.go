// Package controlplane contains the narrow authentication and stable-fencing
// primitives shared by the automatic controller and data-node admin API.
// It does not own cluster topology or application data.
package controlplane

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

const (
	ControlIndexHeader     = "X-MnemoKV-Control-Index"
	ControlSignatureHeader = "X-MnemoKV-Control-Signature"
)

func Sign(secret []byte, method, path string, body []byte, index string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(message(method, path, body, index))
	return hex.EncodeToString(mac.Sum(nil))
}

func Verify(secret []byte, method, path string, body []byte, index, signature string) bool {
	provided, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(message(method, path, body, index))
	return hmac.Equal(provided, mac.Sum(nil))
}

func OperationDigest(method, path string, body []byte, index string) [sha256.Size]byte {
	return sha256.Sum256(message(method, path, body, index))
}

func message(method, path string, body []byte, index string) []byte {
	message := make([]byte, 0, len(method)+len(path)+len(index)+len(body)+3)
	message = append(message, method...)
	message = append(message, '\n')
	message = append(message, path...)
	message = append(message, '\n')
	message = append(message, index...)
	message = append(message, '\n')
	message = append(message, body...)
	return message
}
