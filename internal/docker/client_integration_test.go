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
	"github.com/stretchr/testify/assert"
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
		assert.Equal(t, "test-image:latest", image.Name)
		assert.Contains(t, writer.String(), "Step")
	})

	t.Run("fails with non-existent Dockerfile", func(t *testing.T) {
		writer := newMockWriter()
		ctx := context.Background()

		_, err := client.BuildImage(ctx, "/nonexistent/Dockerfile", "test-image:latest", writer)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read Dockerfile")
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
		assert.Contains(t, output, "Step")
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

		image := docker.Image{Name: "alpine:latest"}
		args := []string{"echo", "test"}
		env := []string{"TEST=value"}
		volumes := []string{}
		workingDir := "/app"

		container, err := client.CreateContainer(ctx, "test-container", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
		require.NoError(t, err)
		assert.NotEmpty(t, container.ID)
		assert.Equal(t, "test-container", container.Name)

		defer func() {
			_ = container.ForceRemove(ctx)
		}()
	})

	t.Run("creates container with volumes", func(t *testing.T) {
		ctx := context.Background()

		image := docker.Image{Name: "alpine:latest"}
		args := []string{"echo", "test"}
		env := []string{}
		volumes := []string{"/tmp:/tmp"}
		workingDir := "/app"

		container, err := client.CreateContainer(ctx, "test-container-vol", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
		require.NoError(t, err)
		assert.NotEmpty(t, container.ID)

		defer func() {
			_ = container.ForceRemove(ctx)
		}()
	})

	t.Run("fails with invalid image", func(t *testing.T) {
		ctx := context.Background()

		image := docker.Image{Name: "nonexistent-image-12345:latest"}
		args := []string{"echo", "test"}
		env := []string{}
		volumes := []string{}
		workingDir := "/app"

		_, err := client.CreateContainer(ctx, "test-container-fail", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create container")
	})
}

// TestClientCloseIntegration tests that Close doesn't panic with real client
func TestClientCloseIntegration(t *testing.T) {
	client, err := docker.NewDefaultClient()
	if err != nil {
		t.Skip("Docker not available:", err)
	}

	t.Run("Close doesn't panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
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

		assert.Equal(t, 1, parsed.ErrorDetail.Code)
		assert.Equal(t, "dockerfile parse error", parsed.ErrorDetail.Message)
	})
}

// TestImage tests the Image struct
func TestImage(t *testing.T) {
	t.Run("creates image with name", func(t *testing.T) {
		image := docker.Image{Name: "test:latest"}
		assert.Equal(t, "test:latest", image.Name)
	})
}
