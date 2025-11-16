package internal_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ryanmoran/contagent/internal"
)

func TestSession(t *testing.T) {
	setup := func(t *testing.T) internal.Session {
		t.Helper()

		return internal.GenerateSession()
	}

	t.Run("GenerateSession", func(t *testing.T) {
		t.Run("generates unique sessions", func(t *testing.T) {
			sessions := make(map[string]struct{})
			iterations := 1000

			for range iterations {
				session := setup(t)
				sessions[session.String()] = struct{}{}
			}

			require.Greater(t, float64(len(sessions)), float64(iterations)*0.9, "expected high uniqueness in session generation")
		})

		t.Run("generates session IDs within valid range", func(t *testing.T) {
			for range 100 {
				session := setup(t)

				require.Regexp(t, `^contagent-\d+$`, session.String())
				require.Regexp(t, `^contagent/\d+$`, session.Branch())
			}
		})

		t.Run("session ID is less than 10000", func(t *testing.T) {
			for range 100 {
				session := setup(t)
				sessionStr := session.String()

				var id int64
				n, err := fmt.Sscanf(sessionStr, "contagent-%d", &id)
				require.NoError(t, err)
				require.Equal(t, 1, n)
				require.GreaterOrEqual(t, id, int64(0))
				require.Less(t, id, int64(10000))
			}
		})
	})

	t.Run("String", func(t *testing.T) {
		t.Run("returns formatted session ID", func(t *testing.T) {
			session := setup(t)
			sessionStr := session.String()

			require.Contains(t, sessionStr, "contagent-")
			require.Greater(t, len(sessionStr), len("contagent-"))
		})
	})

	t.Run("Branch", func(t *testing.T) {
		t.Run("returns formatted branch name", func(t *testing.T) {
			session := setup(t)
			branchName := session.Branch()

			require.Contains(t, branchName, "contagent/")
			require.Greater(t, len(branchName), len("contagent/"))
		})
	})
}
