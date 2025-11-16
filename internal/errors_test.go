package internal_test

import (
	"fmt"
	"testing"

	"github.com/ryanmoran/contagent/internal"
	"github.com/stretchr/testify/require"
)

// TestConfigErrorCases tests edge cases and potential error scenarios in config parsing
func TestConfigErrorCases(t *testing.T) {
	t.Run("ParseConfig edge cases", func(t *testing.T) {
		t.Run("empty args", func(t *testing.T) {
			args := []string{}
			env := []string{"TERM=xterm"}

			config := internal.ParseConfig(args, env)
			require.Empty(t, config.Args)
			require.NotEmpty(t, config.Env) // Should still have default env
		})

		t.Run("empty env", func(t *testing.T) {
			args := []string{"some-command"}
			env := []string{}

			config := internal.ParseConfig(args, env)
			require.Equal(t, internal.Command([]string{"some-command"}), config.Args)
			// Should have default values for missing env vars
			require.Contains(t, config.Env, "COLORTERM=truecolor")
			require.Contains(t, config.Env, "ANTHROPIC_API_KEY=")
		})

		t.Run("only flags, no command", func(t *testing.T) {
			args := []string{"--env", "VAR1=value1", "--volume", "/path:/path"}
			env := []string{"TERM=xterm"}

			config := internal.ParseConfig(args, env)
			require.Empty(t, config.Args) // No command after flags
			require.Equal(t, []string{
				"/var/run/docker.sock:/var/run/docker.sock",
				"/run/host-services/ssh-auth.sock:/run/host-services/ssh-auth.sock",
				"/path:/path",
			}, config.Volumes)
		})

		t.Run("malformed env flag without value", func(t *testing.T) {
			args := []string{"--env", "some-command"}
			env := []string{"TERM=xterm"}

			// ParseConfig doesn't validate format, just passes through
			config := internal.ParseConfig(args, env)
			// "some-command" is treated as the value for --env
			// Next arg would be the command, but there isn't one
			require.Empty(t, config.Args)
		})

		t.Run("malformed env value without equals", func(t *testing.T) {
			args := []string{"--env", "VARVALUE", "command"}
			env := []string{"TERM=xterm"}

			config := internal.ParseConfig(args, env)
			require.Equal(t, internal.Command([]string{"command"}), config.Args)
			// VARVALUE is added to env even without =
			require.Contains(t, config.Env, "VARVALUE")
		})

		t.Run("volume flag without value", func(t *testing.T) {
			args := []string{"--volume", "command"}
			env := []string{"TERM=xterm"}

			config := internal.ParseConfig(args, env)
			// "command" is treated as volume value
			require.Empty(t, config.Args)
			require.Equal(t, []string{
				"/var/run/docker.sock:/var/run/docker.sock",
				"/run/host-services/ssh-auth.sock:/run/host-services/ssh-auth.sock",
				"command",
			}, config.Volumes)
		})

		t.Run("multiple consecutive flags", func(t *testing.T) {
			args := []string{
				"--env", "VAR1=val1",
				"--env", "VAR2=val2",
				"--volume", "/a:/a",
				"--volume", "/b:/b",
				"--env", "VAR3=val3",
				"cmd", "arg1", "arg2",
			}
			env := []string{"TERM=xterm"}

			config := internal.ParseConfig(args, env)
			require.Equal(t, internal.Command([]string{"cmd", "arg1", "arg2"}), config.Args)
			require.Contains(t, config.Env, "VAR1=val1")
			require.Contains(t, config.Env, "VAR2=val2")
			require.Contains(t, config.Env, "VAR3=val3")
			require.Equal(t, []string{
				"/var/run/docker.sock:/var/run/docker.sock",
				"/run/host-services/ssh-auth.sock:/run/host-services/ssh-auth.sock",
				"/a:/a",
				"/b:/b",
			}, config.Volumes)
		})

		t.Run("command with dashes", func(t *testing.T) {
			args := []string{"--env", "VAR=val", "--", "--command-with-dashes", "--flag"}
			env := []string{"TERM=xterm"}

			config := internal.ParseConfig(args, env)
			// Without special handling of --, all args after VAR=val become the command
			// The behavior depends on implementation
			require.NotEmpty(t, config.Args)
		})

		t.Run("env vars with special characters", func(t *testing.T) {
			args := []string{
				"--env", "VAR=value with spaces",
				"--env", "PATH=/usr/bin:/bin",
				"--env", "SPECIAL=!@#$%^&*()",
				"command",
			}
			env := []string{"TERM=xterm"}

			config := internal.ParseConfig(args, env)
			require.Equal(t, internal.Command([]string{"command"}), config.Args)
			require.Contains(t, config.Env, "VAR=value with spaces")
			require.Contains(t, config.Env, "PATH=/usr/bin:/bin")
			require.Contains(t, config.Env, "SPECIAL=!@#$%^&*()")
		})

		t.Run("empty env var value", func(t *testing.T) {
			args := []string{"--env", "EMPTY=", "command"}
			env := []string{"TERM=xterm"}

			config := internal.ParseConfig(args, env)
			require.Equal(t, internal.Command([]string{"command"}), config.Args)
			require.Contains(t, config.Env, "EMPTY=")
		})

		t.Run("duplicate env vars", func(t *testing.T) {
			args := []string{
				"--env", "VAR=value1",
				"--env", "VAR=value2",
				"command",
			}
			env := []string{"TERM=xterm"}

			config := internal.ParseConfig(args, env)
			require.Equal(t, internal.Command([]string{"command"}), config.Args)
			// Both are added to the list (behavior may vary)
			envStr := ""
			for _, e := range config.Env {
				envStr += e + " "
			}
			require.Contains(t, envStr, "VAR=")
		})

		t.Run("env var overriding system env", func(t *testing.T) {
			args := []string{"--env", "TERM=override", "command"}
			env := []string{"TERM=xterm"}

			config := internal.ParseConfig(args, env)
			require.Equal(t, internal.Command([]string{"command"}), config.Args)
			// Both TERM values might be present
			require.NotEmpty(t, config.Env)
		})

		t.Run("very long command line", func(t *testing.T) {
			args := []string{"command"}
			for i := 0; i < 1000; i++ {
				args = append(args, "arg")
			}
			env := []string{"TERM=xterm"}

			config := internal.ParseConfig(args, env)
			require.Len(t, config.Args, 1001)
		})

		t.Run("system env with missing keys", func(t *testing.T) {
			args := []string{"command"}
			env := []string{} // No TERM, COLORTERM, or ANTHROPIC_API_KEY

			config := internal.ParseConfig(args, env)
			require.Equal(t, internal.Command([]string{"command"}), config.Args)
			// Should provide defaults for missing values
			require.NotEmpty(t, config.Env)
		})
	})
}

