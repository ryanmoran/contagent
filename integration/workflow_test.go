package integration_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
)

// TestWorkflow validates the complete end-to-end workflow:
// 1. Git HTTP server starts and serves repository
// 2. Docker image builds successfully
// 3. Container is created with proper configuration
// 4. Git archive is copied into container
// 5. Container executes command and exits
// 6. Cleanup removes all resources
func TestWorkflow(t *testing.T) {
	type Identifiers struct {
		RepositoryPath string
		Dockerfile     string
	}

	setup := func(t *testing.T) Identifiers {
		dir, err := os.MkdirTemp("", "repository-*")
		require.NoError(t, err)

		file, err := os.CreateTemp(dir, "Dockerfile.*")
		require.NoError(t, err)
		defer file.Close()

		fmt.Fprintln(file, "FROM ubuntu:25.10")

		cmd := exec.Command("git", "init")
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()
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
			err := os.RemoveAll(file.Name())
			require.NoError(t, err)

			err = os.RemoveAll(dir)
			require.NoError(t, err)
		})

		return Identifiers{
			RepositoryPath: dir,
			Dockerfile:     file.Name(),
		}
	}

	t.Run("runs a simple echo command", func(t *testing.T) {
		identifiers := setup(t)

		cmd := exec.Command(settings.Path,
			"--dockerfile", identifiers.Dockerfile,
			"bash", "-c", "echo integration_test")
		cmd.Dir = identifiers.RepositoryPath
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		// TODO: assert on output
	})

	t.Run("container has access to git repository", func(t *testing.T) {
		identifiers := setup(t)

		cmd := exec.Command(settings.Path,
			"--dockerfile", identifiers.Dockerfile,
			"bash", "-c", "cd /app && git rev-parse --is-inside-work-tree")
		cmd.Dir = identifiers.RepositoryPath
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		// TODO: assert on output
	})

	t.Run("container can access working directory", func(t *testing.T) {
		identifiers := setup(t)

		cmd := exec.Command(settings.Path,
			"--dockerfile", identifiers.Dockerfile,
			"bash", "-c", "pwd | grep -q /app")
		cmd.Dir = identifiers.RepositoryPath
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		// TODO: assert on output
	})

	t.Run("environment variables are passed through", func(t *testing.T) {
		identifiers := setup(t)

		cmd := exec.Command(settings.Path,
			"--dockerfile", identifiers.Dockerfile,
			"bash", "-c", "test \"$TERM\" = 'screen-256color'")
		cmd.Dir = identifiers.RepositoryPath
		cmd.Env = append(os.Environ(),
			"TERM=screen-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=test-key-123",
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		// TODO: assert on output
	})

	t.Run("container can execute commands with non-zero exit", func(t *testing.T) {
		identifiers := setup(t)

		cmd := exec.Command(settings.Path,
			"--dockerfile", identifiers.Dockerfile,
			"bash", "-c", "exit 42")
		cmd.Dir = identifiers.RepositoryPath
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		// TODO: assert on output
	})

	t.Run("with volumes", func(t *testing.T) {
		identifiers := setup(t)

		tmpDir, err := os.MkdirTemp("", "contagent-volume-*")
		require.NoError(t, err)

		t.Cleanup(func() {
			err := os.RemoveAll(tmpDir)
			require.NoError(t, err)
		})

		testFile := filepath.Join(tmpDir, "test.txt")
		testContent := "integration test content"
		err = os.WriteFile(testFile, []byte(testContent), 0644)
		require.NoError(t, err)

		cmd := exec.Command(settings.Path,
			"--dockerfile", identifiers.Dockerfile,
			"--volume", fmt.Sprintf("%s:/mnt/test", tmpDir),
			"bash", "-c", "cat /mnt/test/test.txt | grep -q 'integration test'",
		)
		cmd.Dir = identifiers.RepositoryPath
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		// TODO: assert on output
	})

	t.Run("with env vars", func(t *testing.T) {
		identifiers := setup(t)

		cmd := exec.Command(settings.Path,
			"--dockerfile", identifiers.Dockerfile,
			"--env", "CUSTOM_VAR=test123",
			"bash", "-c", "test \"$CUSTOM_VAR\" = 'test123'",
		)
		cmd.Dir = identifiers.RepositoryPath
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		// TODO: assert on output
	})

	// TestWorkflowCleanup verifies that cleanup happens and containers are removed
	t.Run("cleanup", func(t *testing.T) {
		identifiers := setup(t)

		cli, err := client.New(client.FromEnv, client.WithAPIVersionNegotiation())
		require.NoError(t, err)

		t.Cleanup(func() {
			cli.Close()
		})

		ctx := t.Context()

		cmd := exec.Command(settings.Path,
			"--dockerfile", identifiers.Dockerfile,
			"bash", "-c", "exit 42")
		cmd.Dir = identifiers.RepositoryPath
		cmd.Env = append(os.Environ(),
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
		)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))

		// Give cleanup a moment to complete
		time.Sleep(500 * time.Millisecond)

		result, err := cli.ContainerList(ctx, client.ContainerListOptions{})
		require.NoError(t, err)

		for _, item := range result.Items {
			for _, name := range item.Names {
				require.NotContains(t, name, "contagent")
			}
		}
	})

	t.Run("failure cases", func(t *testing.T) {
		t.Run("when not in a git repository", func(t *testing.T) {
			identifiers := setup(t)

			tmpDir, err := os.MkdirTemp("", "contagent-test-*")
			require.NoError(t, err)

			t.Cleanup(func() {
				err := os.RemoveAll(tmpDir)
				require.NoError(t, err)
			})

			cmd := exec.Command(settings.Path,
				"--dockerfile", identifiers.Dockerfile,
				"bash", "-c", "echo test",
			)
			cmd.Dir = tmpDir
			cmd.Env = append(os.Environ(),
				"TERM=xterm-256color",
				"COLORTERM=truecolor",
				"ANTHROPIC_API_KEY=",
			)
			output, err := cmd.CombinedOutput()
			require.ErrorContains(t, err, "exit status 1")
			require.Contains(t, string(output), "not a git repository")
		})

		t.Run("when dockerfile is not specified", func(t *testing.T) {
			identifiers := setup(t)

			cmd := exec.Command(settings.Path,
				"bash", "-c", "echo test",
			)
			cmd.Dir = identifiers.RepositoryPath
			cmd.Env = append(os.Environ(),
				"TERM=xterm-256color",
				"COLORTERM=truecolor",
				"ANTHROPIC_API_KEY=",
			)
			output, err := cmd.CombinedOutput()
			require.ErrorContains(t, err, "exit status 1")
			require.Contains(t, string(output), "dockerfile path is required")
			require.Contains(t, string(output), "--dockerfile")
			require.Contains(t, string(output), ".contagent.yaml")
		})
	})
}
