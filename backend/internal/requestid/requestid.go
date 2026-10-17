// Package requestid carries the id of an incoming request in its context (SPEC.md 17.3).
//
// One request has one id. The HTTP boundary resolves it once (requestIDMiddleware) and puts it
// in the request's context and its X-Request-ID header, so the error body, every log line and
// the recorded usage row of that request all name the same id.
package requestid

import (
	"context"
	"regexp"

	"github.com/google/uuid"
)

// ctxKey is the context key of the request id. Only this package reads or writes it.
type ctxKey struct{}

// safeID is an X-Request-ID a client may choose: short, and made only of characters that need no
// escaping in a log line, a header or a URL.
var safeID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// Resolve returns the client's X-Request-ID when it is safe, and a new UUID otherwise. A client
// that sends its own id (a retry, a proxy) keeps it, so the request is counted once.
func Resolve(clientID string) string {
	if safeID.MatchString(clientID) {
		return clientID
	}
	return uuid.New().String()
}

// With returns a copy of ctx that carries id.
func With(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// From returns the request id ctx carries, or "" when it carries none.
func From(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}
