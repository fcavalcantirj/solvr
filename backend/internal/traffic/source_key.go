package traffic

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// PageViewSourceKey names the source of a page view for one UTC day without naming the
// visitor (SPEC.md 27.6). It is HMAC-SHA256 over the address and the user agent, keyed by the
// web report secret and the day, cut to 16 hex characters: the same source gives the same key
// within a day, so the burst rule (db.PageViewBurstThreshold) can count its views in a
// minute; the next day it gives another, so two days of one visitor cannot be joined. The
// address and the user agent themselves are never stored. With no address there is no key.
func PageViewSourceKey(secret []byte, at time.Time, clientIP, userAgent string) string {
	ip := strings.TrimSpace(clientIP)
	if ip == "" {
		return ""
	}
	mac := hmac.New(sha256.New, append(append([]byte{}, secret...), []byte(at.UTC().Format("2006-01-02"))...))
	mac.Write([]byte(ip))
	mac.Write([]byte{0})
	mac.Write([]byte(userAgent))
	return hex.EncodeToString(mac.Sum(nil))[:16]
}
