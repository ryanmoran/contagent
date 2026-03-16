package git

import (
	"archive/tar"
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ryanmoran/contagent/internal"
)

// ArchiveOptions holds the configuration for creating a git archive.
type ArchiveOptions struct {
	Path         string
	Remote       string
	Branch       string
	GitUserName  string
	GitUserEmail string
	UID          int
	GID          int
	DestDir      string
}

// CreateArchive creates a tar archive of the Git repository at the specified path, configured
// with the given remote URL and branch name. It checks out HEAD into a temporary directory,
// configures the remote, creates a new branch, and archives the .git directory and all tracked
// files. The git user name and email are configured in the temporary repository.
//
// opts.UID and opts.GID are applied to all tar headers so that extracted files are owned by
// the correct container user.
//
// When opts.DestDir is non-empty, all archive paths are prefixed with DestDir and a root
// directory entry is written first. This allows copying to the parent directory so Docker
// creates DestDir as a new entry with the correct uid/gid ownership, rather than copying
// into an already-existing root-owned directory.
//
// Returns an io.ReadCloser that streams the tar archive. The caller must close it to clean up
// resources. Returns an error if the Git root cannot be determined, the temporary directory
// cannot be created, .git copying fails, git operations fail, or archive creation fails.
func CreateArchive(opts ArchiveOptions, w internal.Writer) (io.ReadCloser, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = opts.Path
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git root path from %q: %w\nEnsure you're in a git repository", opts.Path, err)
	}

	root := strings.TrimSpace(string(output))

	tempDir, err := os.MkdirTemp("", "contagent-checkout-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w\nCheck disk space and /tmp permissions", err)
	}

	pr, pw := io.Pipe()

	go func() {
		tw := tar.NewWriter(pw)

		err := buildArchive(tw, opts, root, tempDir)
		if err != nil {
			pw.CloseWithError(fmt.Errorf("failed to create git archive: %w", err))
		} else {
			err = tw.Close()
			if err != nil {
				pw.CloseWithError(fmt.Errorf("failed to close tar writer: %w", err))
			}

			pw.Close()
		}
	}()

	return &archiveCloser{pr: pr}, nil
}

