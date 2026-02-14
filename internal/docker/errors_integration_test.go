//go:build integration
// +build integration

package docker_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ryanmoran/contagent/internal/docker"
	"github.com/ryanmoran/contagent/internal/runtime"
	"github.com/stretchr/testify/require"
)

// TestDockerErrorCases tests various error scenarios in the docker package
func TestDockerErrorCases(t *testing.T) {
	t.Run("BuildImage error cases", func(t *testing.T) {
		client, err := docker.NewDefaultClient()
		if err != nil {
			t.Skip("Docker not available:", err)
		}
		defer client.Close()

		t.Run("non-existent Dockerfile", func(t *testing.T) {
			writer := newMockWriter()
			ctx := context.Background()

			_, err := client.BuildImage(ctx, "/nonexistent/path/Dockerfile", "test:latest", writer)
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed to read Dockerfile")
		})

		t.Run("Dockerfile in non-existent directory", func(t *testing.T) {
			writer := newMockWriter()
			ctx := context.Background()

			_, err := client.BuildImage(ctx, "/path/that/does/not/exist/Dockerfile", "test:latest", writer)
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed to read Dockerfile")
		})

		t.Run("empty Dockerfile", func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "docker-empty-test")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
			err = os.WriteFile(dockerfilePath, []byte(""), 0644)
			require.NoError(t, err)

			writer := newMockWriter()
			ctx := context.Background()

			_, err = client.BuildImage(ctx, dockerfilePath, "test:latest", writer)
			require.Error(t, err)
		})

		t.Run("Dockerfile with FROM referencing non-existent image", func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "docker-badimage-test")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
			err = os.WriteFile(dockerfilePath, []byte("FROM nonexistent-image-xyz-12345:latest\n"), 0644)
			require.NoError(t, err)

			writer := newMockWriter()
			ctx := context.Background()

			_, err = client.BuildImage(ctx, dockerfilePath, "test:latest", writer)
			require.Error(t, err)
		})

		t.Run("invalid image name", func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "docker-badname-test")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
			err = os.WriteFile(dockerfilePath, []byte("FROM alpine:latest\n"), 0644)
			require.NoError(t, err)

			writer := newMockWriter()
			ctx := context.Background()

			_, err = client.BuildImage(ctx, dockerfilePath, "INVALID_IMAGE_NAME:@#$", writer)
			require.Error(t, err)
		})

		t.Run("Dockerfile with permission denied", func(t *testing.T) {
			if os.Getuid() == 0 {
				t.Skip("Running as root, cannot test permission denied")
			}

			tmpDir, err := os.MkdirTemp("", "docker-perm-test")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
			err = os.WriteFile(dockerfilePath, []byte("FROM alpine:latest\n"), 0000)
			require.NoError(t, err)

			writer := newMockWriter()
			ctx := context.Background()

			_, err = client.BuildImage(ctx, dockerfilePath, "test:latest", writer)
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed to read Dockerfile")
		})

		t.Run("cancelled context during build", func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "docker-cancel-test")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
			err = os.WriteFile(dockerfilePath, []byte("FROM alpine:latest\nRUN sleep 30\n"), 0644)
			require.NoError(t, err)

			writer := newMockWriter()
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			_, err = client.BuildImage(ctx, dockerfilePath, "test-cancel:latest", writer)
			require.Error(t, err)
			require.True(t, err == context.DeadlineExceeded || err == context.Canceled ||
				(err != nil && (err.Error() != "")),
				"expected context error or build error")
		})
	})

	t.Run("CreateContainer error cases", func(t *testing.T) {
		client, err := docker.NewDefaultClient()
		if err != nil {
			t.Skip("Docker not available:", err)
		}
		defer client.Close()

		t.Run("non-existent image", func(t *testing.T) {
			ctx := context.Background()

			_, err := client.CreateContainer(ctx, runtime.CreateContainerOptions{
				SessionID:   "test-noimage",
				Image:       runtime.Image{Name: "nonexistent-image-xyz-abc-12345:latest"},
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

		t.Run("empty image name", func(t *testing.T) {
			ctx := context.Background()

			_, err := client.CreateContainer(ctx, runtime.CreateContainerOptions{
				SessionID:   "test-emptyimage",
				Image:       runtime.Image{Name: ""},
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

		t.Run("invalid volume mount", func(t *testing.T) {
			ctx := context.Background()

			_, err := client.CreateContainer(ctx, runtime.CreateContainerOptions{
				SessionID:   "test-badvol",
				Image:       runtime.Image{Name: "alpine:latest"},
				Args:        []string{"echo", "test"},
				Env:         []string{},
				Volumes:     []string{"/nonexistent/path"},
				WorkingDir:  "/app",
				Network:     "default",
				StopTimeout: 10,
				TTYRetries:  10,
				RetryDelay:  100 * time.Millisecond,
			})
			if err != nil {
				require.Contains(t, err.Error(), "failed to create container")
			}
		})

		t.Run("cancelled context", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			_, err := client.CreateContainer(ctx, runtime.CreateContainerOptions{
				SessionID:   "test-cancelled",
				Image:       runtime.Image{Name: "alpine:latest"},
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
		})

		t.Run("duplicate container name", func(t *testing.T) {
			ctx := context.Background()

			container1, err := client.CreateContainer(ctx, runtime.CreateContainerOptions{
				SessionID:   "test-duplicate",
				Image:       runtime.Image{Name: "alpine:latest"},
				Args:        []string{"sleep", "10"},
				Env:         []string{},
				Volumes:     []string{},
				WorkingDir:  "/app",
				Network:     "default",
				StopTimeout: 10,
				TTYRetries:  10,
				RetryDelay:  100 * time.Millisecond,
			})
			require.NoError(t, err)
			defer func() {
				_ = container1.ForceRemove(ctx)
			}()

			_, err = client.CreateContainer(ctx, runtime.CreateContainerOptions{
				SessionID:   "test-duplicate",
				Image:       runtime.Image{Name: "alpine:latest"},
				Args:        []string{"sleep", "10"},
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
	})

	t.Run("Container operation error cases", func(t *testing.T) {
		client, err := docker.NewDefaultClient()
		if err != nil {
			t.Skip("Docker not available:", err)
		}
		defer client.Close()

		t.Run("start non-existent container", func(t *testing.T) {
			ctx := context.Background()

			container, err := client.CreateContainer(ctx, integrationOpts("test-start-noexist", []string{"echo", "test"}))
			require.NoError(t, err)

			err = container.ForceRemove(ctx)
			require.NoError(t, err)

			err = container.Start(ctx)
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed to start container")
		})

		t.Run("copy to non-existent container", func(t *testing.T) {
			ctx := context.Background()

			container, err := client.CreateContainer(ctx, integrationOpts("test-copy-noexist", []string{"echo", "test"}))
			require.NoError(t, err)

			err = container.ForceRemove(ctx)
			require.NoError(t, err)

			err = container.CopyTo(ctx, nil, "/tmp")
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed to copy content to container")
		})

		t.Run("remove already removed container", func(t *testing.T) {
			ctx := context.Background()

			container, err := client.CreateContainer(ctx, integrationOpts("test-double-remove", []string{"echo", "test"}))
			require.NoError(t, err)

			dc := container.(docker.Container)
			err = dc.Remove(ctx)
			require.NoError(t, err)

			err = dc.Remove(ctx)
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed to remove container")
		})

		t.Run("attach to removed container", func(t *testing.T) {
			ctx := context.Background()

			container, err := client.CreateContainer(ctx, integrationOpts("test-attach-removed", []string{"echo", "test"}))
			require.NoError(t, err)

			err = container.ForceRemove(ctx)
			require.NoError(t, err)

			writer := newMockWriter()
			err = container.Attach(ctx, writer)
			require.Error(t, err)
		})

		t.Run("wait on removed container", func(t *testing.T) {
			ctx := context.Background()

			container, err := client.CreateContainer(ctx, integrationOpts("test-wait-removed", []string{"echo", "test"}))
			require.NoError(t, err)

			err = container.ForceRemove(ctx)
			require.NoError(t, err)

			writer := newMockWriter()
			err = container.Wait(ctx, writer)
			require.Error(t, err)
		})

		t.Run("start container with cancelled context", func(t *testing.T) {
			ctx := context.Background()

			container, err := client.CreateContainer(ctx, integrationOpts("test-start-cancel", []string{"sleep", "10"}))
			require.NoError(t, err)
			defer func() {
				_ = container.ForceRemove(ctx)
			}()

			cancelCtx, cancel := context.WithCancel(context.Background())
			cancel()

			err = container.Start(cancelCtx)
			require.Error(t, err)
		})
	})

	t.Run("Client lifecycle error cases", func(t *testing.T) {
		t.Run("multiple Close calls don't panic", func(t *testing.T) {
			client, err := docker.NewDefaultClient()
			if err != nil {
				t.Skip("Docker not available:", err)
			}

			require.NotPanics(t, func() {
				client.Close()
				client.Close()
				client.Close()
			})
		})
	})
}
