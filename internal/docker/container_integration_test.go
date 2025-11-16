//go:build integration
// +build integration

package docker_test

import (
	"archive/tar"
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ryanmoran/contagent/internal/docker"
	"github.com/stretchr/testify/require"
)

// TestContainerStart tests starting a container
func TestContainerStart(t *testing.T) {
	client, err := docker.NewDefaultClient()
	if err != nil {
		t.Skip("Docker not available:", err)
	}
	defer client.Close()

	t.Run("starts container successfully", func(t *testing.T) {
		ctx := context.Background()

		image := docker.Image{Name: "alpine:latest"}
		args := []string{"sleep", "10"}
		env := []string{}
		volumes := []string{}
		workingDir := "/app"

		container, err := client.CreateContainer(ctx, "test-start", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
		require.NoError(t, err)
		defer func() {
			_ = container.ForceRemove(ctx)
		}()

		err = container.Start(ctx)
		require.NoError(t, err)
	})

	t.Run("fails to start already removed container", func(t *testing.T) {
		ctx := context.Background()

		image := docker.Image{Name: "alpine:latest"}
		args := []string{"echo", "test"}
		env := []string{}
		volumes := []string{}
		workingDir := "/app"

		container, err := client.CreateContainer(ctx, "test-start-fail", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
		require.NoError(t, err)

		err = container.ForceRemove(ctx)
		require.NoError(t, err)

		err = container.Start(ctx)
		require.ErrorContains(t, err, "failed to start container")
	})
}

// TestContainerRemove tests container removal
func TestContainerRemove(t *testing.T) {
	client, err := docker.NewDefaultClient()
	if err != nil {
		t.Skip("Docker not available:", err)
	}
	defer client.Close()

	t.Run("removes stopped container", func(t *testing.T) {
		ctx := context.Background()

		image := docker.Image{Name: "alpine:latest"}
		args := []string{"echo", "test"}
		env := []string{}
		volumes := []string{}
		workingDir := "/app"

		container, err := client.CreateContainer(ctx, "test-remove", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
		require.NoError(t, err)

		err = container.Remove(ctx)
		require.NoError(t, err)
	})

	t.Run("fails to remove non-existent container", func(t *testing.T) {
		ctx := context.Background()

		image := docker.Image{Name: "alpine:latest"}
		args := []string{"echo", "test"}
		env := []string{}
		volumes := []string{}
		workingDir := "/app"

		container, err := client.CreateContainer(ctx, "test-remove-fail", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
		require.NoError(t, err)

		err = container.Remove(ctx)
		require.NoError(t, err)

		err = container.Remove(ctx)
		require.ErrorContains(t, err, "failed to remove container")
	})
}

// TestContainerForceRemove tests force removal
func TestContainerForceRemove(t *testing.T) {
	client, err := docker.NewDefaultClient()
	if err != nil {
		t.Skip("Docker not available:", err)
	}
	defer client.Close()

	t.Run("force removes running container", func(t *testing.T) {
		ctx := context.Background()

		image := docker.Image{Name: "alpine:latest"}
		args := []string{"sleep", "60"}
		env := []string{}
		volumes := []string{}
		workingDir := "/app"

		container, err := client.CreateContainer(ctx, "test-force-remove", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
		require.NoError(t, err)

		err = container.Start(ctx)
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond)

		err = container.ForceRemove(ctx)
		require.NoError(t, err)
	})

	t.Run("force removes stopped container", func(t *testing.T) {
		ctx := context.Background()

		image := docker.Image{Name: "alpine:latest"}
		args := []string{"echo", "test"}
		env := []string{}
		volumes := []string{}
		workingDir := "/app"

		container, err := client.CreateContainer(ctx, "test-force-remove-stopped", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
		require.NoError(t, err)

		err = container.ForceRemove(ctx)
		require.NoError(t, err)
	})

	t.Run("fails to force remove non-existent container", func(t *testing.T) {
		ctx := context.Background()

		image := docker.Image{Name: "alpine:latest"}
		args := []string{"echo", "test"}
		env := []string{}
		volumes := []string{}
		workingDir := "/app"

		container, err := client.CreateContainer(ctx, "test-force-remove-nonexist", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
		require.NoError(t, err)

		err = container.ForceRemove(ctx)
		require.NoError(t, err)

		err = container.ForceRemove(ctx)
		require.Error(t, err)
	})
}

// TestContainerCopyTo tests copying files to container
func TestContainerCopyTo(t *testing.T) {
	client, err := docker.NewDefaultClient()
	if err != nil {
		t.Skip("Docker not available:", err)
	}
	defer client.Close()

	t.Run("copies tar archive to container", func(t *testing.T) {
		ctx := context.Background()

		image := docker.Image{Name: "alpine:latest"}
		args := []string{"sleep", "10"}
		env := []string{}
		volumes := []string{}
		workingDir := "/app"

		container, err := client.CreateContainer(ctx, "test-copy", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
		require.NoError(t, err)
		defer func() {
			_ = container.ForceRemove(ctx)
		}()

		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)

		content := []byte("test content")
		header := &tar.Header{
			Name: "test.txt",
			Mode: 0644,
			Size: int64(len(content)),
		}
		err = tw.WriteHeader(header)
		require.NoError(t, err)

		_, err = tw.Write(content)
		require.NoError(t, err)

		err = tw.Close()
		require.NoError(t, err)

		err = container.CopyTo(ctx, &buf, "/tmp")
		require.NoError(t, err)
	})

	t.Run("fails to copy to non-existent container", func(t *testing.T) {
		ctx := context.Background()

		image := docker.Image{Name: "alpine:latest"}
		args := []string{"echo", "test"}
		env := []string{}
		volumes := []string{}
		workingDir := "/app"

		container, err := client.CreateContainer(ctx, "test-copy-fail", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
		require.NoError(t, err)

		err = container.ForceRemove(ctx)
		require.NoError(t, err)

		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)
		tw.Close()

		err = container.CopyTo(ctx, &buf, "/tmp")
		require.ErrorContains(t, err, "failed to copy to container")
	})

	t.Run("copies empty archive", func(t *testing.T) {
		ctx := context.Background()

		image := docker.Image{Name: "alpine:latest"}
		args := []string{"sleep", "10"}
		env := []string{}
		volumes := []string{}
		workingDir := "/app"

		container, err := client.CreateContainer(ctx, "test-copy-empty", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
		require.NoError(t, err)
		defer func() {
			_ = container.ForceRemove(ctx)
		}()

		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)
		err = tw.Close()
		require.NoError(t, err)

		err = container.CopyTo(ctx, &buf, "/tmp")
		require.NoError(t, err)
	})
}

// TestContainerWait tests waiting for container completion
func TestContainerWait(t *testing.T) {
	client, err := docker.NewDefaultClient()
	if err != nil {
		t.Skip("Docker not available:", err)
	}
	defer client.Close()

	t.Run("waits for container to exit", func(t *testing.T) {
		ctx := context.Background()

		image := docker.Image{Name: "alpine:latest"}
		args := []string{"sh", "-c", "sleep 0.5 && exit 0"}
		env := []string{}
		volumes := []string{}
		workingDir := "/app"

		container, err := client.CreateContainer(ctx, "test-wait", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
		require.NoError(t, err)
		defer func() {
			_ = container.ForceRemove(ctx)
		}()

		err = container.Start(ctx)
		require.NoError(t, err)

		writer := newMockWriter()
		err = container.Wait(ctx, writer)
		require.NoError(t, err)

		require.Contains(t, writer.String(), "Container exited with status: 0")
	})

	t.Run("reports non-zero exit status", func(t *testing.T) {
		ctx := context.Background()

		image := docker.Image{Name: "alpine:latest"}
		args := []string{"sh", "-c", "exit 42"}
		env := []string{}
		volumes := []string{}
		workingDir := "/app"

		container, err := client.CreateContainer(ctx, "test-wait-fail", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
		require.NoError(t, err)
		defer func() {
			_ = container.ForceRemove(ctx)
		}()

		err = container.Start(ctx)
		require.NoError(t, err)

		writer := newMockWriter()
		err = container.Wait(ctx, writer)
		require.NoError(t, err)

		require.Contains(t, writer.String(), "Container exited with status: 42")
	})
}

// TestContainerAttach tests attaching to container (basic validation)
func TestContainerAttach(t *testing.T) {
	client, err := docker.NewDefaultClient()
	if err != nil {
		t.Skip("Docker not available:", err)
	}
	defer client.Close()

	t.Run("fails to attach to non-running container", func(t *testing.T) {
		ctx := context.Background()

		image := docker.Image{Name: "alpine:latest"}
		args := []string{"echo", "test"}
		env := []string{}
		volumes := []string{}
		workingDir := "/app"

		container, err := client.CreateContainer(ctx, "test-attach-fail", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
		require.NoError(t, err)
		defer func() {
			_ = container.ForceRemove(ctx)
		}()

		err = container.ForceRemove(ctx)
		require.NoError(t, err)

		writer := newMockWriter()
		err = container.Attach(ctx, writer)
		require.Error(t, err)
	})
}

// TestContainerStruct tests the Container struct fields
func TestContainerStruct(t *testing.T) {
	client, err := docker.NewDefaultClient()
	if err != nil {
		t.Skip("Docker not available:", err)
	}
	defer client.Close()

	t.Run("container has expected fields", func(t *testing.T) {
		ctx := context.Background()

		image := docker.Image{Name: "alpine:latest"}
		args := []string{"echo", "test"}
		env := []string{}
		volumes := []string{}
		workingDir := "/app"

		container, err := client.CreateContainer(ctx, "test-struct", image, args, env, volumes, workingDir, 5, 3, 50*time.Millisecond)
		require.NoError(t, err)
		defer func() {
			_ = container.ForceRemove(ctx)
		}()

		require.NotEmpty(t, container.ID)
		require.Equal(t, "test-struct", container.Name)
	})
}

// TestContainerConfiguration tests various container configurations
func TestContainerConfiguration(t *testing.T) {
	client, err := docker.NewDefaultClient()
	if err != nil {
		t.Skip("Docker not available:", err)
	}
	defer client.Close()

	t.Run("creates container with environment variables", func(t *testing.T) {
		ctx := context.Background()

		image := docker.Image{Name: "alpine:latest"}
		args := []string{"sh", "-c", "echo $TEST_VAR"}
		env := []string{"TEST_VAR=hello"}
		volumes := []string{}
		workingDir := "/app"

		container, err := client.CreateContainer(ctx, "test-env", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
		require.NoError(t, err)
		defer func() {
			_ = container.ForceRemove(ctx)
		}()

		require.NotEmpty(t, container.ID)
	})

	t.Run("creates container with custom working directory", func(t *testing.T) {
		ctx := context.Background()

		image := docker.Image{Name: "alpine:latest"}
		args := []string{"pwd"}
		env := []string{}
		volumes := []string{}
		workingDir := "/custom"

		container, err := client.CreateContainer(ctx, "test-workdir", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
		require.NoError(t, err)
		defer func() {
			_ = container.ForceRemove(ctx)
		}()

		require.NotEmpty(t, container.ID)
	})

	t.Run("creates container with multiple args", func(t *testing.T) {
		ctx := context.Background()

		image := docker.Image{Name: "alpine:latest"}
		args := []string{"sh", "-c", "echo arg1 && echo arg2 && echo arg3"}
		env := []string{}
		volumes := []string{}
		workingDir := "/app"

		container, err := client.CreateContainer(ctx, "test-args", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
		require.NoError(t, err)
		defer func() {
			_ = container.ForceRemove(ctx)
		}()

		require.NotEmpty(t, container.ID)
	})
}

// TestContainerLifecycle tests the full lifecycle
func TestContainerLifecycle(t *testing.T) {
	client, err := docker.NewDefaultClient()
	if err != nil {
		t.Skip("Docker not available:", err)
	}
	defer client.Close()

	t.Run("create, start, wait, remove workflow", func(t *testing.T) {
		ctx := context.Background()

		image := docker.Image{Name: "alpine:latest"}
		args := []string{"sh", "-c", "echo 'lifecycle test' && sleep 0.2"}
		env := []string{}
		volumes := []string{}
		workingDir := "/app"

		container, err := client.CreateContainer(ctx, "test-lifecycle", image, args, env, volumes, workingDir, 10, 10, 100*time.Millisecond)
		require.NoError(t, err)

		err = container.Start(ctx)
		require.NoError(t, err)

		writer := newMockWriter()
		err = container.Wait(ctx, writer)
		require.NoError(t, err)

		output := writer.String()
		require.True(t, strings.Contains(output, "Container exited with status: 0") || strings.Contains(output, "Received signal"))

		err = container.Remove(ctx)
		require.NoError(t, err)
	})
}