// buildArchive performs the actual archive creation: copying .git, running git commands,
// and writing all tracked files into the tar writer.
func buildArchive(tw *tar.Writer, opts ArchiveOptions, gitRoot, tempRoot string) error {
	defer os.RemoveAll(tempRoot) // Clean up temp directory

	src := filepath.Join(gitRoot, ".git")
	dst := filepath.Join(tempRoot, ".git")

	if err := copyDirectory(src, dst); err != nil {
		return fmt.Errorf("failed to copy .git directory from %q to %q: %w\nCheck disk space and permissions", src, dst, err)
	}

	cmd := exec.Command("git", "checkout", "HEAD", ".")
	cmd.Dir = tempRoot
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to checkout HEAD in temporary repo: %w\nYou may have uncommitted changes or detached HEAD", err)
	}

	cmd = exec.Command("git", "remote", "remove", "origin")
	cmd.Dir = tempRoot
	err = cmd.Run()
	if err != nil {
		if exitError, ok := errors.AsType[*exec.ExitError](err); !ok || exitError.ExitCode() != 2 {
			return fmt.Errorf("failed to remove remote \"origin\": %w", err)
		}
	}

	cmd = exec.Command("git", "remote", "add", "origin", opts.Remote) //nolint:gosec // args are controlled by internal config, not user input
	cmd.Dir = tempRoot
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to add git remote %q: %w\nCheck that the URL is valid", opts.Remote, err)
	}

	cmd = exec.Command("git", "config", "user.email", opts.GitUserEmail) //nolint:gosec // args are controlled by internal config, not user input
	cmd.Dir = tempRoot
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to configure git user.email to %q: %w", opts.GitUserEmail, err)
	}

	cmd = exec.Command("git", "config", "user.name", opts.GitUserName) //nolint:gosec // args are controlled by internal config, not user input
	cmd.Dir = tempRoot
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to configure git user.name to %q: %w", opts.GitUserName, err)
	}

	cmd = exec.Command("git", "config", "push.autoSetupRemote", "true")
	cmd.Dir = tempRoot
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to configure git push.autoSetupRemote: %w", err)
	}

	cmd = exec.Command("git", "checkout", "-b", opts.Branch) //nolint:gosec // args are controlled by internal config, not user input
	cmd.Dir = tempRoot
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to create and checkout branch %q: %w\nBranch may already exist", opts.Branch, err)
	}

	prefix := func(name string) string {
		if opts.DestDir == "" {
			return name
		}
		return opts.DestDir + "/" + name
	}

	if opts.DestDir != "" {
		rootHeader := &tar.Header{
			Name:     opts.DestDir + "/",
			Mode:     0755,
			Typeflag: tar.TypeDir,
			Uid:      opts.UID,
			Gid:      opts.GID,
		}
		if err := tw.WriteHeader(rootHeader); err != nil {
			return fmt.Errorf("failed to write root directory header: %w", err)
		}
	}

	if err := addDirectoryToArchive(tw, dst, prefix(".git"), opts.UID, opts.GID); err != nil {
		return fmt.Errorf("failed to add .git directory: %w", err)
	}

	cmd = exec.Command("git", "ls-files")
	cmd.Dir = tempRoot
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list git tracked files: %w\nRepository may be corrupted", err)
	}

	// Collect file paths and all unique parent directories
	var filePaths []string
	dirsSeen := make(map[string]bool)
	var sortedDirs []string

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		relPath := scanner.Text()
		if relPath == "" {
			continue
		}
		filePaths = append(filePaths, relPath)

		// Collect all parent directories of this file
		dir := filepath.Dir(relPath)
		for dir != "." && dir != "/" {
			dirPath := strings.ReplaceAll(dir, "\\", "/")
			if !dirsSeen[dirPath] {
				dirsSeen[dirPath] = true
				sortedDirs = append(sortedDirs, dirPath)
			}
			dir = filepath.Dir(dir)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading git file list: %w", err)
	}

	// Sort so parent directories come before their children
	sort.Strings(sortedDirs)

	// Write directory headers first so they have proper permissions when extracted
	for _, dirPath := range sortedDirs {
		fullPath := filepath.Join(tempRoot, dirPath)
		info, err := os.Lstat(fullPath) //nolint:gosec // path is constructed from a controlled temp root
		if err != nil {
			continue
		}

		header := &tar.Header{
			Name:     prefix(dirPath) + "/",
			Mode:     int64(info.Mode()),
			ModTime:  info.ModTime(),
			Typeflag: tar.TypeDir,
			Uid:      opts.UID,
			Gid:      opts.GID,
		}
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write directory header for %s: %w", dirPath, err)
		}
	}

	// Write tracked files
	for _, relPath := range filePaths {
		fullPath := filepath.Join(tempRoot, relPath)
		info, err := os.Lstat(fullPath) //nolint:gosec // path is constructed from a controlled temp root
		if err != nil {
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}

		if info.IsDir() {
			continue
		}

		file, err := os.Open(fullPath) //nolint:gosec // path is constructed from a controlled temp root
		if err != nil {
			return fmt.Errorf("failed to open tracked file %q: %w\nFile may have been deleted", relPath, err)
		}

		header := &tar.Header{
			Name:    prefix(relPath),
			Mode:    int64(info.Mode()),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Uid:     opts.UID,
			Gid:     opts.GID,
		}

		if err := tw.WriteHeader(header); err != nil {
			file.Close()
			return fmt.Errorf("failed to write header for %s: %w", relPath, err)
		}

		if _, err := io.Copy(tw, file); err != nil {
			file.Close()
			return fmt.Errorf("failed to write file %s: %w", relPath, err)
		}
		file.Close()
	}

	return nil
}

// archiveCloser wraps the pipe reader to ensure proper cleanup
type archiveCloser struct {
	pr *io.PipeReader
}

func (a *archiveCloser) Read(p []byte) (int, error) {
	return a.pr.Read(p)
}

func (a *archiveCloser) Close() error {
	return a.pr.Close()
}

// walkDir walks a directory tree, skipping symlinks, and calls visitor for each entry.
// visitor receives the relative path, file info, and absolute path of each entry.
func walkDir(root string, visitor func(relPath string, info os.FileInfo, absPath string) error) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		return visitor(relPath, info, path)
	})
}

func addDirectoryToArchive(tw *tar.Writer, srcDir, tarPath string, uid, gid int) error {
	return walkDir(srcDir, func(relPath string, info os.FileInfo, absPath string) error {
		fullTarPath := filepath.Join(tarPath, relPath)
		fullTarPath = strings.ReplaceAll(fullTarPath, "\\", "/")

		if info.IsDir() {
			header := &tar.Header{
				Name:     fullTarPath + "/",
				Mode:     int64(info.Mode()),
				ModTime:  info.ModTime(),
				Typeflag: tar.TypeDir,
				Uid:      uid,
				Gid:      gid,
			}
			return tw.WriteHeader(header)
		}

		file, err := os.Open(absPath)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %w", absPath, err)
		}
		defer file.Close()

		header := &tar.Header{
			Name:    fullTarPath,
			Mode:    int64(info.Mode()),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Uid:     uid,
			Gid:     gid,
		}

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write header for %s: %w", absPath, err)
		}

		if _, err := io.Copy(tw, file); err != nil {
			return fmt.Errorf("failed to write file %s: %w", absPath, err)
		}

		return nil
	})
}

func copyDirectory(src, dst string) error {
	return walkDir(src, func(relPath string, info os.FileInfo, absPath string) error {
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(absPath, dstPath, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	return nil
}
