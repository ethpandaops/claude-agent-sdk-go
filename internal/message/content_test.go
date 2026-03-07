package message

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnmarshalContentBlock_MissingTypeReturnsError(t *testing.T) {
	t.Parallel()

	_, err := UnmarshalContentBlock([]byte(`{"foo":"bar"}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing or invalid")
}

func TestUnmarshalContentBlock_UnknownTypeRoundTripsOriginalPayload(t *testing.T) {
	t.Parallel()

	block, err := UnmarshalContentBlock([]byte(`{"type":"future_block","foo":"bar"}`))
	require.NoError(t, err)

	encoded, err := json.Marshal(block)
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"future_block","foo":"bar"}`, string(encoded))
}
