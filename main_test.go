package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	t.Run("returns error when no arguments provided", func(t *testing.T) {
		err := run([]string{"contagent", "bash", "-c", "sleep 1 && env"}, nil)
		require.NoError(t, err)
	})
}
