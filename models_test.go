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
	assert.Equal(t, "default", models[0].ID)
}
