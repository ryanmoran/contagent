package docker_test

import (
	"context"
	"errors"
	"io"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// mockDockerClient is a mock implementation of docker.DockerClient for testing
type mockDockerClient struct {
	imageBuildFunc      func(ctx context.Context, buildContext io.Reader, options client.ImageBuildOptions) (client.ImageBuildResult, error)
	containerCreateFunc func(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error)
	containerStartFunc  func(ctx context.Context, containerID string, options client.ContainerStartOptions) (client.ContainerStartResult, error)
	containerAttachFunc func(ctx context.Context, containerID string, options client.ContainerAttachOptions) (client.ContainerAttachResult, error)
	containerWaitFunc   func(ctx context.Context, containerID string, options client.ContainerWaitOptions) client.ContainerWaitResult
	containerStopFunc   func(ctx context.Context, containerID string, options client.ContainerStopOptions) (client.ContainerStopResult, error)
	containerRemoveFunc func(ctx context.Context, containerID string, options client.ContainerRemoveOptions) (client.ContainerRemoveResult, error)
	containerResizeFunc func(ctx context.Context, containerID string, options client.ContainerResizeOptions) (client.ContainerResizeResult, error)
	copyToContainerFunc func(ctx context.Context, containerID string, options client.CopyToContainerOptions) (client.CopyToContainerResult, error)
	pingFunc            func(ctx context.Context, options client.PingOptions) (client.PingResult, error)
	containerListFunc   func(ctx context.Context, options client.ContainerListOptions) (client.ContainerListResult, error)
	closeFunc           func() error
}

func (m *mockDockerClient) ImageBuild(ctx context.Context, buildContext io.Reader, options client.ImageBuildOptions) (client.ImageBuildResult, error) {
	if m.imageBuildFunc != nil {
		return m.imageBuildFunc(ctx, buildContext, options)
	}
	return client.ImageBuildResult{}, errors.New("not implemented")
}

func (m *mockDockerClient) ContainerCreate(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
	if m.containerCreateFunc != nil {
		return m.containerCreateFunc(ctx, options)
	}
	return client.ContainerCreateResult{}, errors.New("not implemented")
}

func (m *mockDockerClient) ContainerStart(ctx context.Context, containerID string, options client.ContainerStartOptions) (client.ContainerStartResult, error) {
	if m.containerStartFunc != nil {
		return m.containerStartFunc(ctx, containerID, options)
	}
	return client.ContainerStartResult{}, errors.New("not implemented")
}

func (m *mockDockerClient) ContainerAttach(ctx context.Context, containerID string, options client.ContainerAttachOptions) (client.ContainerAttachResult, error) {
	if m.containerAttachFunc != nil {
		return m.containerAttachFunc(ctx, containerID, options)
	}
	return client.ContainerAttachResult{}, errors.New("not implemented")
}

func (m *mockDockerClient) ContainerWait(ctx context.Context, containerID string, options client.ContainerWaitOptions) client.ContainerWaitResult {
	if m.containerWaitFunc != nil {
		return m.containerWaitFunc(ctx, containerID, options)
	}
	errCh := make(chan error, 1)
	resCh := make(chan containertypes.WaitResponse, 1)
	errCh <- errors.New("not implemented")
	return client.ContainerWaitResult{Error: errCh, Result: resCh}
}

func (m *mockDockerClient) ContainerStop(ctx context.Context, containerID string, options client.ContainerStopOptions) (client.ContainerStopResult, error) {
	if m.containerStopFunc != nil {
		return m.containerStopFunc(ctx, containerID, options)
	}
	return client.ContainerStopResult{}, errors.New("not implemented")
}

func (m *mockDockerClient) ContainerRemove(ctx context.Context, containerID string, options client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
	if m.containerRemoveFunc != nil {
		return m.containerRemoveFunc(ctx, containerID, options)
	}
	return client.ContainerRemoveResult{}, errors.New("not implemented")
}

func (m *mockDockerClient) ContainerResize(ctx context.Context, containerID string, options client.ContainerResizeOptions) (client.ContainerResizeResult, error) {
	if m.containerResizeFunc != nil {
		return m.containerResizeFunc(ctx, containerID, options)
	}
	return client.ContainerResizeResult{}, errors.New("not implemented")
}

func (m *mockDockerClient) CopyToContainer(ctx context.Context, containerID string, options client.CopyToContainerOptions) (client.CopyToContainerResult, error) {
	if m.copyToContainerFunc != nil {
		return m.copyToContainerFunc(ctx, containerID, options)
	}
	return client.CopyToContainerResult{}, errors.New("not implemented")
}

func (m *mockDockerClient) Ping(ctx context.Context, options client.PingOptions) (client.PingResult, error) {
	if m.pingFunc != nil {
		return m.pingFunc(ctx, options)
	}
	return client.PingResult{}, errors.New("not implemented")
}

func (m *mockDockerClient) ContainerList(ctx context.Context, options client.ContainerListOptions) (client.ContainerListResult, error) {
	if m.containerListFunc != nil {
		return m.containerListFunc(ctx, options)
	}
	return client.ContainerListResult{}, errors.New("not implemented")
}

func (m *mockDockerClient) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}
