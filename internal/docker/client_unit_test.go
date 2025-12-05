package docker_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"github.com/ryanmoran/contagent/internal/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildImageWithMock tests BuildImage using a mock Docker client
func TestBuildImageWithMock(t *testing.T) {
	t.Run("succeeds with valid build response", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "docker-mock-test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
		err = os.WriteFile(dockerfilePath, []byte("FROM alpine:latest\n"), 0644)
		require.NoError(t, err)

		buildOutput := []map[string]interface{}{
			{"stream": "Step 1/1 : FROM alpine:latest\n"},
			{"stream": "Successfully built abc123\n"},
		}
		outputBytes, _ := json.Marshal(buildOutput[0])
		outputBytes = append(outputBytes, '\n')
		output2, _ := json.Marshal(buildOutput[1])
		outputBytes = append(outputBytes, output2...)
		outputBytes = append(outputBytes, '\n')

		mock := &mockDockerClient{
			imageBuildFunc: func(ctx context.Context, buildContext io.Reader, options client.ImageBuildOptions) (client.ImageBuildResult, error) {
				return client.ImageBuildResult{
					Body: io.NopCloser(bytes.NewReader(outputBytes)),
				}, nil
			},
		}

		c := docker.NewClient(mock)
		writer := newMockWriter()
		ctx := context.Background()

		image, err := c.BuildImage(ctx, dockerfilePath, "test:latest", writer)
		require.NoError(t, err)
		assert.Equal(t, "test:latest", image.Name)
		assert.Contains(t, writer.String(), "Step")
	})

	t.Run("fails when ImageBuild returns error", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "docker-mock-test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
		err = os.WriteFile(dockerfilePath, []byte("FROM alpine:latest\n"), 0644)
		require.NoError(t, err)

		mock := &mockDockerClient{
			imageBuildFunc: func(ctx context.Context, buildContext io.Reader, options client.ImageBuildOptions) (client.ImageBuildResult, error) {
				return client.ImageBuildResult{}, errors.New("build failed")
			},
		}

		c := docker.NewClient(mock)
		writer := newMockWriter()
		ctx := context.Background()

		_, err = c.BuildImage(ctx, dockerfilePath, "test:latest", writer)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to build image")
	})

	t.Run("fails when build output contains error detail", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "docker-mock-test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
		err = os.WriteFile(dockerfilePath, []byte("FROM alpine:latest\n"), 0644)
		require.NoError(t, err)

		errorOutput := map[string]interface{}{
			"errorDetail": map[string]interface{}{
				"code":    1,
				"message": "dockerfile parse error",
			},
		}
		outputBytes, _ := json.Marshal(errorOutput)
		outputBytes = append(outputBytes, '\n')

		mock := &mockDockerClient{
			imageBuildFunc: func(ctx context.Context, buildContext io.Reader, options client.ImageBuildOptions) (client.ImageBuildResult, error) {
				return client.ImageBuildResult{
					Body: io.NopCloser(bytes.NewReader(outputBytes)),
				}, nil
			},
		}

		c := docker.NewClient(mock)
		writer := newMockWriter()
		ctx := context.Background()

		_, err = c.BuildImage(ctx, dockerfilePath, "test:latest", writer)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dockerfile parse error")
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "docker-mock-test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
		err = os.WriteFile(dockerfilePath, []byte("FROM alpine:latest\n"), 0644)
		require.NoError(t, err)

		mock := &mockDockerClient{
			imageBuildFunc: func(ctx context.Context, buildContext io.Reader, options client.ImageBuildOptions) (client.ImageBuildResult, error) {
				return client.ImageBuildResult{}, context.Canceled
			},
		}

		c := docker.NewClient(mock)
		writer := newMockWriter()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err = c.BuildImage(ctx, dockerfilePath, "test:latest", writer)
		require.Error(t, err)
	})
}

// TestCreateContainerWithMock tests CreateContainer using a mock Docker client
func TestCreateContainerWithMock(t *testing.T) {
	t.Run("creates container successfully", func(t *testing.T) {
		mock := &mockDockerClient{
			containerCreateFunc: func(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
				return client.ContainerCreateResult{
					ID: "container123",
				}, nil
			},
		}

		c := docker.NewClient(mock)
		ctx := context.Background()
		image := docker.Image{Name: "alpine:latest"}

		container, err := c.CreateContainer(ctx, "test-container", image, []string{"echo", "test"}, []string{}, []string{}, "/app", "some-network", 10, 10, 100*time.Millisecond)
		require.NoError(t, err)
		assert.Equal(t, "container123", container.ID)
		assert.Equal(t, "test-container", container.Name)
	})

	t.Run("fails when ContainerCreate returns error", func(t *testing.T) {
		mock := &mockDockerClient{
			containerCreateFunc: func(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
				return client.ContainerCreateResult{}, errors.New("image not found")
			},
		}

		c := docker.NewClient(mock)
		ctx := context.Background()
		image := docker.Image{Name: "nonexistent:latest"}

		_, err := c.CreateContainer(ctx, "test-container", image, []string{"echo", "test"}, []string{}, []string{}, "/app", "some-network", 10, 10, 100*time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create container")
	})

	t.Run("passes correct configuration to Docker API", func(t *testing.T) {
		var capturedOptions client.ContainerCreateOptions

		mock := &mockDockerClient{
			containerCreateFunc: func(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
				capturedOptions = options
				return client.ContainerCreateResult{ID: "container123"}, nil
			},
		}

		c := docker.NewClient(mock)
		ctx := context.Background()
		image := docker.Image{Name: "alpine:latest"}
		args := []string{"sh", "-c", "echo test"}
		env := []string{"FOO=bar", "BAZ=qux"}
		volumes := []string{"/host:/container"}
		workingDir := "/custom"

		_, err := c.CreateContainer(ctx, "test-name", image, args, env, volumes, workingDir, "some-network", 10, 10, 100*time.Millisecond)
		require.NoError(t, err)

		assert.Equal(t, "alpine:latest", capturedOptions.Config.Image)
		assert.Equal(t, args, capturedOptions.Config.Cmd)
		assert.Equal(t, env, capturedOptions.Config.Env)
		assert.Equal(t, workingDir, capturedOptions.Config.WorkingDir)
		assert.True(t, capturedOptions.Config.Tty)
		assert.True(t, capturedOptions.Config.OpenStdin)
		assert.Equal(t, volumes, capturedOptions.HostConfig.Binds)
		assert.Contains(t, capturedOptions.HostConfig.ExtraHosts, "host.docker.internal:host-gateway")
		assert.Equal(t, "test-name", capturedOptions.Name)
	})
}

// TestClientClose tests that Close works correctly
func TestClientClose(t *testing.T) {
	t.Run("calls close on underlying client", func(t *testing.T) {
		closeCalled := false
		mock := &mockDockerClient{
			closeFunc: func() error {
				closeCalled = true
				return nil
			},
		}

		c := docker.NewClient(mock)
		c.Close()

		assert.True(t, closeCalled)
	})

	t.Run("handles close error gracefully", func(t *testing.T) {
		mock := &mockDockerClient{
			closeFunc: func() error {
				return errors.New("close failed")
			},
		}

		c := docker.NewClient(mock)
		assert.NotPanics(t, func() {
			c.Close()
		})
	})
}
