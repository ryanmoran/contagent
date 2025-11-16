package git

import (
	"archive/tar"
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ryanmoran/contagent/internal"
)

// CreateArchive creates a tar archive of the Git repository at the specified path, configured
// with the given remote URL and branch name. It checks out HEAD into a temporary directory,
// configures the remote, creates a new branch, and archives the .git directory and all tracked
// files. The git user name and email are configured in the temporary repository.
//
// Returns an io.ReadCloser that streams the tar archive. The caller must close it to clean up
// resources. Returns an error if the Git root cannot be determined, the temporary directory cannot
// be created, .git copying fails, git operations fail, or archive creation fails.
func CreateArchive(path, remote, branch, gitUserName, gitUserEmail string, w internal.Writer) (io.ReadCloser, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = path
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git root path from %q: %w\nEnsure you're in a git repository", path, err)
	}

	root := strings.TrimSpace(string(output))

	tempDir, err := os.MkdirTemp("", "contagent-checkout-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w\nCheck disk space and /tmp permissions", err)
	}

	pr, pw := io.Pipe()

	archive := func(tw *tar.Writer, gitRoot, tempRoot string) error {
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
			exitError, ok := err.(*exec.ExitError)
			if !ok || exitError.ExitCode() != 2 {
				return fmt.Errorf("failed to remove remote \"origin\": %w", err)
			}
		}

		cmd = exec.Command("git", "remote", "add", "origin", remote)
		cmd.Dir = tempRoot
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to add git remote %q: %w\nCheck that the URL is valid", remote, err)
		}

		cmd = exec.Command("git", "config", "user.email", gitUserEmail)
		cmd.Dir = tempRoot
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to configure git user.email to %q: %w", gitUserEmail, err)
		}

		cmd = exec.Command("git", "config", "user.name", gitUserName)
		cmd.Dir = tempRoot
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to configure git user.name to %q: %w", gitUserName, err)
		}

		cmd = exec.Command("git", "checkout", "-b", branch)
		cmd.Dir = tempRoot
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("failed to create and checkout branch %q: %w\nBranch may already exist", branch, err)
		}

		header := &tar.Header{
			Name:     "app/",
			Mode:     0755,
			Typeflag: tar.TypeDir,
		}
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to create app directory: %w", err)
		}

		if err := addDirectoryToArchive(tw, dst, "app/.git"); err != nil {
			return fmt.Errorf("failed to add .git directory: %w", err)
		}

		cmd = exec.Command("git", "ls-files")
		cmd.Dir = tempRoot
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to list git tracked files: %w\nRepository may be corrupted", err)
		}

		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		for scanner.Scan() {
			relPath := scanner.Text()
			if relPath == "" {
				continue
			}

			fullPath := filepath.Join(tempRoot, relPath)
			info, err := os.Lstat(fullPath)
			if err != nil {
				continue
			}

			if info.Mode()&os.ModeSymlink != 0 {
				continue
			}

			if info.IsDir() {
				header := &tar.Header{
					Name:     filepath.Join("app", relPath) + "/",
					Mode:     int64(info.Mode()),
					ModTime:  info.ModTime(),
					Typeflag: tar.TypeDir,
				}
				if err := tw.WriteHeader(header); err != nil {
					return fmt.Errorf("failed to write directory header for %s: %w", relPath, err)
				}
			} else {
				file, err := os.Open(fullPath)
				if err != nil {
					return fmt.Errorf("failed to open tracked file %q: %w\nFile may have been deleted", relPath, err)
				}
				defer file.Close()

				header := &tar.Header{
					Name:    filepath.Join("app", relPath),
					Mode:    int64(info.Mode()),
					Size:    info.Size(),
					ModTime: info.ModTime(),
				}

				if err := tw.WriteHeader(header); err != nil {
					return fmt.Errorf("failed to write header for %s: %w", relPath, err)
				}

				if _, err := io.Copy(tw, file); err != nil {
					return fmt.Errorf("failed to write file %s: %w", relPath, err)
				}
			}
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("error reading git file list: %w", err)
		}

		return nil
	}

	go func() {
		tw := tar.NewWriter(pw)
		defer tw.Close()

		err := archive(tw, root, tempDir)
		if err != nil {
			pw.CloseWithError(fmt.Errorf("failed to create git archive: %w", err))
		} else {
			pw.Close()
		}
	}()

	return &archiveCloser{pr: pr}, nil
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

func addDirectoryToArchive(tw *tar.Writer, srcDir, tarPath string) error {
	return filepath.Walk(srcDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, filePath)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		fullTarPath := filepath.Join(tarPath, relPath)
		fullTarPath = strings.ReplaceAll(fullTarPath, "\\", "/")

		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		if info.IsDir() {
			header := &tar.Header{
				Name:     fullTarPath + "/",
				Mode:     int64(info.Mode()),
				ModTime:  info.ModTime(),
				Typeflag: tar.TypeDir,
			}
			return tw.WriteHeader(header)
		} else {
			file, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("failed to open file %s: %w", filePath, err)
			}
			defer file.Close()

			header := &tar.Header{
				Name:    fullTarPath,
				Mode:    int64(info.Mode()),
				Size:    info.Size(),
				ModTime: info.ModTime(),
			}

			if err := tw.WriteHeader(header); err != nil {
				return fmt.Errorf("failed to write header for %s: %w", filePath, err)
			}

			if _, err := io.Copy(tw, file); err != nil {
				return fmt.Errorf("failed to write file %s: %w", filePath, err)
			}
		}

		return nil
	})
}

func copyDirectory(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		dstPath := filepath.Join(dst, relPath)

		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		} else {
			return copyFile(path, dstPath, info.Mode())
		}
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
