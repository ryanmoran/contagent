package docker

import (
	"context"
	"io"

	"github.com/moby/moby/client"
)

// DockerClient is an interface that wraps the Docker API methods we use.
// This allows for dependency injection and testing with mocks.
//
// The real Docker client (*client.Client from moby/moby/client) implements this interface.
//
// Usage:
//
//	// Production code: use real Docker client
//	dockerClient, err := client.New(client.FromEnv, client.WithAPIVersionNegotiation())
//	if err != nil {
//	    return err
//	}
//	c := docker.NewClient(dockerClient)
//
//	// Or use the convenience function:
//	c, err := docker.NewDefaultClient()
//
//	// Test code: inject a mock
//	type mockDockerClient struct{}
//	func (m *mockDockerClient) ImageBuild(...) { /* mock implementation */ }
//	// ... implement other methods ...
//	c := docker.NewClient(&mockDockerClient{})
type DockerClient interface {
	ImageBuild(ctx context.Context, buildContext io.Reader, options client.ImageBuildOptions) (client.ImageBuildResult, error)
	ContainerCreate(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error)
	ContainerStart(ctx context.Context, containerID string, options client.ContainerStartOptions) (client.ContainerStartResult, error)
	ContainerAttach(ctx context.Context, containerID string, options client.ContainerAttachOptions) (client.ContainerAttachResult, error)
	ContainerWait(ctx context.Context, containerID string, options client.ContainerWaitOptions) client.ContainerWaitResult
	ContainerStop(ctx context.Context, containerID string, options client.ContainerStopOptions) (client.ContainerStopResult, error)
	ContainerRemove(ctx context.Context, containerID string, options client.ContainerRemoveOptions) (client.ContainerRemoveResult, error)
	ContainerResize(ctx context.Context, containerID string, options client.ContainerResizeOptions) (client.ContainerResizeResult, error)
	CopyToContainer(ctx context.Context, containerID string, options client.CopyToContainerOptions) (client.CopyToContainerResult, error)
	Ping(ctx context.Context, options client.PingOptions) (client.PingResult, error)
	ContainerList(ctx context.Context, options client.ContainerListOptions) (client.ContainerListResult, error)
	Close() error
}
