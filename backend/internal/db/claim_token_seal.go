package db

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// claimTokenSealDomain separates the sealing key from every other key derived from the
// same server secret (the JWT signing key, the stream-ticket key).
const claimTokenSealDomain = "solvr/claim-token-seal/v1"

var errClaimTokenUnsealable = errors.New("claim token cannot be unsealed")

// hashClaimToken is the SHA-256 hex of a claim token: how a row is looked up. Migration
// 000108 backfills existing rows with the same function in SQL.
func hashClaimToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// claimTokenSealer keeps a claim token recoverable by the servers and unreadable from the
// database alone. A repeat POST /v1/agents/me/claim hands the same link back, and the
// database stores only a hash, so the token is kept AES-256-GCM sealed under a key derived
// from the server secret. The row's hash is bound in as additional data, so a sealed copy
// cannot be moved to another row. A nil sealer (no secret configured) seals nothing.
type claimTokenSealer struct{ aead cipher.AEAD }

func newClaimTokenSealer(secret string) *claimTokenSealer {
	if secret == "" {
		return nil
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(claimTokenSealDomain))
	block, _ := aes.NewCipher(mac.Sum(nil)) // a 32-byte key: cannot fail
	aead, _ := cipher.NewGCM(block)         // the standard nonce size: cannot fail
	return &claimTokenSealer{aead: aead}
}

// seal returns nonce||ciphertext, or nil when there is no key.
func (s *claimTokenSealer) seal(token, hash string) ([]byte, error) {
	if s == nil {
		return nil, nil
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return s.aead.Seal(nonce, nonce, []byte(token), []byte(hash)), nil
}

// open returns the token a seal holds, or an error for a foreign key, a foreign row, a
// damaged blob or no key at all.
func (s *claimTokenSealer) open(sealed []byte, hash string) (string, error) {
	if s == nil || len(sealed) < s.aead.NonceSize()+s.aead.Overhead() {
		return "", errClaimTokenUnsealable
	}
	nonce, ciphertext := sealed[:s.aead.NonceSize()], sealed[s.aead.NonceSize():]
	token, err := s.aead.Open(nil, nonce, ciphertext, []byte(hash))
	if err != nil {
		return "", errClaimTokenUnsealable
	}
	return string(token), nil
}
