package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFindGlobalConfig(t *testing.T) {
	t.Run("with CONTAGENT_GLOBAL_CONFIG_FILE set and file exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "custom-config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("image: test"), 0644))

		env := []string{"CONTAGENT_GLOBAL_CONFIG_FILE=" + configPath}
		path, err := FindGlobalConfig(env)
		require.NoError(t, err)
		require.Equal(t, configPath, path)
	})

	t.Run("with CONTAGENT_GLOBAL_CONFIG_FILE set but file does not exist", func(t *testing.T) {
		env := []string{"CONTAGENT_GLOBAL_CONFIG_FILE=/nonexistent/config.yaml"}
		path, err := FindGlobalConfig(env)
		require.Error(t, err)
		require.Contains(t, err.Error(), "file does not exist")
		require.Empty(t, path)
	})

	t.Run("with CONTAGENT_GLOBAL_CONFIG_FILE set but stat fails with permission error", func(t *testing.T) {
		// This test is hard to reliably trigger cross-platform without root/admin,
		// so we'll skip a deep test and focus on the IsNotExist path above.
		// The non-IsNotExist error path at line 28 would be triggered by permission errors.
		t.Skip("Permission-based stat errors are difficult to test reliably cross-platform")
	})

	t.Run("with XDG_CONFIG_HOME set and file exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, "contagent")
		require.NoError(t, os.MkdirAll(configDir, 0755))
		configPath := filepath.Join(configDir, "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("image: test"), 0644))

		env := []string{"XDG_CONFIG_HOME=" + tmpDir}
		path, err := FindGlobalConfig(env)
		require.NoError(t, err)
		require.Equal(t, configPath, path)
	})

	t.Run("with XDG_CONFIG_HOME set but file does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		env := []string{"XDG_CONFIG_HOME=" + tmpDir}
		path, err := FindGlobalConfig(env)
		require.NoError(t, err)
		require.Empty(t, path)
	})

	t.Run("with HOME set and default config exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, ".config", "contagent")
		require.NoError(t, os.MkdirAll(configDir, 0755))
		configPath := filepath.Join(configDir, "config.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("image: test"), 0644))

		env := []string{"HOME=" + tmpDir}
		path, err := FindGlobalConfig(env)
		require.NoError(t, err)
		require.Equal(t, configPath, path)
	})

	t.Run("with HOME set but default config does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		env := []string{"HOME=" + tmpDir}
		path, err := FindGlobalConfig(env)
		require.NoError(t, err)
		require.Empty(t, path)
	})

	t.Run("without HOME set", func(t *testing.T) {
		env := []string{}
		path, err := FindGlobalConfig(env)
		require.NoError(t, err)
		require.Empty(t, path)
	})

	t.Run("precedence order", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create all three config locations
		customPath := filepath.Join(tmpDir, "custom.yaml")
		require.NoError(t, os.WriteFile(customPath, []byte("image: custom"), 0644))

		xdgDir := filepath.Join(tmpDir, "xdg", "contagent")
		require.NoError(t, os.MkdirAll(xdgDir, 0755))
		xdgPath := filepath.Join(xdgDir, "config.yaml")
		require.NoError(t, os.WriteFile(xdgPath, []byte("image: xdg"), 0644))

		homeDir := filepath.Join(tmpDir, "home", ".config", "contagent")
		require.NoError(t, os.MkdirAll(homeDir, 0755))
		homePath := filepath.Join(homeDir, "config.yaml")
		require.NoError(t, os.WriteFile(homePath, []byte("image: home"), 0644))

		// CONTAGENT_GLOBAL_CONFIG_FILE takes precedence
		env := []string{
			"CONTAGENT_GLOBAL_CONFIG_FILE=" + customPath,
			"XDG_CONFIG_HOME=" + filepath.Join(tmpDir, "xdg"),
			"HOME=" + filepath.Join(tmpDir, "home"),
		}
		path, err := FindGlobalConfig(env)
		require.NoError(t, err)
		require.Equal(t, customPath, path)

		// XDG_CONFIG_HOME takes precedence over HOME
		env = []string{
			"XDG_CONFIG_HOME=" + filepath.Join(tmpDir, "xdg"),
			"HOME=" + filepath.Join(tmpDir, "home"),
		}
		path, err = FindGlobalConfig(env)
		require.NoError(t, err)
		require.Equal(t, xdgPath, path)

		// HOME is the fallback
		env = []string{
			"HOME=" + filepath.Join(tmpDir, "home"),
		}
		path, err = FindGlobalConfig(env)
		require.NoError(t, err)
		require.Equal(t, homePath, path)
	})
}

