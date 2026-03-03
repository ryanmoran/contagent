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
	"github.com/ryanmoran/contagent/internal/runtime"
	"github.com/stretchr/testify/require"
)

// TestBuildImageWithMock tests BuildImage using a mock Docker client
func TestBuildImageWithMock(t *testing.T) {
	t.Run("succeeds with valid build response", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "docker-mock-test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
		err = os.WriteFile(dockerfilePath, []byte("FROM alpine:latest\n"), 0600)
		require.NoError(t, err)

		buildOutput := []map[string]interface{}{
			{"stream": "Step 1/1 : FROM alpine:latest\n"},
			{"stream": "Successfully built abc123\n"},
		}
		outputBytes, err := json.Marshal(buildOutput[0])
		require.NoError(t, err)
		outputBytes = append(outputBytes, '\n')
		output2, err := json.Marshal(buildOutput[1])
		require.NoError(t, err)
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
		require.Equal(t, "test:latest", image.Name)
		require.Contains(t, writer.String(), "Step")
	})

	t.Run("fails when ImageBuild returns error", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "docker-mock-test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
		err = os.WriteFile(dockerfilePath, []byte("FROM alpine:latest\n"), 0600)
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
		require.Contains(t, err.Error(), "failed to build image")
	})

	t.Run("fails when build output contains error detail", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "docker-mock-test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
		err = os.WriteFile(dockerfilePath, []byte("FROM alpine:latest\n"), 0600)
		require.NoError(t, err)

		errorOutput := map[string]interface{}{
			"errorDetail": map[string]interface{}{
				"code":    1,
				"message": "dockerfile parse error",
			},
		}
		outputBytes, err := json.Marshal(errorOutput)
		require.NoError(t, err)
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
		require.Contains(t, err.Error(), "dockerfile parse error")
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "docker-mock-test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
		err = os.WriteFile(dockerfilePath, []byte("FROM alpine:latest\n"), 0600)
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

		container, err := c.CreateContainer(ctx, runtime.CreateContainerOptions{
			SessionID:   "test-container",
			Image:       runtime.Image{Name: "alpine:latest"},
			Args:        []string{"echo", "test"},
			Env:         []string{},
			Volumes:     []string{},
			WorkingDir:  "/app",
			Network:     "some-network",
			StopTimeout: 10,
			TTYRetries:  10,
			RetryDelay:  100 * time.Millisecond,
		})
		require.NoError(t, err)
		dc, ok := container.(docker.Container)
		require.True(t, ok, "container should be docker.Container type")
		require.Equal(t, "container123", dc.ID)
		require.Equal(t, "test-container", dc.Name)
	})

	t.Run("fails when ContainerCreate returns error", func(t *testing.T) {
		mock := &mockDockerClient{
			containerCreateFunc: func(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
				return client.ContainerCreateResult{}, errors.New("image not found")
			},
		}

		c := docker.NewClient(mock)
		ctx := context.Background()

		_, err := c.CreateContainer(ctx, runtime.CreateContainerOptions{
			SessionID:   "test-container",
			Image:       runtime.Image{Name: "nonexistent:latest"},
			Args:        []string{"echo", "test"},
			Env:         []string{},
			Volumes:     []string{},
			WorkingDir:  "/app",
			Network:     "some-network",
			StopTimeout: 10,
			TTYRetries:  10,
			RetryDelay:  100 * time.Millisecond,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to create container")
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
		args := []string{"sh", "-c", "echo test"}
		env := []string{"FOO=bar", "BAZ=qux"}
		volumes := []string{"/host:/container"}
		workingDir := "/custom"

		_, err := c.CreateContainer(ctx, runtime.CreateContainerOptions{
			SessionID:   "test-name",
			Image:       runtime.Image{Name: "alpine:latest"},
			Args:        args,
			Env:         env,
			Volumes:     volumes,
			WorkingDir:  workingDir,
			Network:     "some-network",
			StopTimeout: 10,
			TTYRetries:  10,
			RetryDelay:  100 * time.Millisecond,
		})
		require.NoError(t, err)

		require.Equal(t, "alpine:latest", capturedOptions.Config.Image)
		require.Equal(t, args, capturedOptions.Config.Cmd)
		require.Equal(t, env, capturedOptions.Config.Env)
		require.Equal(t, workingDir, capturedOptions.Config.WorkingDir)
		require.True(t, capturedOptions.Config.Tty)
		require.True(t, capturedOptions.Config.OpenStdin)
		require.Equal(t, volumes, capturedOptions.HostConfig.Binds)
		require.Contains(t, capturedOptions.HostConfig.ExtraHosts, "host.docker.internal:host-gateway")
		require.Equal(t, "test-name", capturedOptions.Name)
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
		err := c.Close()

		require.NoError(t, err)
		require.True(t, closeCalled)
	})

	t.Run("returns close error", func(t *testing.T) {
		mock := &mockDockerClient{
			closeFunc: func() error {
				return errors.New("close failed")
			},
		}

		c := docker.NewClient(mock)
		err := c.Close()
		require.Error(t, err)
		require.Contains(t, err.Error(), "close failed")
	})
}

// TestHostAddress tests HostAddress returns correct value
func TestHostAddress(t *testing.T) {
	mock := &mockDockerClient{}
	c := docker.NewClient(mock)
	require.Equal(t, "host.docker.internal", c.HostAddress())
}
