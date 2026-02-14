package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoad_WithDefaultsOnly(t *testing.T) {
	// No CLI args, no environment variables
	cfg, args, err := Load([]string{}, []string{}, t.TempDir())
	require.NoError(t, err)
	require.Empty(t, args)

	// Verify hardcoded defaults are set
	require.Equal(t, "contagent:latest", cfg.Image)
	require.Equal(t, "/app", cfg.WorkingDir)
	require.Equal(t, "", cfg.Dockerfile) // No default for dockerfile
	require.Equal(t, "default", cfg.Network)
	require.Equal(t, 10, cfg.StopTimeout)
	require.Equal(t, 10, cfg.TTYRetries)
	require.Equal(t, 10*time.Millisecond, cfg.RetryDelay)
	require.Equal(t, "Contagent", cfg.Git.User.Name)
	require.Equal(t, "contagent@example.com", cfg.Git.User.Email)
	require.NotNil(t, cfg.Env)
	require.NotNil(t, cfg.Volumes)
}

func TestLoad_WithCLIFlags(t *testing.T) {
	args := []string{
		"--dockerfile", "./Dockerfile.dev",
		"--image", "myapp:v1",
		"--working-dir", "/workspace",
		"--network", "custom-network",
		"--env", "FOO=bar",
		"--env", "BAZ=qux",
		"--volume", "/host:/container",
		"--volume", "/data:/data",
	}

	cfg, programArgs, err := Load(args, []string{}, t.TempDir())
	require.NoError(t, err)
	require.Empty(t, programArgs)

	// CLI flags should override defaults
	require.Equal(t, "myapp:v1", cfg.Image)
	require.Equal(t, "/workspace", cfg.WorkingDir)
	require.Equal(t, "./Dockerfile.dev", cfg.Dockerfile)
	require.Equal(t, "custom-network", cfg.Network)

	// Defaults should still be present for non-overridden values
	require.Equal(t, 10, cfg.StopTimeout)
	require.Equal(t, 10, cfg.TTYRetries)
	require.Equal(t, 10*time.Millisecond, cfg.RetryDelay)

	// Env variables should be parsed
	require.Equal(t, "bar", cfg.Env["FOO"])
	require.Equal(t, "qux", cfg.Env["BAZ"])

	// Volumes should be added
	require.Contains(t, cfg.Volumes, "/host:/container")
	require.Contains(t, cfg.Volumes, "/data:/data")
}

func TestLoad_WithGitUserFlags(t *testing.T) {
	args := []string{
		"--git-user-name", "Alice",
		"--git-user-email", "alice@example.com",
	}

	cfg, programArgs, err := Load(args, []string{}, t.TempDir())
	require.NoError(t, err)
	require.Empty(t, programArgs)

	require.Equal(t, "Alice", cfg.Git.User.Name)
	require.Equal(t, "alice@example.com", cfg.Git.User.Email)
}

func TestLoad_WithNumericFlags(t *testing.T) {
	args := []string{
		"--stop-timeout", "30",
		"--tty-retries", "5",
		"--retry-delay", "50ms",
	}

	cfg, programArgs, err := Load(args, []string{}, t.TempDir())
	require.NoError(t, err)
	require.Empty(t, programArgs)

	require.Equal(t, 30, cfg.StopTimeout)
	require.Equal(t, 5, cfg.TTYRetries)
	require.Equal(t, 50*time.Millisecond, cfg.RetryDelay)
}

func TestLoad_WithEnvironmentVariableExpansion(t *testing.T) {
	args := []string{
		"--env", "MY_PATH=$HOME/bin",
		"--env", "USER_DIR=${HOME}/${USER}",
		"--volume", "$HOME/data:/data",
		"--volume", "${HOME}/cache:/cache",
	}
	environment := []string{
		"HOME=/home/alice",
		"USER=alice",
	}

	cfg, programArgs, err := Load(args, environment, t.TempDir())
	require.NoError(t, err)
	require.Empty(t, programArgs)

	// Environment variables should be expanded
	require.Equal(t, "/home/alice/bin", cfg.Env["MY_PATH"])
	require.Equal(t, "/home/alice/alice", cfg.Env["USER_DIR"])

	// Volumes should have expanded paths
	require.Contains(t, cfg.Volumes, "/home/alice/data:/data")
	require.Contains(t, cfg.Volumes, "/home/alice/cache:/cache")
}

func TestLoad_WithInvalidRetryDelay(t *testing.T) {
	args := []string{
		"--retry-delay", "invalid-duration",
	}

	cfg, programArgs, err := Load(args, []string{}, t.TempDir())
	require.Error(t, err)
	require.Contains(t, err.Error(), "time: invalid duration")
	require.Equal(t, Config{}, cfg)
	require.Nil(t, programArgs)
}

func TestLoad_WithInvalidEnvFormat(t *testing.T) {
	// Environment variables without '=' should be ignored
	args := []string{
		"--env", "VALID=value",
		"--env", "INVALID_NO_EQUALS",
		"--env", "ALSO_VALID=another",
	}

	cfg, programArgs, err := Load(args, []string{}, t.TempDir())
	require.NoError(t, err)
	require.Empty(t, programArgs)

	// Only valid entries should be parsed
	require.Equal(t, "value", cfg.Env["VALID"])
	require.Equal(t, "another", cfg.Env["ALSO_VALID"])
	require.NotContains(t, cfg.Env, "INVALID_NO_EQUALS")
}

func TestLoad_WithEmptyArgs(t *testing.T) {
	// Verify flag parsing handles empty args without panicking
	cfg, programArgs, err := Load([]string{}, []string{}, t.TempDir())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Empty(t, programArgs)
}

func TestLoad_WithTrailingArgs(t *testing.T) {
	// Trailing args after flags should be captured as program args
	args := []string{
		"--image", "myapp:v1",
		"--dockerfile", "Dockerfile",
		"bash", // trailing command arg
		"-c",   // another trailing arg
		"echo hello",
	}

	cfg, programArgs, err := Load(args, []string{}, t.TempDir())
	require.NoError(t, err)
	require.Equal(t, "myapp:v1", cfg.Image)
	require.Equal(t, "Dockerfile", cfg.Dockerfile)
	require.Equal(t, []string{"bash", "-c", "echo hello"}, programArgs)
}

func TestStringSlice_String(t *testing.T) {
	s := stringSlice{"a", "b", "c"}
	require.Equal(t, "a,b,c", s.String())
}

func TestStringSlice_Set(t *testing.T) {
	var s stringSlice
	err := s.Set("first")
	require.NoError(t, err)
	require.Equal(t, stringSlice{"first"}, s)

	err = s.Set("second")
	require.NoError(t, err)
	require.Equal(t, stringSlice{"first", "second"}, s)
}
