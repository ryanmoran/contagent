package docker

import (
	"archive/tar"
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/moby/moby/client"
	"github.com/ryanmoran/contagent/internal/runtime"
)

// resolveImageUser parses the Docker USER field and resolves it to uid/gid.
// The USER field format is "[user][:group]" where each part can be a name or numeric ID.
// If names are present, they are resolved via /etc/passwd and /etc/group copied from
// the container with the given containerID (which may be stopped).
func resolveImageUser(ctx context.Context, dockerClient DockerClient, containerID, userStr string) (runtime.ImageUser, error) {
	if userStr == "" {
		return runtime.ImageUser{UID: 0, GID: 0}, nil
	}

	userPart, groupPart, hasGroup := strings.Cut(userStr, ":")

	userUID, userIsNumeric := tryParseInt(userPart)
	groupGID, groupIsNumeric := tryParseInt(groupPart)

	// Fast path: both parts are numeric (or group is absent with a numeric user)
	if userIsNumeric && (!hasGroup || groupIsNumeric) {
		gid := userUID // default gid = uid when no group specified
		if hasGroup {
			gid = groupGID
		}
		return runtime.ImageUser{UID: userUID, GID: gid}, nil
	}

	// Slow path: resolve names via /etc/passwd and /etc/group from the container
	var uid, gid int
	var err error

	if userIsNumeric {
		uid = userUID
		gid = userUID // fallback; may be overridden below
	} else {
		uid, gid, err = lookupUser(ctx, dockerClient, containerID, userPart)
		if err != nil {
			return runtime.ImageUser{}, fmt.Errorf("failed to resolve user %q: %w", userPart, err)
		}
	}

	if hasGroup {
		if groupIsNumeric {
			gid = groupGID
		} else {
			gid, err = lookupGroup(ctx, dockerClient, containerID, groupPart)
			if err != nil {
				return runtime.ImageUser{}, fmt.Errorf("failed to resolve group %q: %w", groupPart, err)
			}
		}
	}

	return runtime.ImageUser{UID: uid, GID: gid}, nil
}

func tryParseInt(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}

func copyFileFromContainer(ctx context.Context, dockerClient DockerClient, containerID, srcPath string) (string, error) {
	result, err := dockerClient.CopyFromContainer(ctx, containerID, client.CopyFromContainerOptions{
		SourcePath: srcPath,
	})
	if err != nil {
		return "", fmt.Errorf("failed to copy %q from container: %w", srcPath, err)
	}
	defer result.Content.Close()

	tr := tar.NewReader(result.Content)
	if _, err = tr.Next(); err != nil {
		return "", fmt.Errorf("failed to read tar entry from container copy: %w", err)
	}

	content, err := io.ReadAll(tr)
	if err != nil {
		return "", fmt.Errorf("failed to read file content: %w", err)
	}

	return string(content), nil
}

// lookupUser finds a username in /etc/passwd and returns its uid and primary gid.
func lookupUser(ctx context.Context, dockerClient DockerClient, containerID, username string) (uid, gid int, err error) {
	content, err := copyFileFromContainer(ctx, dockerClient, containerID, "/etc/passwd")
	if err != nil {
		return 0, 0, err
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.SplitN(line, ":", 7)
		if len(fields) < 4 || fields[0] != username {
			continue
		}
		uid, err = strconv.Atoi(fields[2])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid uid for user %q in /etc/passwd: %w", username, err)
		}
		gid, err = strconv.Atoi(fields[3])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid gid for user %q in /etc/passwd: %w", username, err)
		}
		return uid, gid, nil
	}
	return 0, 0, fmt.Errorf("user %q not found in /etc/passwd", username)
}

// lookupGroup finds a group name in /etc/group and returns its gid.
func lookupGroup(ctx context.Context, dockerClient DockerClient, containerID, groupName string) (gid int, err error) {
	content, err := copyFileFromContainer(ctx, dockerClient, containerID, "/etc/group")
	if err != nil {
		return 0, err
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.SplitN(line, ":", 4)
		if len(fields) < 3 || fields[0] != groupName {
			continue
		}
		gid, err = strconv.Atoi(fields[2])
		if err != nil {
			return 0, fmt.Errorf("invalid gid for group %q in /etc/group: %w", groupName, err)
		}
		return gid, nil
	}
	return 0, fmt.Errorf("group %q not found in /etc/group", groupName)
}
