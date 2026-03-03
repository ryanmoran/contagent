package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExpandEnv(t *testing.T) {
	t.Run("EmptyConfig", func(t *testing.T) {
		cfg := Config{}
		environment := []string{}

		result := ExpandEnv(cfg, environment)

		require.Equal(t, cfg, result)
	})

	t.Run("NoVariablesToExpand", func(t *testing.T) {
		cfg := Config{
			Env: map[string]string{
				"PLAIN": "value",
				"OTHER": "another",
			},
			Volumes: []string{
				"/host:/container",
			},
		}
		environment := []string{"HOME=/home/user"}

		result := ExpandEnv(cfg, environment)

		require.Equal(t, "value", result.Env["PLAIN"])
		require.Equal(t, "another", result.Env["OTHER"])
		require.Equal(t, []string{"/host:/container"}, result.Volumes)
	})

	t.Run("SimpleVariableInEnv", func(t *testing.T) {
		cfg := Config{
			Env: map[string]string{
				"PATH_VAR": "$HOME/bin",
			},
		}
		environment := []string{"HOME=/home/user"}

		result := ExpandEnv(cfg, environment)

		require.Equal(t, "/home/user/bin", result.Env["PATH_VAR"])
	})

	t.Run("BracedVariableInEnv", func(t *testing.T) {
		cfg := Config{
			Env: map[string]string{
				"PATH_VAR": "${HOME}/bin",
			},
		}
		environment := []string{"HOME=/home/user"}

		result := ExpandEnv(cfg, environment)

		require.Equal(t, "/home/user/bin", result.Env["PATH_VAR"])
	})

	t.Run("MultipleVariablesInSingleValue", func(t *testing.T) {
		cfg := Config{
			Env: map[string]string{
				"COMPLEX": "$HOME/path-${USER}-suffix",
			},
		}
		environment := []string{"HOME=/home/user", "USER=alice"}

		result := ExpandEnv(cfg, environment)

		require.Equal(t, "/home/user/path-alice-suffix", result.Env["COMPLEX"])
	})

	t.Run("UndefinedVariable", func(t *testing.T) {
		cfg := Config{
			Env: map[string]string{
				"WITH_UNDEFINED": "$UNDEFINED_VAR/path",
			},
		}
		environment := []string{}

		result := ExpandEnv(cfg, environment)

		require.Equal(t, "/path", result.Env["WITH_UNDEFINED"])
	})

	t.Run("VariablesInVolumes", func(t *testing.T) {
		cfg := Config{
			Volumes: []string{
				"$HOME/data:/data",
				"${HOME}/cache:/cache",
			},
		}
		environment := []string{"HOME=/home/user"}

		result := ExpandEnv(cfg, environment)

		require.Equal(t, "/home/user/data:/data", result.Volumes[0])
		require.Equal(t, "/home/user/cache:/cache", result.Volumes[1])
	})

	t.Run("BothEnvAndVolumes", func(t *testing.T) {
		cfg := Config{
			Env: map[string]string{
				"MY_PATH": "$HOME/bin",
				"MY_USER": "${USER}",
			},
			Volumes: []string{
				"$HOME/data:/data",
				"${HOME}/cache:/cache",
			},
		}
		environment := []string{"HOME=/home/alice", "USER=alice"}

		result := ExpandEnv(cfg, environment)

		require.Equal(t, "/home/alice/bin", result.Env["MY_PATH"])
		require.Equal(t, "alice", result.Env["MY_USER"])
		require.Equal(t, "/home/alice/data:/data", result.Volumes[0])
		require.Equal(t, "/home/alice/cache:/cache", result.Volumes[1])
	})

	t.Run("PreservesOtherFields", func(t *testing.T) {
		cfg := Config{
			Image:       "myimage:latest",
			WorkingDir:  "/workspace",
			Dockerfile:  "./Dockerfile",
			Network:     "custom",
			StopTimeout: 30,
			TTYRetries:  5,
			RetryDelay:  50 * time.Millisecond,
			Git: GitConfig{
				User: GitUserConfig{
					Name:  "Test User",
					Email: "test@example.com",
				},
			},
			Env: map[string]string{
				"VAR": "$HOME",
			},
			Volumes: []string{},
		}
		environment := []string{"HOME=/home/user"}

		result := ExpandEnv(cfg, environment)

		// Check that non-env/volume fields are preserved
		require.Equal(t, "myimage:latest", result.Image)
		require.Equal(t, "/workspace", result.WorkingDir)
		require.Equal(t, "./Dockerfile", result.Dockerfile)
		require.Equal(t, "custom", result.Network)
		require.Equal(t, 30, result.StopTimeout)
		require.Equal(t, 5, result.TTYRetries)
		require.Equal(t, 50*time.Millisecond, result.RetryDelay)
		require.Equal(t, "Test User", result.Git.User.Name)
		require.Equal(t, "test@example.com", result.Git.User.Email)

		// Check that env was expanded
		require.Equal(t, "/home/user", result.Env["VAR"])
	})

	t.Run("ReturnsNewConfig", func(t *testing.T) {
		cfg := Config{
			Env: map[string]string{
				"VAR": "$HOME",
			},
			Volumes: []string{"$HOME/data:/data"},
		}
		environment := []string{"HOME=/home/user"}

		result := ExpandEnv(cfg, environment)

		// Verify original config is unchanged
		require.Equal(t, "$HOME", cfg.Env["VAR"])
		require.Equal(t, "$HOME/data:/data", cfg.Volumes[0])

		// Verify result has expanded values
		require.Equal(t, "/home/user", result.Env["VAR"])
		require.Equal(t, "/home/user/data:/data", result.Volumes[0])
	})
}

