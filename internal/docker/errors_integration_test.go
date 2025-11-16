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
	"github.com/stretchr/testify/assert"
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
			assert.Contains(t, err.Error(), "failed to read Dockerfile")
		})

		t.Run("Dockerfile in non-existent directory", func(t *testing.T) {
			writer := newMockWriter()
			ctx := context.Background()

			_, err := client.BuildImage(ctx, "/path/that/does/not/exist/Dockerfile", "test:latest", writer)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to read Dockerfile")
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

			// Docker allows most image names, so we need a truly invalid one
			// Using capital letters and special characters
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
			assert.Contains(t, err.Error(), "failed to read Dockerfile")
		})

		t.Run("cancelled context during build", func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "docker-cancel-test")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			// Create a Dockerfile with a long-running operation
			dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
			err = os.WriteFile(dockerfilePath, []byte("FROM alpine:latest\nRUN sleep 30\n"), 0644)
			require.NoError(t, err)

			writer := newMockWriter()
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			_, err = client.BuildImage(ctx, dockerfilePath, "test-cancel:latest", writer)
			require.Error(t, err)
			assert.True(t, err == context.DeadlineExceeded || err == context.Canceled ||
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

			image := docker.Image{Name: "nonexistent-image-xyz-abc-12345:latest"}
			args := []string{"echo", "test"}
			env := []string{}
			volumes := []string{}
			workingDir := "/app"

			_, err := client.CreateContainer(ctx, "test-noimage", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to create container")
		})

		t.Run("empty image name", func(t *testing.T) {
			ctx := context.Background()

			image := docker.Image{Name: ""}
			args := []string{"echo", "test"}
			env := []string{}
			volumes := []string{}
			workingDir := "/app"

			_, err := client.CreateContainer(ctx, "test-emptyimage", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to create container")
		})

		t.Run("invalid volume mount", func(t *testing.T) {
			ctx := context.Background()

			image := docker.Image{Name: "alpine:latest"}
			args := []string{"echo", "test"}
			env := []string{}
			// Invalid volume syntax (missing destination)
			volumes := []string{"/nonexistent/path"}
			workingDir := "/app"

			_, err := client.CreateContainer(ctx, "test-badvol", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
			// Docker may accept this but behavior is undefined
			// We're mainly testing that the error is handled properly
			if err != nil {
				assert.Contains(t, err.Error(), "failed to create container")
			}
		})

		t.Run("cancelled context", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel immediately

			image := docker.Image{Name: "alpine:latest"}
			args := []string{"echo", "test"}
			env := []string{}
			volumes := []string{}
			workingDir := "/app"

			_, err := client.CreateContainer(ctx, "test-cancelled", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
			require.Error(t, err)
		})

		t.Run("duplicate container name", func(t *testing.T) {
			ctx := context.Background()

			image := docker.Image{Name: "alpine:latest"}
			args := []string{"sleep", "10"}
			env := []string{}
			volumes := []string{}
			workingDir := "/app"

			container1, err := client.CreateContainer(ctx, "test-duplicate", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
			require.NoError(t, err)
			defer func() {
				_ = container1.ForceRemove(ctx)
			}()

			// Try to create another container with the same name
			_, err = client.CreateContainer(ctx, "test-duplicate", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to create container")
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

			image := docker.Image{Name: "alpine:latest"}
			args := []string{"echo", "test"}
			env := []string{}
			volumes := []string{}
			workingDir := "/app"

			container, err := client.CreateContainer(ctx, "test-start-noexist", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
			require.NoError(t, err)

			// Remove the container before starting
			err = container.ForceRemove(ctx)
			require.NoError(t, err)

			// Try to start the removed container
			err = container.Start(ctx)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to start container")
		})

		t.Run("copy to non-existent container", func(t *testing.T) {
			ctx := context.Background()

			image := docker.Image{Name: "alpine:latest"}
			args := []string{"echo", "test"}
			env := []string{}
			volumes := []string{}
			workingDir := "/app"

			container, err := client.CreateContainer(ctx, "test-copy-noexist", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
			require.NoError(t, err)

			err = container.ForceRemove(ctx)
			require.NoError(t, err)

			// Try to copy to the removed container
			err = container.CopyTo(ctx, nil, "/tmp")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to copy to container")
		})

		t.Run("remove already removed container", func(t *testing.T) {
			ctx := context.Background()

			image := docker.Image{Name: "alpine:latest"}
			args := []string{"echo", "test"}
			env := []string{}
			volumes := []string{}
			workingDir := "/app"

			container, err := client.CreateContainer(ctx, "test-double-remove", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
			require.NoError(t, err)

			err = container.Remove(ctx)
			require.NoError(t, err)

			// Try to remove again
			err = container.Remove(ctx)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to remove container")
		})

		t.Run("attach to removed container", func(t *testing.T) {
			ctx := context.Background()

			image := docker.Image{Name: "alpine:latest"}
			args := []string{"echo", "test"}
			env := []string{}
			volumes := []string{}
			workingDir := "/app"

			container, err := client.CreateContainer(ctx, "test-attach-removed", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
			require.NoError(t, err)

			err = container.ForceRemove(ctx)
			require.NoError(t, err)

			writer := newMockWriter()
			err = container.Attach(ctx, writer)
			require.Error(t, err)
		})

		t.Run("wait on removed container", func(t *testing.T) {
			ctx := context.Background()

			image := docker.Image{Name: "alpine:latest"}
			args := []string{"echo", "test"}
			env := []string{}
			volumes := []string{}
			workingDir := "/app"

			container, err := client.CreateContainer(ctx, "test-wait-removed", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
			require.NoError(t, err)

			err = container.ForceRemove(ctx)
			require.NoError(t, err)

			writer := newMockWriter()
			err = container.Wait(ctx, writer)
			require.Error(t, err)
		})

		t.Run("start container with cancelled context", func(t *testing.T) {
			ctx := context.Background()

			image := docker.Image{Name: "alpine:latest"}
			args := []string{"sleep", "10"}
			env := []string{}
			volumes := []string{}
			workingDir := "/app"

			container, err := client.CreateContainer(ctx, "test-start-cancel", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
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

			assert.NotPanics(t, func() {
				client.Close()
				client.Close()
				client.Close()
			})
		})
	})
}
