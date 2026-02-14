package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMerge(t *testing.T) {
	t.Run("empty override returns base unchanged", func(t *testing.T) {
		base := Config{
			Image:       "base-image",
			WorkingDir:  "/base",
			Dockerfile:  "Dockerfile.base",
			Network:     "base-network",
			StopTimeout: 5,
			TTYRetries:  3,
			RetryDelay:  5 * time.Millisecond,
			Git: GitConfig{
				User: GitUserConfig{
					Name:  "Base User",
					Email: "base@example.com",
				},
			},
			Env: map[string]string{
				"BASE_KEY": "base_value",
			},
			Volumes: []string{"/base/volume"},
		}

		override := Config{}

		result := Merge(base, override)

		require.Equal(t, "base-image", result.Image)
		require.Equal(t, "/base", result.WorkingDir)
		require.Equal(t, "Dockerfile.base", result.Dockerfile)
		require.Equal(t, "base-network", result.Network)
		require.Equal(t, 5, result.StopTimeout)
		require.Equal(t, 3, result.TTYRetries)
		require.Equal(t, 5*time.Millisecond, result.RetryDelay)
		require.Equal(t, "Base User", result.Git.User.Name)
		require.Equal(t, "base@example.com", result.Git.User.Email)
		require.Equal(t, map[string]string{"BASE_KEY": "base_value"}, result.Env)
		require.Equal(t, []string{"/base/volume"}, result.Volumes)
	})

	t.Run("override replaces scalar fields", func(t *testing.T) {
		base := Config{
			Image:       "base-image",
			WorkingDir:  "/base",
			Dockerfile:  "Dockerfile.base",
			Network:     "base-network",
			StopTimeout: 5,
			TTYRetries:  3,
			RetryDelay:  5 * time.Millisecond,
			Git: GitConfig{
				User: GitUserConfig{
					Name:  "Base User",
					Email: "base@example.com",
				},
			},
		}

		override := Config{
			Image:       "override-image",
			WorkingDir:  "/override",
			Dockerfile:  "Dockerfile.override",
			Network:     "override-network",
			StopTimeout: 10,
			TTYRetries:  7,
			RetryDelay:  10 * time.Millisecond,
			Git: GitConfig{
				User: GitUserConfig{
					Name:  "Override User",
					Email: "override@example.com",
				},
			},
		}

		result := Merge(base, override)

		require.Equal(t, "override-image", result.Image)
		require.Equal(t, "/override", result.WorkingDir)
		require.Equal(t, "Dockerfile.override", result.Dockerfile)
		require.Equal(t, "override-network", result.Network)
		require.Equal(t, 10, result.StopTimeout)
		require.Equal(t, 7, result.TTYRetries)
		require.Equal(t, 10*time.Millisecond, result.RetryDelay)
		require.Equal(t, "Override User", result.Git.User.Name)
		require.Equal(t, "override@example.com", result.Git.User.Email)
	})

	t.Run("partial override only replaces specified fields", func(t *testing.T) {
		base := Config{
			Image:       "base-image",
			WorkingDir:  "/base",
			StopTimeout: 5,
			Git: GitConfig{
				User: GitUserConfig{
					Name:  "Base User",
					Email: "base@example.com",
				},
			},
		}

		override := Config{
			Image: "override-image",
			Git: GitConfig{
				User: GitUserConfig{
					Email: "override@example.com",
				},
			},
		}

		result := Merge(base, override)

		require.Equal(t, "override-image", result.Image)
		require.Equal(t, "/base", result.WorkingDir, "WorkingDir should remain from base")
		require.Equal(t, 5, result.StopTimeout, "StopTimeout should remain from base")
		require.Equal(t, "Base User", result.Git.User.Name, "Git.User.Name should remain from base")
		require.Equal(t, "override@example.com", result.Git.User.Email)
	})

	t.Run("env maps are merged with override precedence", func(t *testing.T) {
		base := Config{
			Env: map[string]string{
				"KEY1": "base1",
				"KEY2": "base2",
				"KEY3": "base3",
			},
		}

		override := Config{
			Env: map[string]string{
				"KEY2": "override2",
				"KEY4": "override4",
			},
		}

		result := Merge(base, override)

		expected := map[string]string{
			"KEY1": "base1",
			"KEY2": "override2", // Override wins
			"KEY3": "base3",
			"KEY4": "override4",
		}

		require.Equal(t, expected, result.Env)
	})

	t.Run("volumes are appended", func(t *testing.T) {
		base := Config{
			Volumes: []string{"/base/vol1", "/base/vol2"},
		}

		override := Config{
			Volumes: []string{"/override/vol1", "/override/vol2"},
		}

		result := Merge(base, override)

		expected := []string{"/base/vol1", "/base/vol2", "/override/vol1", "/override/vol2"}
		require.Equal(t, expected, result.Volumes)
	})

	t.Run("empty base with override", func(t *testing.T) {
		base := Config{}

		override := Config{
			Image:       "override-image",
			WorkingDir:  "/override",
			StopTimeout: 10,
			Env: map[string]string{
				"KEY": "value",
			},
			Volumes: []string{"/vol"},
		}

		result := Merge(base, override)

		require.Equal(t, "override-image", result.Image)
		require.Equal(t, "/override", result.WorkingDir)
		require.Equal(t, 10, result.StopTimeout)
		require.Equal(t, map[string]string{"KEY": "value"}, result.Env)
		require.Equal(t, []string{"/vol"}, result.Volumes)
	})

	t.Run("both empty returns empty", func(t *testing.T) {
		base := Config{}
		override := Config{}

		result := Merge(base, override)

		require.Equal(t, "", result.Image)
		require.Equal(t, "", result.WorkingDir)
		require.Equal(t, 0, result.StopTimeout)
		require.NotNil(t, result.Env, "Env should be non-nil empty map")
		require.Empty(t, result.Env, "Env should be empty")
		require.Nil(t, result.Volumes)
	})

	t.Run("nil env and volumes handling", func(t *testing.T) {
		base := Config{
			Image: "base-image",
		}

		override := Config{
			Image: "override-image",
			Env: map[string]string{
				"KEY": "value",
			},
			Volumes: []string{"/vol"},
		}

		result := Merge(base, override)

		require.Equal(t, "override-image", result.Image)
		require.Equal(t, map[string]string{"KEY": "value"}, result.Env)
		require.Equal(t, []string{"/vol"}, result.Volumes)
	})
}

