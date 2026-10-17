package requestid

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// SPEC.md 17.3: one request has one id, carried in its context, so the error body, every log
// line and the recorded usage row of the request name the same id.
func TestResolve_KeepsASafeClientIDAndMintsOtherwise(t *testing.T) {
	assert.Equal(t, "req-12345", Resolve("req-12345"))
	assert.Equal(t, "a.b_c-9", Resolve("a.b_c-9"))
	for _, unsafe := range []string{"", "has space", "new\nline", strings.Repeat("x", 65), "quote\"", "émoji"} {
		got := Resolve(unsafe)
		assert.NotEqual(t, unsafe, got, "an unsafe client id is replaced: %q", unsafe)
		assert.Regexp(t, `^[0-9a-f-]{36}$`, got, "a minted id is a UUID")
	}
	assert.NotEqual(t, Resolve(""), Resolve(""), "each minted id is new")
}

func TestWithAndFrom_CarryTheIDInTheContext(t *testing.T) {
	assert.Equal(t, "", From(context.Background()), "a context without an id has none")
	ctx := With(context.Background(), "req-1")
	assert.Equal(t, "req-1", From(ctx))
	assert.Equal(t, "req-2", From(With(ctx, "req-2")), "the innermost id wins")
}
