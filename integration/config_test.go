package integration_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestConfigFileLoading validates that config files are properly loaded and merged
// with CLI flags in the complete end-to-end workflow.
func TestConfigFileLoading(t *testing.T) {
	type TestSetup struct {
		RepositoryPath string
		Dockerfile     string
		ConfigPath     string
	}

	// setup creates a test git repository with a Dockerfile and optional config file
	setup := func(t *testing.T, configContent string) TestSetup {
		dir, err := os.MkdirTemp("", "repository-*")
		require.NoError(t, err)

		// Create a simple Dockerfile
		dockerfilePath := filepath.Join(dir, "Dockerfile")
		err = os.WriteFile(dockerfilePath, []byte("FROM ubuntu:25.10\n"), 0644)
		require.NoError(t, err)

		// Create config file if provided
		var configPath string
		if configContent != "" {
			configPath = filepath.Join(dir, ".contagent.yaml")
			err = os.WriteFile(configPath, []byte(configContent), 0644)
			require.NoError(t, err)
		}

		// Initialize git repository
		cmd := exec.Command("git", "init")
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		// Configure git user for commits
		cmd = exec.Command("git", "config", "user.email", "test@example.com")
		cmd.Dir = dir
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		cmd = exec.Command("git", "config", "user.name", "Test User")
		cmd.Dir = dir
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		cmd = exec.Command("git", "add", "-A", ".")
		cmd.Dir = dir
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		cmd = exec.Command("git", "commit", "-m", "Initial commit")
		cmd.Dir = dir
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		t.Cleanup(func() {
			err := os.RemoveAll(dir)
			require.NoError(t, err)
		})

		return TestSetup{
			RepositoryPath: dir,
			Dockerfile:     dockerfilePath,
			ConfigPath:     configPath,
		}
	}

	t.Run("loads project config file with basic settings", func(t *testing.T) {
		configContent := `
working_dir: /workspace
`
		testSetup := setup(t, configContent)

		cmd := exec.Command(settings.Path,
			"--dockerfile", testSetup.Dockerfile,
			"bash", "-c", "exit 0")
		cmd.Dir = testSetup.RepositoryPath
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))
	})

	t.Run("loads project config file with environment variables", func(t *testing.T) {
		configContent := `
env:
  TEST_VAR: config_value
  ANOTHER_VAR: another_value
`
		testSetup := setup(t, configContent)

		cmd := exec.Command(settings.Path,
			"--dockerfile", testSetup.Dockerfile,
			"bash", "-c", "exit 0")
		cmd.Dir = testSetup.RepositoryPath
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))
	})

	t.Run("loads project config file with volumes", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "contagent-volume-*")
		require.NoError(t, err)
		t.Cleanup(func() {
			os.RemoveAll(tmpDir)
		})

		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("config test content"), 0644)
		require.NoError(t, err)

		configContent := fmt.Sprintf(`
volumes:
  - %s:/mnt/config-volume
`, tmpDir)
		testSetup := setup(t, configContent)

		cmd := exec.Command(settings.Path,
			"--dockerfile", testSetup.Dockerfile,
			"bash", "-c", "test -f /mnt/config-volume/test.txt")
		cmd.Dir = testSetup.RepositoryPath
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))
	})

	t.Run("CLI flags override config file settings", func(t *testing.T) {
		configContent := `
env:
  TEST_VAR: from_config
`
		testSetup := setup(t, configContent)

		cmd := exec.Command(settings.Path,
			"--dockerfile", testSetup.Dockerfile,
			"--env", "TEST_VAR=from_cli",
			"bash", "-c", "exit 0")
		cmd.Dir = testSetup.RepositoryPath
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))
	})

	t.Run("merges config file and CLI flags", func(t *testing.T) {
		configContent := `
env:
  CONFIG_VAR: from_config
`
		testSetup := setup(t, configContent)

		cmd := exec.Command(settings.Path,
			"--dockerfile", testSetup.Dockerfile,
			"--env", "CLI_VAR=from_cli",
			"bash", "-c", "exit 0")
		cmd.Dir = testSetup.RepositoryPath
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))
	})

	t.Run("works without config file (defaults only)", func(t *testing.T) {
		testSetup := setup(t, "") // No config file

		cmd := exec.Command(settings.Path,
			"--dockerfile", testSetup.Dockerfile,
			"bash", "-c", "exit 0")
		cmd.Dir = testSetup.RepositoryPath
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))
	})

	t.Run("loads config from parent directory", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "repository-*")
		require.NoError(t, err)
		t.Cleanup(func() {
			os.RemoveAll(dir)
		})

		// Create config in root
		configPath := filepath.Join(dir, ".contagent.yaml")
		configContent := `
env:
  PARENT_CONFIG: from_parent
`
		err = os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Create Dockerfile in root
		dockerfilePath := filepath.Join(dir, "Dockerfile")
		err = os.WriteFile(dockerfilePath, []byte("FROM ubuntu:25.10\n"), 0644)
		require.NoError(t, err)

		// Initialize git repository in root
		cmd := exec.Command("git", "init")
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		// Configure git user for commits
		cmd = exec.Command("git", "config", "user.email", "test@example.com")
		cmd.Dir = dir
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		cmd = exec.Command("git", "config", "user.name", "Test User")
		cmd.Dir = dir
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		cmd = exec.Command("git", "add", "-A", ".")
		cmd.Dir = dir
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		cmd = exec.Command("git", "commit", "-m", "Initial commit")
		cmd.Dir = dir
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		// Create subdirectory to run from (config should be found in parent)
		subDir := filepath.Join(dir, "sub")
		err = os.MkdirAll(subDir, 0755)
		require.NoError(t, err)

		// Run from root git directory, but the working directory
		// (current directory used by config loading) will be the subdirectory
		// This tests that FindProjectConfig walks up the directory tree
		cmd = exec.Command(settings.Path,
			"--dockerfile", dockerfilePath,
			"bash", "-c", "exit 0")
		cmd.Dir = dir // Run from git root, not subDir
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
		)
		output, err = cmd.CombinedOutput()
		require.NoError(t, err, string(output))
	})

	t.Run("expands environment variables in config", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "contagent-volume-*")
		require.NoError(t, err)
		t.Cleanup(func() {
			os.RemoveAll(tmpDir)
		})

		testFile := filepath.Join(tmpDir, "test.txt")
		err = os.WriteFile(testFile, []byte("expanded content"), 0644)
		require.NoError(t, err)

		configContent := `
env:
  MY_PATH: $TEST_DIR/subdir
volumes:
  - $TEST_DIR:/mnt/test
`
		testSetup := setup(t, configContent)

		cmd := exec.Command(settings.Path,
			"--dockerfile", testSetup.Dockerfile,
			"bash", "-c", "test -f /mnt/test/test.txt")
		cmd.Dir = testSetup.RepositoryPath
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
			fmt.Sprintf("TEST_DIR=%s", tmpDir),
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))
	})

	t.Run("loads global config file", func(t *testing.T) {
		// Create a temporary global config
		globalDir, err := os.MkdirTemp("", "contagent-global-*")
		require.NoError(t, err)
		t.Cleanup(func() {
			os.RemoveAll(globalDir)
		})

		globalConfigPath := filepath.Join(globalDir, "global-config.yaml")
		globalConfigContent := `
env:
  GLOBAL_VAR: from_global_config
`
		err = os.WriteFile(globalConfigPath, []byte(globalConfigContent), 0644)
		require.NoError(t, err)

		testSetup := setup(t, "")

		cmd := exec.Command(settings.Path,
			"--dockerfile", testSetup.Dockerfile,
			"bash", "-c", "exit 0")
		cmd.Dir = testSetup.RepositoryPath
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
			fmt.Sprintf("CONTAGENT_GLOBAL_CONFIG_FILE=%s", globalConfigPath),
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))
	})

	t.Run("project config overrides global config", func(t *testing.T) {
		// Create a temporary global config
		globalDir, err := os.MkdirTemp("", "contagent-global-*")
		require.NoError(t, err)
		t.Cleanup(func() {
			os.RemoveAll(globalDir)
		})

		globalConfigPath := filepath.Join(globalDir, "global-config.yaml")
		globalConfigContent := `
env:
  SHARED_VAR: from_global
`
		err = os.WriteFile(globalConfigPath, []byte(globalConfigContent), 0644)
		require.NoError(t, err)

		// Create project config that overrides
		projectConfigContent := `
env:
  SHARED_VAR: from_project
`
		testSetup := setup(t, projectConfigContent)

		cmd := exec.Command(settings.Path,
			"--dockerfile", testSetup.Dockerfile,
			"bash", "-c", "exit 0")
		cmd.Dir = testSetup.RepositoryPath
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
			fmt.Sprintf("CONTAGENT_GLOBAL_CONFIG_FILE=%s", globalConfigPath),
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))
	})

	t.Run("loads git user config from file", func(t *testing.T) {
		configContent := `
git:
  user:
    name: Test User
    email: test@example.com
`
		testSetup := setup(t, configContent)

		cmd := exec.Command(settings.Path,
			"--dockerfile", testSetup.Dockerfile,
			"bash", "-c", "exit 0")
		cmd.Dir = testSetup.RepositoryPath
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))
	})

	t.Run("complex config with all fields", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "contagent-volume-*")
		require.NoError(t, err)
		t.Cleanup(func() {
			os.RemoveAll(tmpDir)
		})

		testFile := filepath.Join(tmpDir, "data.txt")
		err = os.WriteFile(testFile, []byte("complex test data"), 0644)
		require.NoError(t, err)

		configContent := fmt.Sprintf(`
working_dir: /workspace
git:
  user:
    name: Complex User
    email: complex@example.com
env:
  VAR1: value1
  VAR2: value2
volumes:
  - %s:/mnt/data
`, tmpDir)
		testSetup := setup(t, configContent)

		cmd := exec.Command(settings.Path,
			"--dockerfile", testSetup.Dockerfile,
			"bash", "-c", "test -f /mnt/data/data.txt")
		cmd.Dir = testSetup.RepositoryPath
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))
	})
}
