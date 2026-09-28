package db

import (
	"bytes"
	"testing"
)

// A claim token is a credential that lets whoever holds it become the human behind an
// agent. The database keeps only its SHA-256 (to look it up) and a sealed copy (so a repeat
// request can hand the same link back); the clear value is never stored.

func TestHashClaimToken_IsTheSHA256Hex(t *testing.T) {
	// The NIST vector for "abc"; the migration backfills with the same function in SQL.
	const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := hashClaimToken("abc"); got != want {
		t.Fatalf("hashClaimToken(abc) = %s, want %s", got, want)
	}
}

func TestClaimTokenSealer_RoundTripsAndHidesTheToken(t *testing.T) {
	s := newClaimTokenSealer("server-secret")
	token := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	hash := hashClaimToken(token)

	sealed, err := s.seal(token, hash)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if bytes.Contains(sealed, []byte(token)) || bytes.Contains(sealed, []byte(token[:16])) {
		t.Fatal("the sealed copy contains the clear token")
	}
	got, err := s.open(sealed, hash)
	if err != nil || got != token {
		t.Fatalf("open = %q, %v; want the token back", got, err)
	}

	again, err := s.seal(token, hash)
	if err != nil {
		t.Fatalf("seal again: %v", err)
	}
	if bytes.Equal(sealed, again) {
		t.Fatal("two seals of one token are identical: the nonce is not fresh")
	}
}

func TestClaimTokenSealer_RefusesAnythingButItsOwnSealForItsOwnRow(t *testing.T) {
	token := "a-claim-token"
	hash := hashClaimToken(token)
	sealed, err := newClaimTokenSealer("server-secret").seal(token, hash)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}

	cases := []struct {
		name   string
		sealer *claimTokenSealer
		sealed []byte
		hash   string
	}{
		{"another server secret", newClaimTokenSealer("another-secret"), sealed, hash},
		{"another row (the hash is bound in)", newClaimTokenSealer("server-secret"), sealed, hashClaimToken("someone else")},
		{"a flipped byte", newClaimTokenSealer("server-secret"), flipLastByte(sealed), hash},
		{"a truncated blob", newClaimTokenSealer("server-secret"), sealed[:5], hash},
		{"no blob", newClaimTokenSealer("server-secret"), nil, hash},
		{"no key configured", newClaimTokenSealer(""), sealed, hash},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := tc.sealer.open(tc.sealed, tc.hash); err == nil {
				t.Fatalf("open succeeded with %q; it must fail", got)
			}
		})
	}
}

func TestClaimTokenSealer_NoKeyMeansNothingIsSealed(t *testing.T) {
	s := newClaimTokenSealer("")
	sealed, err := s.seal("a-claim-token", hashClaimToken("a-claim-token"))
	if err != nil || sealed != nil {
		t.Fatalf("seal without a key = %v, %v; want nil, nil (hash only, never a clear fallback)", sealed, err)
	}
}

func flipLastByte(b []byte) []byte {
	out := append([]byte(nil), b...)
	out[len(out)-1] ^= 0xff
	return out
}
