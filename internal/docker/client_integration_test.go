//go:build integration
// +build integration

package docker_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ryanmoran/contagent/internal/docker"
	"github.com/ryanmoran/contagent/internal/runtime"
	"github.com/stretchr/testify/require"
)

// TestNewClient tests that we can create a Docker client
func TestNewClient(t *testing.T) {
	t.Run("creates client successfully", func(t *testing.T) {
		client, err := docker.NewDefaultClient()
		if err != nil {
			t.Skip("Docker not available:", err)
		}
		defer client.Close()

		require.NoError(t, err)
	})
}

// TestBuildImage tests image building with real Docker
func TestBuildImage(t *testing.T) {
	client, err := docker.NewDefaultClient()
	if err != nil {
		t.Skip("Docker not available:", err)
	}
	defer client.Close()

	t.Run("builds image from valid Dockerfile", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "docker-build-test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
		err = os.WriteFile(dockerfilePath, []byte("FROM alpine:latest\nRUN echo 'test'\n"), 0644)
		require.NoError(t, err)

		writer := newMockWriter()
		ctx := context.Background()

		image, err := client.BuildImage(ctx, dockerfilePath, "test-image:latest", writer)
		require.NoError(t, err)
		require.Equal(t, "test-image:latest", image.Name)
		require.Contains(t, writer.String(), "Step")
	})

	t.Run("fails with non-existent Dockerfile", func(t *testing.T) {
		writer := newMockWriter()
		ctx := context.Background()

		_, err := client.BuildImage(ctx, "/nonexistent/Dockerfile", "test-image:latest", writer)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to read Dockerfile")
	})

	t.Run("fails with invalid Dockerfile syntax", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "docker-build-test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
		err = os.WriteFile(dockerfilePath, []byte("INVALID SYNTAX\n"), 0644)
		require.NoError(t, err)

		writer := newMockWriter()
		ctx := context.Background()

		_, err = client.BuildImage(ctx, dockerfilePath, "test-image:latest", writer)
		require.Error(t, err)
	})

	t.Run("outputs build logs to writer", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "docker-build-test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
		err = os.WriteFile(dockerfilePath, []byte("FROM alpine:latest\nRUN echo 'hello world'\n"), 0644)
		require.NoError(t, err)

		writer := newMockWriter()
		ctx := context.Background()

		_, err = client.BuildImage(ctx, dockerfilePath, "test-output:latest", writer)
		require.NoError(t, err)

		output := writer.String()
		require.Contains(t, output, "Step")
	})
}

// TestCreateContainer tests container creation
func TestCreateContainer(t *testing.T) {
	client, err := docker.NewDefaultClient()
	if err != nil {
		t.Skip("Docker not available:", err)
	}
	defer client.Close()

	t.Run("creates container with basic config", func(t *testing.T) {
		ctx := context.Background()

		container, err := client.CreateContainer(ctx, runtime.CreateContainerOptions{
			SessionID:   "test-container",
			Image:       runtime.Image{Name: "alpine:latest"},
			Args:        []string{"echo", "test"},
			Env:         []string{"TEST=value"},
			Volumes:     []string{},
			WorkingDir:  "/app",
			Network:     "default",
			StopTimeout: 10,
			TTYRetries:  10,
			RetryDelay:  100 * time.Millisecond,
		})
		require.NoError(t, err)
		dc := container.(docker.Container)
		require.NotEmpty(t, dc.ID)
		require.Equal(t, "test-container", dc.Name)

		defer func() {
			_ = container.ForceRemove(ctx)
		}()
	})

	t.Run("creates container with volumes", func(t *testing.T) {
		ctx := context.Background()

		container, err := client.CreateContainer(ctx, runtime.CreateContainerOptions{
			SessionID:   "test-container-vol",
			Image:       runtime.Image{Name: "alpine:latest"},
			Args:        []string{"echo", "test"},
			Env:         []string{},
			Volumes:     []string{"/tmp:/tmp"},
			WorkingDir:  "/app",
			Network:     "default",
			StopTimeout: 10,
			TTYRetries:  10,
			RetryDelay:  100 * time.Millisecond,
		})
		require.NoError(t, err)
		dc := container.(docker.Container)
		require.NotEmpty(t, dc.ID)

		defer func() {
			_ = container.ForceRemove(ctx)
		}()
	})

	t.Run("fails with invalid image", func(t *testing.T) {
		ctx := context.Background()

		_, err := client.CreateContainer(ctx, runtime.CreateContainerOptions{
			SessionID:   "test-container-fail",
			Image:       runtime.Image{Name: "nonexistent-image-12345:latest"},
			Args:        []string{"echo", "test"},
			Env:         []string{},
			Volumes:     []string{},
			WorkingDir:  "/app",
			Network:     "default",
			StopTimeout: 10,
			TTYRetries:  10,
			RetryDelay:  100 * time.Millisecond,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to create container")
	})
}

// TestClientCloseIntegration tests that Close doesn't panic with real client
func TestClientCloseIntegration(t *testing.T) {
	client, err := docker.NewDefaultClient()
	if err != nil {
		t.Skip("Docker not available:", err)
	}

	t.Run("Close doesn't panic", func(t *testing.T) {
		require.NotPanics(t, func() {
			client.Close()
		})
	})
}

// TestBuildImageOutputParsing tests that build errors are properly parsed
func TestBuildImageOutputParsing(t *testing.T) {
	t.Run("parses error detail from build output", func(t *testing.T) {
		type buildOutput struct {
			Stream      string `json:"stream,omitempty"`
			ErrorDetail struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"errorDetail,omitempty"`
		}

		output := buildOutput{
			ErrorDetail: struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}{
				Code:    1,
				Message: "dockerfile parse error",
			},
		}

		data, err := json.Marshal(output)
		require.NoError(t, err)

		var parsed buildOutput
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		require.Equal(t, 1, parsed.ErrorDetail.Code)
		require.Equal(t, "dockerfile parse error", parsed.ErrorDetail.Message)
	})
}

// TestImage tests the Image struct
func TestImage(t *testing.T) {
	t.Run("creates image with name", func(t *testing.T) {
		image := runtime.Image{Name: "test:latest"}
		require.Equal(t, "test:latest", image.Name)
	})
}
