package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertLegacyDependencyGone fails unless key ("code:<file>" or "typeref:<file>") is neither
// discovered in the source tree nor listed in the registry: the legacy code it named was
// deleted with the legacy tables (idx 68).
func assertLegacyDependencyGone(t *testing.T, key string) {
	t.Helper()
	_, listed := LegacyDependencyDispositions[key]
	assert.False(t, listed, "%s: deleted legacy code keeps no disposition", key)
	src, err := ScanLegacySourceDependencies(backendRoot(t))
	require.NoError(t, err)
	refs, err := ScanLegacyPostTypeReferences(backendRoot(t))
	require.NoError(t, err)
	for _, d := range append(src, refs...) {
		assert.NotEqual(t, key, d.Key, "%s names no legacy table or type any more", key)
	}
}
