package claudesdk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListModels(t *testing.T) {
	t.Parallel()

	models, err := ListModels(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, models)
	assert.Equal(t, "claude-opus-4-6", models[0].ID)
}
