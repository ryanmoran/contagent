package docker

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/ryanmoran/contagent/internal"
	"github.com/ryanmoran/contagent/internal/runtime"
)

// Compile-time check that Client implements runtime.Runtime.
var _ runtime.Runtime = Client{} //nolint:exhaustruct // Intentional zero value for interface check

type Client struct {
	client DockerClient
}

// NewClient creates a Client that wraps the provided Docker client interface.
func NewClient(dockerClient DockerClient) Client {
	return Client{
		client: dockerClient,
	}
}

// NewDefaultClient creates a Client with a real Docker client from the environment.
func NewDefaultClient() (Client, error) {
	cli, err := client.New(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return Client{}, fmt.Errorf("failed to create docker client: %w\nEnsure Docker is running and DOCKER_HOST is set correctly", err)
	}

	return NewClient(cli), nil
}

// Close closes the underlying Docker client connection.
func (c Client) Close() error {
	return c.client.Close()
}

// HostAddress returns the hostname that containers use to reach the host.
func (c Client) HostAddress() string {
	return "host.docker.internal"
}

// BuildImage builds a Docker image from a Dockerfile and tags it with the specified image name.
// It creates a tar archive containing the Dockerfile, sends it to the Docker daemon, and streams
// the build output to the provided Writer. Returns an error if the Dockerfile cannot be read,
// the tar archive cannot be created, the image build fails, or the build output cannot be decoded.
func (c Client) BuildImage(ctx context.Context, dockerfilePath string, imageName internal.ImageName, w internal.Writer) (runtime.Image, error) {
	dockerfile, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return runtime.Image{}, fmt.Errorf("failed to read Dockerfile at %q: %w\nEnsure the file exists and is readable", dockerfilePath, err)
	}

	pr, pw := io.Pipe()
	defer pr.Close()

	errChan := make(chan error, 1)

	go func() {
		tw := tar.NewWriter(pw)
		defer func() {
			tw.Close()
			pw.Close()
		}()

		header := &tar.Header{
			Name: "Dockerfile",
			Mode: 0644,
			Size: int64(len(dockerfile)),
		}

		if err := tw.WriteHeader(header); err != nil {
			errChan <- fmt.Errorf("failed to write tar header for Dockerfile: %w\nThis is a system error with tar archive creation", err)
			return
		}

		if _, err := tw.Write(dockerfile); err != nil {
			errChan <- fmt.Errorf("failed to write Dockerfile to tar archive: %w\nThis is a system error with tar archive creation", err)
			return
		}
		errChan <- nil
	}()

	response, err := c.client.ImageBuild(ctx, pr, client.ImageBuildOptions{
		Dockerfile: "Dockerfile",
		Tags:       []string{string(imageName)},
		Remove:     true,
	})
	if err != nil {
		return runtime.Image{}, fmt.Errorf("failed to build image %q: %w\nCheck Docker daemon logs for details", imageName, err)
	}
	defer response.Body.Close()

	// Check if tar creation had an error
	select {
	case err := <-errChan:
		if err != nil {
			return runtime.Image{}, err
		}
	case <-ctx.Done():
		return runtime.Image{}, ctx.Err()
	default:
	}

	decoder := json.NewDecoder(response.Body)
	for decoder.More() {
		select {
		case <-ctx.Done():
			return runtime.Image{}, ctx.Err()
		default:
		}

		var output struct {
			Stream      string `json:"stream"`
			ErrorDetail struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"errorDetail"`
		}
		err := decoder.Decode(&output)
		if err != nil {
			return runtime.Image{}, fmt.Errorf("failed to decode build output: %w\nDocker may have returned malformed JSON", err)
		}

		if output.ErrorDetail.Code != 0 {
			return runtime.Image{}, fmt.Errorf("docker build failed: %s\nCheck your Dockerfile syntax and base image availability", output.ErrorDetail.Message)
		}

		w.Print(output.Stream)
	}

	return runtime.Image{
		Name: string(imageName),
	}, nil
}

// CreateContainer creates a new Docker container with the specified configuration.
// It configures the container with TTY support, stdin attachment, environment variables,
// working directory, volume mounts, and network settings to allow communication with the host
// via host.docker.internal. Returns a Container handle or an error if creation fails.
func (c Client) CreateContainer(ctx context.Context, opts runtime.CreateContainerOptions) (runtime.Container, error) {
	response, err := c.client.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Image:        opts.Image.Name,
			Cmd:          []string(opts.Args),
			Tty:          true,
			OpenStdin:    true,
			AttachStdin:  true,
			AttachStdout: true,
			AttachStderr: true,
			Env:          []string(opts.Env),
			WorkingDir:   opts.WorkingDir,
		},
		HostConfig: &container.HostConfig{
			ExtraHosts: []string{
				"host.docker.internal:host-gateway",
			},
			Binds:       opts.Volumes,
			NetworkMode: container.NetworkMode(opts.Network),
		},
		Name:             string(opts.SessionID),
		NetworkingConfig: nil,
		Platform:         nil,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create container %q from image %q: %w\nEnsure image exists and container config is valid", opts.SessionID, opts.Image.Name, err)
	}

	return Container{
		ID:          response.ID,
		Name:        string(opts.SessionID),
		client:      c.client,
		StopTimeout: opts.StopTimeout,
		TTYRetries:  opts.TTYRetries,
		RetryDelay:  opts.RetryDelay,
	}, nil
}

// Ping pings the Docker daemon and returns the API version if successful.
func (c Client) Ping(ctx context.Context) (string, error) {
	ping, err := c.client.Ping(ctx, client.PingOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to ping docker daemon: %w", err)
	}
	return ping.APIVersion, nil
}

// ListContainers lists all containers and returns their IDs.
func (c Client) ListContainers(ctx context.Context) ([]string, error) {
	result, err := c.client.ContainerList(ctx, client.ContainerListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var ids []string
	for _, item := range result.Items {
		ids = append(ids, item.ID)
	}
	return ids, nil
}