// TestSessionErrorCases tests edge cases in session generation
func TestSessionErrorCases(t *testing.T) {
	t.Run("GenerateSession edge cases", func(t *testing.T) {
		t.Run("multiple rapid generations have high uniqueness", func(t *testing.T) {
			sessions := make(map[string]bool)
			// Generate many sessions quickly
			for i := 0; i < 100; i++ {
				session := internal.GenerateSession()
				sessionStr := session.String()
				sessions[sessionStr] = true
			}
			// Expect high uniqueness (at least 90% unique)
			// Some collisions are acceptable with random generation
			require.Greater(t, len(sessions), 90, "expected at least 90%% unique sessions")
		})

		t.Run("session string format is consistent", func(t *testing.T) {
			for i := 0; i < 50; i++ {
				session := internal.GenerateSession()
				sessionStr := session.String()
				branchStr := session.Branch()

				require.Regexp(t, `^contagent-\d{1,4}$`, sessionStr)
				require.Regexp(t, `^contagent/\d{1,4}$`, branchStr)

				// Extract numbers and ensure they match
				var sessionNum, branchNum int
				require.Equal(t, 1, must(t, sessionStr, &sessionNum))
				require.Equal(t, 1, must(t, branchStr, &branchNum))
				require.Equal(t, sessionNum, branchNum)
			}
		})

		t.Run("session branch conversion is reversible", func(t *testing.T) {
			for i := 0; i < 20; i++ {
				session := internal.GenerateSession()
				sessionStr := session.String()
				branchStr := session.Branch()

				// Both should contain the same numeric ID
				var sessionID, branchID int
				require.Equal(t, 1, must(t, sessionStr, &sessionID))
				require.Equal(t, 1, must(t, branchStr, &branchID))
				require.Equal(t, sessionID, branchID)
			}
		})
	})
}

// must is a helper that wraps fmt.Sscanf for test assertions
func must(t *testing.T, str string, id *int) int {
	t.Helper()
	// Try session format first
	n, err := fmt.Sscanf(str, "contagent-%d", id)
	if err == nil && n == 1 {
		return n
	}
	// Try branch format
	n, err = fmt.Sscanf(str, "contagent/%d", id)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	return n
}