func TestMergeEnv(t *testing.T) {
	t.Run("empty base and override returns empty", func(t *testing.T) {
		base := map[string]string{}
		override := map[string]string{}

		result := MergeEnv(base, override)

		require.Empty(t, result)
		require.NotNil(t, result, "should return non-nil map")
	})

	t.Run("nil base and override returns empty", func(t *testing.T) {
		result := MergeEnv(nil, nil)

		require.Empty(t, result)
		require.NotNil(t, result, "should return non-nil map")
	})

	t.Run("only base returns base copy", func(t *testing.T) {
		base := map[string]string{
			"KEY1": "value1",
			"KEY2": "value2",
		}

		result := MergeEnv(base, nil)

		require.Equal(t, base, result)
		// Verify it's a copy, not the same map
		result["KEY3"] = "value3"
		require.NotContains(t, base, "KEY3", "original base should not be modified")
	})

	t.Run("only override returns override copy", func(t *testing.T) {
		override := map[string]string{
			"KEY1": "value1",
			"KEY2": "value2",
		}

		result := MergeEnv(nil, override)

		require.Equal(t, override, result)
		// Verify it's a copy, not the same map
		result["KEY3"] = "value3"
		require.NotContains(t, override, "KEY3", "original override should not be modified")
	})

	t.Run("override keys win over base keys", func(t *testing.T) {
		base := map[string]string{
			"KEY1": "base1",
			"KEY2": "base2",
			"KEY3": "base3",
		}

		override := map[string]string{
			"KEY2": "override2",
			"KEY4": "override4",
		}

		result := MergeEnv(base, override)

		expected := map[string]string{
			"KEY1": "base1",
			"KEY2": "override2", // Override wins
			"KEY3": "base3",
			"KEY4": "override4",
		}

		require.Equal(t, expected, result)
	})

	t.Run("all keys are unique", func(t *testing.T) {
		base := map[string]string{
			"KEY1": "base1",
			"KEY2": "base2",
		}

		override := map[string]string{
			"KEY3": "override3",
			"KEY4": "override4",
		}

		result := MergeEnv(base, override)

		expected := map[string]string{
			"KEY1": "base1",
			"KEY2": "base2",
			"KEY3": "override3",
			"KEY4": "override4",
		}

		require.Equal(t, expected, result)
	})

	t.Run("empty string values are preserved", func(t *testing.T) {
		base := map[string]string{
			"KEY1": "value1",
			"KEY2": "",
		}

		override := map[string]string{
			"KEY3": "",
		}

		result := MergeEnv(base, override)

		expected := map[string]string{
			"KEY1": "value1",
			"KEY2": "",
			"KEY3": "",
		}

		require.Equal(t, expected, result)
	})

	t.Run("override with empty string replaces base value", func(t *testing.T) {
		base := map[string]string{
			"KEY1": "value1",
		}

		override := map[string]string{
			"KEY1": "",
		}

		result := MergeEnv(base, override)

		expected := map[string]string{
			"KEY1": "", // Override wins even with empty string
		}

		require.Equal(t, expected, result)
	})

	t.Run("does not modify original maps", func(t *testing.T) {
		base := map[string]string{
			"KEY1": "base1",
		}

		override := map[string]string{
			"KEY2": "override2",
		}

		baseCopy := make(map[string]string)
		overrideCopy := make(map[string]string)
		for k, v := range base {
			baseCopy[k] = v
		}
		for k, v := range override {
			overrideCopy[k] = v
		}

		MergeEnv(base, override)

		require.Equal(t, baseCopy, base, "base should not be modified")
		require.Equal(t, overrideCopy, override, "override should not be modified")
	})
}