func TestExpandHome(t *testing.T) {
	t.Run("ExpandsHomeDirectory", func(t *testing.T) {
		result := expandHome("~/config")

		// Should expand to actual home directory
		home, err := os.UserHomeDir()
		require.NoError(t, err)
		require.Equal(t, filepath.Join(home, "config"), result)
	})

	t.Run("ExpandsHomeDirWithSubpath", func(t *testing.T) {
		result := expandHome("~/projects/myapp")

		home, err := os.UserHomeDir()
		require.NoError(t, err)
		require.Equal(t, filepath.Join(home, "projects/myapp"), result)
	})

	t.Run("DoesNotExpandTildeNotAtStart", func(t *testing.T) {
		result := expandHome("/path/to/~/file")

		require.Equal(t, "/path/to/~/file", result)
	})

	t.Run("DoesNotExpandTildeWithoutSlash", func(t *testing.T) {
		result := expandHome("~file")

		require.Equal(t, "~file", result)
	})

	t.Run("HandlesRootPath", func(t *testing.T) {
		result := expandHome("/absolute/path")

		require.Equal(t, "/absolute/path", result)
	})

	t.Run("HandlesEmptyPath", func(t *testing.T) {
		result := expandHome("")

		require.Equal(t, "", result)
	})
}

func TestExpandEnvWithHomeDirectory(t *testing.T) {
	// Get actual home directory for test assertions
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	t.Run("ExpandsWorkingDir", func(t *testing.T) {
		cfg := Config{
			WorkingDir: "~/workspace",
		}
		environment := []string{}

		result := ExpandEnv(cfg, environment)

		require.Equal(t, filepath.Join(home, "workspace"), result.WorkingDir)
	})

	t.Run("ExpandsDockerfile", func(t *testing.T) {
		cfg := Config{
			Dockerfile: "~/dockerfiles/Dockerfile.dev",
		}
		environment := []string{}

		result := ExpandEnv(cfg, environment)

		require.Equal(t, filepath.Join(home, "dockerfiles/Dockerfile.dev"), result.Dockerfile)
	})

	t.Run("ExpandsHomeInVolumes", func(t *testing.T) {
		cfg := Config{
			Volumes: []string{
				"~/data:/data",
				"~/cache:/cache",
			},
		}
		environment := []string{}

		result := ExpandEnv(cfg, environment)

		require.Equal(t, filepath.Join(home, "data")+":/data", result.Volumes[0])
		require.Equal(t, filepath.Join(home, "cache")+":/cache", result.Volumes[1])
	})

	t.Run("ExpandsHomeAfterEnvVar", func(t *testing.T) {
		cfg := Config{
			Volumes: []string{
				"$HOME/data:/data",
			},
		}
		environment := []string{"HOME=" + home}

		result := ExpandEnv(cfg, environment)

		require.Equal(t, filepath.Join(home, "data")+":/data", result.Volumes[0])
	})

	t.Run("DoesNotExpandNonHomePrefix", func(t *testing.T) {
		cfg := Config{
			WorkingDir: "/workspace",
			Dockerfile: "./Dockerfile",
		}
		environment := []string{}

		result := ExpandEnv(cfg, environment)

		require.Equal(t, "/workspace", result.WorkingDir)
		require.Equal(t, "./Dockerfile", result.Dockerfile)
	})

	t.Run("ExpandsMultiplePathsWithHome", func(t *testing.T) {
		cfg := Config{
			WorkingDir: "~/app",
			Dockerfile: "~/docker/Dockerfile",
			Volumes: []string{
				"~/data:/data",
				"/absolute:/absolute",
			},
		}
		environment := []string{}

		result := ExpandEnv(cfg, environment)

		require.Equal(t, filepath.Join(home, "app"), result.WorkingDir)
		require.Equal(t, filepath.Join(home, "docker/Dockerfile"), result.Dockerfile)
		require.Equal(t, filepath.Join(home, "data")+":/data", result.Volumes[0])
		require.Equal(t, "/absolute:/absolute", result.Volumes[1])
	})
}
