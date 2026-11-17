package k8s

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/store"
)

const twoSliceRecipe = `App: shop

Web:
  from image nginx:latest
  open to the public on port 80

Worker:
  from image busybox:latest
`

func openCleanupStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func recordSuccessfulDeploy(t *testing.T, s *store.Store, recipeContent string) {
	t.Helper()
	d, err := s.CreateDeploy("shop", "k8s", "", "shop", recipeContent, "test")
	require.NoError(t, err)
	require.NoError(t, s.CompleteDeploy(d.ID, "success", "", 1))
}

func TestDetectRemovedSlices(t *testing.T) {
	t.Parallel()
	current := []plan.Slice{{Name: "web"}}

	t.Run("no previous deploy", func(t *testing.T) {
		t.Parallel()
		removed, err := DetectRemovedSlices(openCleanupStore(t), "shop", current)
		require.NoError(t, err)
		assert.Empty(t, removed)
	})

	t.Run("slice removed since the last deploy", func(t *testing.T) {
		t.Parallel()
		s := openCleanupStore(t)
		recordSuccessfulDeploy(t, s, twoSliceRecipe)

		removed, err := DetectRemovedSlices(s, "shop", current)
		require.NoError(t, err)
		assert.Equal(t, []string{"worker"}, removed)
	})

	t.Run("store error is returned", func(t *testing.T) {
		t.Parallel()
		s := openCleanupStore(t)
		require.NoError(t, s.Close())

		_, err := DetectRemovedSlices(s, "shop", current)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "previous deploy")
	})

	t.Run("unparseable previous recipe is returned", func(t *testing.T) {
		t.Parallel()
		s := openCleanupStore(t)
		recordSuccessfulDeploy(t, s, "App: shop\n\nWeb:\n  from image\n  run \"unterminated\n")

		_, err := DetectRemovedSlices(s, "shop", current)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse the recipe")
	})
}
