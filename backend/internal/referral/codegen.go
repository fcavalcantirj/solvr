// Package referral provides referral code generation and utilities.
package referral

import (
	"crypto/rand"
	"math/big"
)

// charset is the set of characters used for referral codes: A-Z0-9.
// ~1.7 trillion combinations (36^8) — collision probability negligible.
const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// CodeLength is the length of generated referral codes.
const CodeLength = 8

// GenerateCode generates a cryptographically random 8-character
// alphanumeric referral code using crypto/rand.
func GenerateCode() (string, error) {
	code := make([]byte, CodeLength)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		code[i] = charset[n.Int64()]
	}
	return string(code), nil
}
