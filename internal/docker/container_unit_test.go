package docker_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/ryanmoran/contagent/internal/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContainerStartWithMock tests Container.Start using a mock Docker client
func TestContainerStartWithMock(t *testing.T) {
	t.Run("starts container successfully", func(t *testing.T) {
		startCalled := false
		mock := &mockDockerClient{
			containerCreateFunc: func(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
				return client.ContainerCreateResult{ID: "container123"}, nil
			},
			containerStartFunc: func(ctx context.Context, containerID string, options client.ContainerStartOptions) (client.ContainerStartResult, error) {
				startCalled = true
				assert.Equal(t, "container123", containerID)
				return client.ContainerStartResult{}, nil
			},
		}

		c := docker.NewClient(mock)
		ctx := context.Background()
		image := docker.Image{Name: "alpine:latest"}

		container, err := c.CreateContainer(ctx, "test", image, []string{"echo"}, []string{}, []string{}, "/app", 10, 10, 100*time.Millisecond)
		require.NoError(t, err)

		err = container.Start(ctx)
		require.NoError(t, err)
		assert.True(t, startCalled)
	})

	t.Run("fails when ContainerStart returns error", func(t *testing.T) {
		mock := &mockDockerClient{
			containerCreateFunc: func(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
				return client.ContainerCreateResult{ID: "container123"}, nil
			},
			containerStartFunc: func(ctx context.Context, containerID string, options client.ContainerStartOptions) (client.ContainerStartResult, error) {
				return client.ContainerStartResult{}, errors.New("container not found")
			},
		}

		c := docker.NewClient(mock)
		ctx := context.Background()
		image := docker.Image{Name: "alpine:latest"}

		container, err := c.CreateContainer(ctx, "test", image, []string{"echo"}, []string{}, []string{}, "/app", 10, 10, 100*time.Millisecond)
		require.NoError(t, err)

		err = container.Start(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to start container")
	})
}

// TestContainerRemoveWithMock tests Container.Remove using a mock Docker client
func TestContainerRemoveWithMock(t *testing.T) {
	t.Run("removes container successfully", func(t *testing.T) {
		removeCalled := false
		mock := &mockDockerClient{
			containerCreateFunc: func(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
				return client.ContainerCreateResult{ID: "container123"}, nil
			},
			containerRemoveFunc: func(ctx context.Context, containerID string, options client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
				removeCalled = true
				assert.Equal(t, "container123", containerID)
				assert.False(t, options.Force)
				return client.ContainerRemoveResult{}, nil
			},
		}

		c := docker.NewClient(mock)
		ctx := context.Background()
		image := docker.Image{Name: "alpine:latest"}

		container, err := c.CreateContainer(ctx, "test", image, []string{"echo"}, []string{}, []string{}, "/app", 10, 10, 100*time.Millisecond)
		require.NoError(t, err)

		err = container.Remove(ctx)
		require.NoError(t, err)
		assert.True(t, removeCalled)
	})

	t.Run("fails when ContainerRemove returns error", func(t *testing.T) {
		mock := &mockDockerClient{
			containerCreateFunc: func(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
				return client.ContainerCreateResult{ID: "container123"}, nil
			},
			containerRemoveFunc: func(ctx context.Context, containerID string, options client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
				return client.ContainerRemoveResult{}, errors.New("container not found")
			},
		}

		c := docker.NewClient(mock)
		ctx := context.Background()
		image := docker.Image{Name: "alpine:latest"}

		container, err := c.CreateContainer(ctx, "test", image, []string{"echo"}, []string{}, []string{}, "/app", 10, 10, 100*time.Millisecond)
		require.NoError(t, err)

		err = container.Remove(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to remove container")
	})
}

// TestContainerForceRemoveWithMock tests Container.ForceRemove using a mock Docker client
func TestContainerForceRemoveWithMock(t *testing.T) {
	t.Run("force removes container successfully", func(t *testing.T) {
		removeCalled := false
		mock := &mockDockerClient{
			containerCreateFunc: func(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
				return client.ContainerCreateResult{ID: "container123"}, nil
			},
			containerRemoveFunc: func(ctx context.Context, containerID string, options client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
				removeCalled = true
				assert.Equal(t, "container123", containerID)
				assert.True(t, options.Force)
				return client.ContainerRemoveResult{}, nil
			},
		}

		c := docker.NewClient(mock)
		ctx := context.Background()
		image := docker.Image{Name: "alpine:latest"}

		container, err := c.CreateContainer(ctx, "test", image, []string{"echo"}, []string{}, []string{}, "/app", 10, 10, 100*time.Millisecond)
		require.NoError(t, err)

		err = container.ForceRemove(ctx)
		require.NoError(t, err)
		assert.True(t, removeCalled)
	})

	t.Run("fails when force remove returns error", func(t *testing.T) {
		mock := &mockDockerClient{
			containerCreateFunc: func(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
				return client.ContainerCreateResult{ID: "container123"}, nil
			},
			containerRemoveFunc: func(ctx context.Context, containerID string, options client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
				return client.ContainerRemoveResult{}, errors.New("remove failed")
			},
		}

		c := docker.NewClient(mock)
		ctx := context.Background()
		image := docker.Image{Name: "alpine:latest"}

		container, err := c.CreateContainer(ctx, "test", image, []string{"echo"}, []string{}, []string{}, "/app", 10, 10, 100*time.Millisecond)
		require.NoError(t, err)

		err = container.ForceRemove(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to force remove container")
	})
}

// TestContainerCopyToWithMock tests Container.CopyTo using a mock Docker client
func TestContainerCopyToWithMock(t *testing.T) {
	t.Run("copies content to container successfully", func(t *testing.T) {
		copyCalled := false
		mock := &mockDockerClient{
			containerCreateFunc: func(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
				return client.ContainerCreateResult{ID: "container123"}, nil
			},
			copyToContainerFunc: func(ctx context.Context, containerID string, options client.CopyToContainerOptions) (client.CopyToContainerResult, error) {
				copyCalled = true
				assert.Equal(t, "container123", containerID)
				assert.Equal(t, "/tmp", options.DestinationPath)
				return client.CopyToContainerResult{}, nil
			},
		}

		c := docker.NewClient(mock)
		ctx := context.Background()
		image := docker.Image{Name: "alpine:latest"}

		container, err := c.CreateContainer(ctx, "test", image, []string{"echo"}, []string{}, []string{}, "/app", 10, 10, 100*time.Millisecond)
		require.NoError(t, err)

		err = container.CopyTo(ctx, io.NopCloser(nil), "/tmp")
		require.NoError(t, err)
		assert.True(t, copyCalled)
	})

	t.Run("fails when CopyToContainer returns error", func(t *testing.T) {
		mock := &mockDockerClient{
			containerCreateFunc: func(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
				return client.ContainerCreateResult{ID: "container123"}, nil
			},
			copyToContainerFunc: func(ctx context.Context, containerID string, options client.CopyToContainerOptions) (client.CopyToContainerResult, error) {
				return client.CopyToContainerResult{}, errors.New("copy failed")
			},
		}

		c := docker.NewClient(mock)
		ctx := context.Background()
		image := docker.Image{Name: "alpine:latest"}

		container, err := c.CreateContainer(ctx, "test", image, []string{"echo"}, []string{}, []string{}, "/app", 10, 10, 100*time.Millisecond)
		require.NoError(t, err)

		err = container.CopyTo(ctx, io.NopCloser(nil), "/tmp")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to copy content to container")
	})
}

// TestContainerWaitWithMock tests Container.Wait using a mock Docker client
func TestContainerWaitWithMock(t *testing.T) {
	t.Run("waits for container to complete with exit code 0", func(t *testing.T) {
		mock := &mockDockerClient{
			containerCreateFunc: func(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
				return client.ContainerCreateResult{ID: "container123"}, nil
			},
			containerWaitFunc: func(ctx context.Context, containerID string, options client.ContainerWaitOptions) client.ContainerWaitResult {
				assert.Equal(t, "container123", containerID)
				assert.Equal(t, containertypes.WaitConditionNotRunning, options.Condition)

				errCh := make(chan error, 1)
				resCh := make(chan containertypes.WaitResponse, 1)
				resCh <- containertypes.WaitResponse{StatusCode: 0}
				return client.ContainerWaitResult{Error: errCh, Result: resCh}
			},
		}

		c := docker.NewClient(mock)
		ctx := context.Background()
		image := docker.Image{Name: "alpine:latest"}

		container, err := c.CreateContainer(ctx, "test", image, []string{"echo"}, []string{}, []string{}, "/app", 10, 10, 100*time.Millisecond)
		require.NoError(t, err)

		writer := newMockWriter()
		err = container.Wait(ctx, writer)
		require.NoError(t, err)
		assert.Contains(t, writer.String(), "Container exited with status: 0")
	})

	t.Run("waits for container to complete with non-zero exit code", func(t *testing.T) {
		mock := &mockDockerClient{
			containerCreateFunc: func(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
				return client.ContainerCreateResult{ID: "container123"}, nil
			},
			containerWaitFunc: func(ctx context.Context, containerID string, options client.ContainerWaitOptions) client.ContainerWaitResult {
				errCh := make(chan error, 1)
				resCh := make(chan containertypes.WaitResponse, 1)
				resCh <- containertypes.WaitResponse{StatusCode: 42}
				return client.ContainerWaitResult{Error: errCh, Result: resCh}
			},
		}

		c := docker.NewClient(mock)
		ctx := context.Background()
		image := docker.Image{Name: "alpine:latest"}

		container, err := c.CreateContainer(ctx, "test", image, []string{"sh", "-c", "exit 42"}, []string{}, []string{}, "/app", 10, 10, 100*time.Millisecond)
		require.NoError(t, err)

		writer := newMockWriter()
		err = container.Wait(ctx, writer)
		require.NoError(t, err)
		assert.Contains(t, writer.String(), "Container exited with status: 42")
	})

	t.Run("handles wait error", func(t *testing.T) {
		mock := &mockDockerClient{
			containerCreateFunc: func(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
				return client.ContainerCreateResult{ID: "container123"}, nil
			},
			containerWaitFunc: func(ctx context.Context, containerID string, options client.ContainerWaitOptions) client.ContainerWaitResult {
				errCh := make(chan error, 1)
				resCh := make(chan containertypes.WaitResponse, 1)
				errCh <- errors.New("wait failed")
				return client.ContainerWaitResult{Error: errCh, Result: resCh}
			},
		}

		c := docker.NewClient(mock)
		ctx := context.Background()
		image := docker.Image{Name: "alpine:latest"}

		container, err := c.CreateContainer(ctx, "test", image, []string{"echo"}, []string{}, []string{}, "/app", 10, 10, 100*time.Millisecond)
		require.NoError(t, err)

		writer := newMockWriter()
		err = container.Wait(ctx, writer)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to wait for container")
	})
}
