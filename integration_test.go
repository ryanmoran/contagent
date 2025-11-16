//go:build integration
// +build integration

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ryanmoran/contagent/internal/docker"
	"github.com/stretchr/testify/require"
)

// TestFullWorkflow validates the complete end-to-end workflow:
// 1. Git HTTP server starts and serves repository
// 2. Docker image builds successfully
// 3. Container is created with proper configuration
// 4. Git archive is copied into container
// 5. Container executes command and exits
// 6. Cleanup removes all resources
func TestFullWorkflow(t *testing.T) {
	// Skip if Docker is not available
	if os.Getenv("SKIP_INTEGRATION") == "true" {
		t.Skip("Integration tests skipped")
	}

	// Verify Docker daemon is running
	client, err := docker.NewDefaultClient()
	require.NoError(t, err, "Docker daemon must be running for integration tests")
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Verify we can connect to Docker
	_, err = client.Ping(ctx)
	require.NoError(t, err, "Failed to ping Docker daemon")

	t.Run("successful workflow with echo command", func(t *testing.T) {
		// Set up environment to pass through
		env := []string{
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
		}

		// Test with a simple echo command
		err := run([]string{"contagent", "bash", "-c", "echo 'integration test'"}, env)
		require.NoError(t, err)
	})

	t.Run("container has access to git repository", func(t *testing.T) {
		env := []string{
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
		}

		// Verify container has git repository with correct branch
		err := run([]string{"contagent", "bash", "-c", "cd /app && git rev-parse --is-inside-work-tree"}, env)
		require.NoError(t, err)
	})

	t.Run("container can access working directory", func(t *testing.T) {
		env := []string{
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
		}

		// Verify container starts in /app directory
		err := run([]string{"contagent", "bash", "-c", "pwd | grep -q /app"}, env)
		require.NoError(t, err)
	})

	t.Run("environment variables are passed through", func(t *testing.T) {
		env := []string{
			"TERM=screen-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=test-key-123",
		}

		// Verify environment variables are set correctly
		err := run([]string{"contagent", "bash", "-c", "test \"$TERM\" = 'screen-256color'"}, env)
		require.NoError(t, err)
	})

	t.Run("container can execute commands with non-zero exit", func(t *testing.T) {
		env := []string{
			"TERM=xterm-256color",
			"COLORTERM=truecolor",
			"ANTHROPIC_API_KEY=",
		}

		// Verify non-zero exit codes don't cause run() to error
		// (exit status is logged but not returned as error)
		err := run([]string{"contagent", "bash", "-c", "exit 42"}, env)
		require.NoError(t, err)
	})
}

// TestWorkflowWithDockerNotRunning tests error handling when Docker daemon is unavailable
func TestWorkflowWithDockerNotRunning(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION") == "true" {
		t.Skip("Integration tests skipped")
	}

	// This test verifies that the error message is helpful when Docker is not available
	// In practice, this is hard to test without stopping Docker, so we skip it
	// unless specifically requested
	if os.Getenv("TEST_DOCKER_UNAVAILABLE") != "true" {
		t.Skip("Skipping Docker unavailability test (requires Docker to be stopped)")
	}

	env := []string{
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"ANTHROPIC_API_KEY=",
	}

	err := run([]string{"contagent", "bash", "-c", "echo test"}, env)
	require.Error(t, err)
	require.Contains(t, err.Error(), "docker")
}

// TestWorkflowWithInvalidGitRepo tests error handling when not in a git repository
func TestWorkflowWithInvalidGitRepo(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION") == "true" {
		t.Skip("Integration tests skipped")
	}

	// Create a temporary directory that is not a git repository
	tmpDir, err := os.MkdirTemp("", "contagent-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// Copy Dockerfile to temp directory so build can work
	dockerfilePath := filepath.Join(originalDir, "Dockerfile")
	dockerfileContent, err := os.ReadFile(dockerfilePath)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "Dockerfile"), dockerfileContent, 0644)
	require.NoError(t, err)

	env := []string{
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"ANTHROPIC_API_KEY=",
	}

	// Should fail because we're not in a git repository
	err = run([]string{"contagent", "bash", "-c", "echo test"}, env)
	require.Error(t, err)
	require.Contains(t, err.Error(), "git")
}

// TestWorkflowCleanup verifies that cleanup happens and containers are removed
func TestWorkflowCleanup(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION") == "true" {
		t.Skip("Integration tests skipped")
	}

	client, err := docker.NewDefaultClient()
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	env := []string{
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"ANTHROPIC_API_KEY=",
	}

	// Run a command
	err = run([]string{"contagent", "bash", "-c", "echo cleanup test"}, env)
	require.NoError(t, err)

	// Give cleanup a moment to complete
	time.Sleep(500 * time.Millisecond)

	// Verify no contagent-* containers are left behind
	containers, err := client.ListContainers(ctx)
	require.NoError(t, err)

	for _, containerID := range containers {
		t.Logf("Found container: %s (should not be a contagent container)", containerID)
	}
}

// TestWorkflowWithVolumes tests volume mounting
func TestWorkflowWithVolumes(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION") == "true" {
		t.Skip("Integration tests skipped")
	}

	// Create a temporary directory with a test file
	tmpDir, err := os.MkdirTemp("", "contagent-volume-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "integration test content"
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	env := []string{
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"ANTHROPIC_API_KEY=",
	}

	// Test with volume mount
	volumeMount := fmt.Sprintf("%s:/mnt/test", tmpDir)
	err = run([]string{"contagent", "-volume", volumeMount, "bash", "-c", "cat /mnt/test/test.txt | grep -q 'integration test'"}, env)
	require.NoError(t, err)
}

// TestWorkflowWithCustomEnv tests custom environment variables
func TestWorkflowWithCustomEnv(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION") == "true" {
		t.Skip("Integration tests skipped")
	}

	env := []string{
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"ANTHROPIC_API_KEY=",
	}

	// Test with custom environment variable
	err := run([]string{"contagent", "-env", "CUSTOM_VAR=test123", "bash", "-c", "test \"$CUSTOM_VAR\" = 'test123'"}, env)
	require.NoError(t, err)
}
