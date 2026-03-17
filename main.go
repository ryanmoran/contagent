package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/ryanmoran/contagent/internal"
	"github.com/ryanmoran/contagent/internal/apple"
	"github.com/ryanmoran/contagent/internal/docker"
	"github.com/ryanmoran/contagent/internal/git"
	"github.com/ryanmoran/contagent/internal/runtime"
)

func main() {
	var exitCode int
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic occurred: %v", r)
			exitCode = 1
		}
		if exitCode != 0 {
			os.Exit(exitCode)
		}
	}()

	if err := run(os.Args, os.Environ()); err != nil {
		log.Println(err)
		exitCode = 1
	}
}

func run(args, env []string) error {
	cleanup := internal.NewCleanupManager()
	defer cleanup.Execute()

	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w\nThis is a system error - check file system permissions", err)
	}

	config, err := internal.ParseConfig(args[1:], env, workingDirectory)
	if err != nil {
		return fmt.Errorf("failed to parse configuration: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cleanup.Add("cancel-context", func() error { cancel(); return nil })

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	w := internal.NewStandardWriter()

	session := internal.GenerateSession()

	gitRoot, err := git.FindRoot(workingDirectory)
	if err != nil {
		return fmt.Errorf("not a git repository: %w", err)
	}

	relPath, err := filepath.Rel(gitRoot, workingDirectory)
	if err != nil {
		return fmt.Errorf("failed to compute relative path from git root: %w", err)
	}

	// containerWorkingDir is where the container starts. When running from a
	// subdirectory, it is config.WorkingDir + relPath. The archive always
	// extracts to config.WorkingDir so the git root path in the container
	// remains stable regardless of where contagent was invoked.
	containerWorkingDir := config.WorkingDir
	if relPath != "." {
		containerWorkingDir = filepath.Join(config.WorkingDir, relPath)
	}

	remote, err := git.NewServer(gitRoot, w)
	if err != nil {
		return fmt.Errorf("failed to start git server in directory %q: %w", gitRoot, err)
	}
	cleanup.Add("git-server", remote.Close)

	// Create runtime based on config
	var rt runtime.Runtime
	switch config.Runtime {
	case "apple":
		rt = apple.NewRuntime()
	case "docker":
		dockerClient, err := docker.NewDefaultClient()
		if err != nil {
			return fmt.Errorf("failed to create docker client: %w\nMake sure Docker is installed and running (try 'docker ps')", err)
		}
		rt = dockerClient
	default:
		return fmt.Errorf("unknown runtime: %q\nSupported runtimes: docker, apple", config.Runtime)
	}
	cleanup.Add("runtime", rt.Close)

	// Validate that Dockerfile path is provided
	if config.DockerfilePath == "" {
		return fmt.Errorf("dockerfile path is required but not specified\n" +
			"Specify it using:\n" +
			"  - CLI flag: --dockerfile ./Dockerfile\n" +
			"  - Config file: Add 'dockerfile: ./Dockerfile' to .contagent.yaml\n" +
			"See .contagent.example.yaml for more details")
	}

	image, err := rt.BuildImage(ctx, config.DockerfilePath, config.ImageName, w)
	if err != nil {
		return fmt.Errorf("failed to build image %q from %q: %w", config.ImageName, config.DockerfilePath, err)
	}

	container, err := rt.CreateContainer(
		ctx,
		runtime.CreateContainerOptions{
			SessionID:   session.ID(),
			Image:       image,
			Args:        config.Args,
			Env:         config.Env,
			Volumes:     config.Volumes,
			WorkingDir:  containerWorkingDir,
			Network:     config.Network,
			StopTimeout: config.StopTimeout,
			TTYRetries:  config.TTYRetries,
			RetryDelay:  config.RetryDelay,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create container %q from image %q: %w", session.ID(), image.Name, err)
	}
	cleanup.Add("container", func() error {
		return container.ForceRemove(ctx)
	})

	imageUser, err := container.InspectUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to inspect user for image %q: %w", image.Name, err)
	}

	// DestDir and CopyTo work in tandem: DestDir is the final path component of
	// config.WorkingDir, and CopyTo receives its parent. Together they cause the
	// archive to be extracted at exactly config.WorkingDir in the container.
	// Both must remain derived from the same config.WorkingDir value.
	archive, err := git.CreateArchive(git.ArchiveOptions{
		Path:         gitRoot,
		Remote:       fmt.Sprintf("http://%s:%d", rt.HostAddress(), remote.Port()),
		Branch:       session.Branch(),
		GitUserName:  config.GitUser.Name,
		GitUserEmail: config.GitUser.Email,
		UID:          imageUser.UID,
		GID:          imageUser.GID,
		DestDir:      filepath.Base(config.WorkingDir),
	}, w)
	if err != nil {
		return fmt.Errorf("failed to create git archive from %q on branch %q: %w", workingDirectory, session.Branch(), err)
	}
	cleanup.Add("archive", archive.Close)

	err = container.CopyTo(ctx, archive, filepath.Dir(config.WorkingDir))
	if err != nil {
		return fmt.Errorf("failed to copy git archive to container %q: %w", session.ID(), err)
	}

	err = container.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start container %q: %w", session.ID(), err)
	}

	err = container.Attach(ctx, cancel, w)
	if err != nil {
		return fmt.Errorf("failed to attach to container %q: %w\nThis may indicate a TTY configuration issue", session.ID(), err)
	}

	err = container.Wait(ctx, w)
	if err != nil {
		return fmt.Errorf("failed to wait for container %q: %w", session.ID(), err)
	}

	return nil
}