func TestFindProjectConfig(t *testing.T) {
	t.Run("finds config in current directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ".contagent.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("image: test"), 0644))

		path, err := FindProjectConfig(tmpDir)
		require.NoError(t, err)
		require.Equal(t, configPath, path)
	})

	t.Run("finds config in parent directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ".contagent.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("image: test"), 0644))

		subDir := filepath.Join(tmpDir, "sub", "dir")
		require.NoError(t, os.MkdirAll(subDir, 0755))

		path, err := FindProjectConfig(subDir)
		require.NoError(t, err)
		require.Equal(t, configPath, path)
	})

	t.Run("returns empty when no config found", func(t *testing.T) {
		tmpDir := t.TempDir()
		path, err := FindProjectConfig(tmpDir)
		require.NoError(t, err)
		require.Empty(t, path)
	})

	t.Run("ignores directories named .contagent.yaml", func(t *testing.T) {
		tmpDir := t.TempDir()
		dirPath := filepath.Join(tmpDir, ".contagent.yaml")
		require.NoError(t, os.MkdirAll(dirPath, 0755))

		path, err := FindProjectConfig(tmpDir)
		require.NoError(t, err)
		require.Empty(t, path)
	})

	t.Run("stops at nearest config", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create config in root
		rootConfig := filepath.Join(tmpDir, ".contagent.yaml")
		require.NoError(t, os.WriteFile(rootConfig, []byte("image: root"), 0644))

		// Create config in subdirectory
		subDir := filepath.Join(tmpDir, "sub")
		require.NoError(t, os.MkdirAll(subDir, 0755))
		subConfig := filepath.Join(subDir, ".contagent.yaml")
		require.NoError(t, os.WriteFile(subConfig, []byte("image: sub"), 0644))

		// Should find the nearest one (subConfig)
		deepDir := filepath.Join(subDir, "deep")
		require.NoError(t, os.MkdirAll(deepDir, 0755))
		path, err := FindProjectConfig(deepDir)
		require.NoError(t, err)
		require.Equal(t, subConfig, path)
	})

	t.Run("returns error for invalid path", func(t *testing.T) {
		// Test with a path containing null bytes (invalid on most filesystems)
		// This will trigger an error either in Abs() or Stat()
		path, err := FindProjectConfig("invalid\x00path")
		require.Error(t, err)
		require.Empty(t, path)
		// Either "cannot get absolute path" or "cannot stat" are valid errors
		require.True(t, 
			err.Error() == "cannot get absolute path" ||
			(err.Error() != "" && (err.Error()[:11] == "cannot stat" || err.Error()[:20] == "cannot get absolute")),
			"Expected error about invalid path, got: %v", err)
	})
}

func TestParseFile(t *testing.T) {
	t.Run("parses valid complete config", func(t *testing.T) {
		configPath := filepath.Join("testdata", "valid.yaml")
		cfg, err := ParseFile(configPath)
		require.NoError(t, err)

		require.Equal(t, "test-image:v1", cfg.Image)
		require.Equal(t, "/test", cfg.WorkingDir)
		require.Equal(t, "Dockerfile.test", cfg.Dockerfile)
		require.Equal(t, "test-network", cfg.Network)
		require.Equal(t, 20, cfg.StopTimeout)
		require.Equal(t, 5, cfg.TTYRetries)
		require.Equal(t, 100*time.Millisecond, cfg.RetryDelay)
		require.Equal(t, "Test User", cfg.Git.User.Name)
		require.Equal(t, "test@example.com", cfg.Git.User.Email)
		require.Equal(t, "bar", cfg.Env["FOO"])
		require.Equal(t, "qux", cfg.Env["BAZ"])
		require.Contains(t, cfg.Volumes, "/host:/container")
		require.Contains(t, cfg.Volumes, "/data:/data")
	})

	t.Run("parses minimal config", func(t *testing.T) {
		configPath := filepath.Join("testdata", "minimal.yaml")
		cfg, err := ParseFile(configPath)
		require.NoError(t, err)

		require.Equal(t, "minimal-image:latest", cfg.Image)
		require.Empty(t, cfg.WorkingDir)
		require.Empty(t, cfg.Dockerfile)
	})

	t.Run("returns error for nonexistent file", func(t *testing.T) {
		cfg, err := ParseFile("/nonexistent/config.yaml")
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot read config file")
		require.Equal(t, Config{}, cfg)
	})

	t.Run("returns error for invalid YAML", func(t *testing.T) {
		configPath := filepath.Join("testdata", "invalid.yaml")
		cfg, err := ParseFile(configPath)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot parse config file")
		require.Equal(t, Config{}, cfg)
	})
}

func TestMakeEnvMap(t *testing.T) {
	t.Run("converts environment slice to map", func(t *testing.T) {
		env := []string{
			"FOO=bar",
			"BAZ=qux",
			"PATH=/usr/bin:/bin",
		}
		envMap := makeEnvMap(env)
		require.Equal(t, "bar", envMap["FOO"])
		require.Equal(t, "qux", envMap["BAZ"])
		require.Equal(t, "/usr/bin:/bin", envMap["PATH"])
	})

	t.Run("handles empty environment", func(t *testing.T) {
		envMap := makeEnvMap([]string{})
		require.Empty(t, envMap)
	})

	t.Run("handles values with equals signs", func(t *testing.T) {
		env := []string{"KEY=value=with=equals"}
		envMap := makeEnvMap(env)
		require.Equal(t, "value=with=equals", envMap["KEY"])
	})

	t.Run("skips invalid entries", func(t *testing.T) {
		env := []string{
			"VALID=value",
			"INVALID",
		}
		envMap := makeEnvMap(env)
		require.Equal(t, "value", envMap["VALID"])
		require.NotContains(t, envMap, "INVALID")
	})
}
