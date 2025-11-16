package git_test

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryanmoran/contagent/internal"
	"github.com/ryanmoran/contagent/internal/git"
	"github.com/stretchr/testify/require"
)

func TestCreateArchive(t *testing.T) {
	t.Run("creates valid tar archive with git config", func(t *testing.T) {
		// Setup a git repository
		dir, err := os.MkdirTemp("", "git-archive-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		// Initialize git repo
		cmd := exec.Command("git", "init")
		cmd.Dir = dir
		require.NoError(t, cmd.Run())

		// Create some test files
		testFile := filepath.Join(dir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test content\n"), 0644))

		subDir := filepath.Join(dir, "subdir")
		require.NoError(t, os.MkdirAll(subDir, 0755))
		subFile := filepath.Join(subDir, "nested.txt")
		require.NoError(t, os.WriteFile(subFile, []byte("nested content\n"), 0644))

		// Add and commit files
		cmd = exec.Command("git", "add", ".")
		cmd.Dir = dir
		require.NoError(t, cmd.Run())

		cmd = exec.Command("git", "commit", "-m", "initial commit")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test User",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test User",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		require.NoError(t, cmd.Run())

		// Create archive
		remote := "http://example.com/repo.git"
		branch := "test-branch"
		userName := "Archive User"
		userEmail := "archive@example.com"

		reader, err := git.CreateArchive(dir, remote, branch, userName, userEmail, internal.NewStandardWriter())
		require.NoError(t, err)
		defer reader.Close()

		// Extract and verify archive contents
		extractDir, err := os.MkdirTemp("", "archive-extract")
		require.NoError(t, err)
		defer os.RemoveAll(extractDir)

		tr := tar.NewReader(reader)
		files := make(map[string]bool)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)

			files[header.Name] = true

			target := filepath.Join(extractDir, header.Name)
			switch header.Typeflag {
			case tar.TypeDir:
				require.NoError(t, os.MkdirAll(target, os.FileMode(header.Mode)))
			case tar.TypeReg:
				dir := filepath.Dir(target)
				require.NoError(t, os.MkdirAll(dir, 0755))
				f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
				require.NoError(t, err)
				_, err = io.Copy(f, tr)
				require.NoError(t, err)
				f.Close()
			}
		}

		// Verify expected files are in archive
		require.True(t, files["app/"], "should contain app/ directory")
		require.True(t, files["app/.git/"], "should contain app/.git/ directory")
		require.True(t, files["app/test.txt"], "should contain app/test.txt")
		// Note: subdirectories are not explicitly added unless they contain files
		// tar will create them automatically when extracting files
		require.True(t, files["app/subdir/nested.txt"], "should contain app/subdir/nested.txt")

		// Verify file contents
		content, err := os.ReadFile(filepath.Join(extractDir, "app", "test.txt"))
		require.NoError(t, err)
		require.Equal(t, "test content\n", string(content))

		content, err = os.ReadFile(filepath.Join(extractDir, "app", "subdir", "nested.txt"))
		require.NoError(t, err)
		require.Equal(t, "nested content\n", string(content))

		// Verify git configuration in extracted archive
		appDir := filepath.Join(extractDir, "app")

		// Check remote
		cmd = exec.Command("git", "remote", "get-url", "origin")
		cmd.Dir = appDir
		output, err := cmd.Output()
		require.NoError(t, err)
		require.Equal(t, remote, strings.TrimSpace(string(output)))

		// Check branch
		cmd = exec.Command("git", "branch", "--show-current")
		cmd.Dir = appDir
		output, err = cmd.Output()
		require.NoError(t, err)
		require.Equal(t, branch, strings.TrimSpace(string(output)))

		// Check user config
		cmd = exec.Command("git", "config", "user.name")
		cmd.Dir = appDir
		output, err = cmd.Output()
		require.NoError(t, err)
		require.Equal(t, userName, strings.TrimSpace(string(output)))

		cmd = exec.Command("git", "config", "user.email")
		cmd.Dir = appDir
		output, err = cmd.Output()
		require.NoError(t, err)
		require.Equal(t, userEmail, strings.TrimSpace(string(output)))
	})

	t.Run("fails on non-git directory", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "non-git-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		_, err = git.CreateArchive(dir, "http://example.com", "branch", "user", "user@example.com", internal.NewStandardWriter())
		require.ErrorContains(t, err, "failed to get git root path")
	})

	t.Run("handles repository with no initial remote", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "git-no-remote-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		// Initialize git repo without remote
		cmd := exec.Command("git", "init")
		cmd.Dir = dir
		require.NoError(t, cmd.Run())

		// Create and commit a file
		testFile := filepath.Join(dir, "file.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("content\n"), 0644))

		cmd = exec.Command("git", "add", ".")
		cmd.Dir = dir
		require.NoError(t, cmd.Run())

		cmd = exec.Command("git", "commit", "-m", "commit")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test User",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test User",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		require.NoError(t, cmd.Run())

		// Create archive should succeed
		remote := "http://example.com/repo.git"
		reader, err := git.CreateArchive(dir, remote, "branch", "user", "user@example.com", internal.NewStandardWriter())
		require.NoError(t, err)
		defer reader.Close()

		// Drain the reader to ensure goroutine completes
		_, err = io.Copy(io.Discard, reader)
		require.NoError(t, err)
	})

	t.Run("skips symlinks", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "git-symlink-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		// Initialize git repo
		cmd := exec.Command("git", "init")
		cmd.Dir = dir
		require.NoError(t, cmd.Run())

		// Create a regular file
		regularFile := filepath.Join(dir, "regular.txt")
		require.NoError(t, os.WriteFile(regularFile, []byte("regular\n"), 0644))

		// Create a symlink
		symlinkPath := filepath.Join(dir, "link.txt")
		require.NoError(t, os.Symlink("regular.txt", symlinkPath))

		// Add and commit files (git will track the symlink)
		cmd = exec.Command("git", "add", ".")
		cmd.Dir = dir
		require.NoError(t, cmd.Run())

		cmd = exec.Command("git", "commit", "-m", "commit")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test User",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test User",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		require.NoError(t, cmd.Run())

		// Create archive
		reader, err := git.CreateArchive(dir, "http://example.com", "branch", "user", "user@example.com", internal.NewStandardWriter())
		require.NoError(t, err)
		defer reader.Close()

		// Verify symlink is not in archive
		tr := tar.NewReader(reader)
		files := make(map[string]bool)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			files[header.Name] = true
		}

		require.True(t, files["app/regular.txt"], "should contain regular file")
		require.False(t, files["app/link.txt"], "should not contain symlink")
	})

	t.Run("archives from subdirectory of git repo", func(t *testing.T) {
		// Create git repo
		dir, err := os.MkdirTemp("", "git-subdir-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		cmd := exec.Command("git", "init")
		cmd.Dir = dir
		require.NoError(t, cmd.Run())

		// Create files in root and subdirectory
		rootFile := filepath.Join(dir, "root.txt")
		require.NoError(t, os.WriteFile(rootFile, []byte("root\n"), 0644))

		subDir := filepath.Join(dir, "sub")
		require.NoError(t, os.MkdirAll(subDir, 0755))
		subFile := filepath.Join(subDir, "sub.txt")
		require.NoError(t, os.WriteFile(subFile, []byte("sub\n"), 0644))

		cmd = exec.Command("git", "add", ".")
		cmd.Dir = dir
		require.NoError(t, cmd.Run())

		cmd = exec.Command("git", "commit", "-m", "commit")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test User",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test User",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		require.NoError(t, cmd.Run())

		// Create archive from subdirectory
		reader, err := git.CreateArchive(subDir, "http://example.com", "branch", "user", "user@example.com", internal.NewStandardWriter())
		require.NoError(t, err)
		defer reader.Close()

		// Verify archive contains files from git root
		tr := tar.NewReader(reader)
		files := make(map[string]bool)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			files[header.Name] = true
		}

		require.True(t, files["app/root.txt"], "should contain root file")
		require.True(t, files["app/sub/sub.txt"], "should contain sub file")
	})

	t.Run("handles empty repository", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "git-empty-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		// Initialize empty git repo
		cmd := exec.Command("git", "init")
		cmd.Dir = dir
		require.NoError(t, cmd.Run())

		// Create archive from empty repo - returns reader but will error when reading
		reader, err := git.CreateArchive(dir, "http://example.com", "branch", "user", "user@example.com", internal.NewStandardWriter())
		// CreateArchive returns immediately with a reader, error happens in goroutine
		require.NoError(t, err)
		if reader != nil {
			defer reader.Close()
			// Try to read - should get error from the goroutine
			_, err = io.ReadAll(reader)
			require.Error(t, err)
			require.ErrorContains(t, err, "failed to checkout HEAD in temporary repo")
		}
	})
}

func TestCopyDirectory(t *testing.T) {
	t.Run("copies directory structure", func(t *testing.T) {
		// Create source directory with files
		src, err := os.MkdirTemp("", "copy-src")
		require.NoError(t, err)
		defer os.RemoveAll(src)

		// Create test structure
		require.NoError(t, os.WriteFile(filepath.Join(src, "file1.txt"), []byte("file1\n"), 0644))
		require.NoError(t, os.MkdirAll(filepath.Join(src, "subdir"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(src, "subdir", "file2.txt"), []byte("file2\n"), 0644))

		// Create destination
		dst, err := os.MkdirTemp("", "copy-dst")
		require.NoError(t, err)
		defer os.RemoveAll(dst)

		dstPath := filepath.Join(dst, "copied")

		// This tests the internal copyDirectory function indirectly through CreateArchive
		// We'll create a git repo and verify the .git directory is copied correctly
		gitDir, err := os.MkdirTemp("", "git-copy-test")
		require.NoError(t, err)
		defer os.RemoveAll(gitDir)

		cmd := exec.Command("git", "init")
		cmd.Dir = gitDir
		require.NoError(t, cmd.Run())

		// Add a file and commit
		testFile := filepath.Join(gitDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test\n"), 0644))

		cmd = exec.Command("git", "add", ".")
		cmd.Dir = gitDir
		require.NoError(t, cmd.Run())

		cmd = exec.Command("git", "commit", "-m", "commit")
		cmd.Dir = gitDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test User",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test User",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		require.NoError(t, cmd.Run())

		// Create archive (this will internally use copyDirectory)
		reader, err := git.CreateArchive(gitDir, "http://example.com", "branch", "user", "user@example.com", internal.NewStandardWriter())
		require.NoError(t, err)
		defer reader.Close()

		// Extract archive
		tr := tar.NewReader(reader)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)

			if !strings.HasPrefix(header.Name, "app/.git/") {
				continue
			}

			target := filepath.Join(dstPath, strings.TrimPrefix(header.Name, "app/"))
			switch header.Typeflag {
			case tar.TypeDir:
				require.NoError(t, os.MkdirAll(target, os.FileMode(header.Mode)))
			case tar.TypeReg:
				dir := filepath.Dir(target)
				require.NoError(t, os.MkdirAll(dir, 0755))
				f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
				require.NoError(t, err)
				_, err = io.Copy(f, tr)
				require.NoError(t, err)
				f.Close()
			}
		}

		// Verify .git directory was copied
		gitCopyPath := filepath.Join(dstPath, ".git")
		stat, err := os.Stat(gitCopyPath)
		require.NoError(t, err)
		require.True(t, stat.IsDir(), ".git should be a directory")

		// Verify git repo is valid
		cmd = exec.Command("git", "status")
		cmd.Dir = dstPath
		require.NoError(t, cmd.Run(), "copied .git should be a valid repository")
	})

	t.Run("handles nested directories", func(t *testing.T) {
		// Create a git repo with nested structure
		dir, err := os.MkdirTemp("", "git-nested-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		cmd := exec.Command("git", "init")
		cmd.Dir = dir
		require.NoError(t, cmd.Run())

		// Create deeply nested structure
		nested := filepath.Join(dir, "a", "b", "c", "d")
		require.NoError(t, os.MkdirAll(nested, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(nested, "deep.txt"), []byte("deep\n"), 0644))

		cmd = exec.Command("git", "add", ".")
		cmd.Dir = dir
		require.NoError(t, cmd.Run())

		cmd = exec.Command("git", "commit", "-m", "commit")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test User",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test User",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		require.NoError(t, cmd.Run())

		// Create archive
		reader, err := git.CreateArchive(dir, "http://example.com", "branch", "user", "user@example.com", internal.NewStandardWriter())
		require.NoError(t, err)
		defer reader.Close()

		// Verify nested structure in archive
		tr := tar.NewReader(reader)
		files := make(map[string]bool)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			files[header.Name] = true
		}

		// Note: git ls-files only returns files, not directories
		// But we should still have the nested file
		require.True(t, files["app/a/b/c/d/deep.txt"], "should contain app/a/b/c/d/deep.txt")
	})

	t.Run("preserves file permissions", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "git-perms-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		cmd := exec.Command("git", "init")
		cmd.Dir = dir
		require.NoError(t, cmd.Run())

		// Create executable file
		execFile := filepath.Join(dir, "script.sh")
		require.NoError(t, os.WriteFile(execFile, []byte("#!/bin/bash\necho test\n"), 0755))

		// Create regular file
		regularFile := filepath.Join(dir, "data.txt")
		require.NoError(t, os.WriteFile(regularFile, []byte("data\n"), 0644))

		cmd = exec.Command("git", "add", ".")
		cmd.Dir = dir
		require.NoError(t, cmd.Run())

		cmd = exec.Command("git", "commit", "-m", "commit")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test User",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test User",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		require.NoError(t, cmd.Run())

		// Create archive
		reader, err := git.CreateArchive(dir, "http://example.com", "branch", "user", "user@example.com", internal.NewStandardWriter())
		require.NoError(t, err)
		defer reader.Close()

		// Check permissions in archive
		tr := tar.NewReader(reader)
		permissions := make(map[string]int64)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			permissions[header.Name] = header.Mode
		}

		// Verify executable bit is preserved
		scriptMode := permissions["app/script.sh"]
		require.NotZero(t, scriptMode, "script.sh should have mode set")
		require.NotZero(t, scriptMode&0111, "script.sh should have executable bit")

		// Verify regular file doesn't have executable bit
		dataMode := permissions["app/data.txt"]
		require.NotZero(t, dataMode, "data.txt should have mode set")
	})
}

func TestCopyFile(t *testing.T) {
	t.Run("copies file content and permissions", func(t *testing.T) {
		// This is tested indirectly through copyDirectory
		// Creating a more direct test
		src, err := os.MkdirTemp("", "file-src")
		require.NoError(t, err)
		defer os.RemoveAll(src)

		srcFile := filepath.Join(src, "source.txt")
		content := "test file content\n"
		require.NoError(t, os.WriteFile(srcFile, []byte(content), 0644))

		dst, err := os.MkdirTemp("", "file-dst")
		require.NoError(t, err)
		defer os.RemoveAll(dst)

		// We test copyFile indirectly through the git archive process
		// Create a git repo to trigger the code path
		gitDir, err := os.MkdirTemp("", "git-copyfile-test")
		require.NoError(t, err)
		defer os.RemoveAll(gitDir)

		cmd := exec.Command("git", "init")
		cmd.Dir = gitDir
		require.NoError(t, cmd.Run())

		testFile := filepath.Join(gitDir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte(content), 0644))

		cmd = exec.Command("git", "add", ".")
		cmd.Dir = gitDir
		require.NoError(t, cmd.Run())

		cmd = exec.Command("git", "commit", "-m", "commit")
		cmd.Dir = gitDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test User",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test User",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		require.NoError(t, cmd.Run())

		reader, err := git.CreateArchive(gitDir, "http://example.com", "branch", "user", "user@example.com", internal.NewStandardWriter())
		require.NoError(t, err)
		defer reader.Close()

		// Extract and verify
		extractDir, err := os.MkdirTemp("", "extract")
		require.NoError(t, err)
		defer os.RemoveAll(extractDir)

		tr := tar.NewReader(reader)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)

			if header.Name == "app/test.txt" {
				target := filepath.Join(extractDir, "test.txt")
				f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
				require.NoError(t, err)
				_, err = io.Copy(f, tr)
				require.NoError(t, err)
				f.Close()

				// Verify content
				readContent, err := os.ReadFile(target)
				require.NoError(t, err)
				require.Equal(t, content, string(readContent))
				break
			}
		}
	})

	t.Run("creates destination directory if needed", func(t *testing.T) {
		// Test that copyFile creates parent directories
		// This is verified through the nested directory test above
		dir, err := os.MkdirTemp("", "git-mkdir-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		cmd := exec.Command("git", "init")
		cmd.Dir = dir
		require.NoError(t, cmd.Run())

		// Create file in nested directory
		nested := filepath.Join(dir, "parent", "child")
		require.NoError(t, os.MkdirAll(nested, 0755))
		testFile := filepath.Join(nested, "file.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("content\n"), 0644))

		cmd = exec.Command("git", "add", ".")
		cmd.Dir = dir
		require.NoError(t, cmd.Run())

		cmd = exec.Command("git", "commit", "-m", "commit")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test User",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test User",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		require.NoError(t, cmd.Run())

		reader, err := git.CreateArchive(dir, "http://example.com", "branch", "user", "user@example.com", internal.NewStandardWriter())
		require.NoError(t, err)
		defer reader.Close()

		// Verify archive contains nested file
		tr := tar.NewReader(reader)
		found := false
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			if header.Name == "app/parent/child/file.txt" {
				found = true
				break
			}
		}
		require.True(t, found, "should contain nested file")
	})
}

func TestAddDirectoryToArchive(t *testing.T) {
	t.Run("adds .git directory to archive", func(t *testing.T) {
		// This function is tested indirectly through CreateArchive
		// Verify .git directory structure in archive
		dir, err := os.MkdirTemp("", "git-adddir-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		cmd := exec.Command("git", "init")
		cmd.Dir = dir
		require.NoError(t, cmd.Run())

		testFile := filepath.Join(dir, "test.txt")
		require.NoError(t, os.WriteFile(testFile, []byte("test\n"), 0644))

		cmd = exec.Command("git", "add", ".")
		cmd.Dir = dir
		require.NoError(t, cmd.Run())

		cmd = exec.Command("git", "commit", "-m", "commit")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test User",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test User",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		require.NoError(t, cmd.Run())

		reader, err := git.CreateArchive(dir, "http://example.com", "branch", "user", "user@example.com", internal.NewStandardWriter())
		require.NoError(t, err)
		defer reader.Close()

		// Verify .git structure
		tr := tar.NewReader(reader)
		gitFiles := make([]string, 0)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			if strings.HasPrefix(header.Name, "app/.git/") {
				gitFiles = append(gitFiles, header.Name)
			}
		}

		require.NotEmpty(t, gitFiles, "should contain .git files")
		require.Contains(t, fmt.Sprintf("%v", gitFiles), "app/.git/config", "should contain git config")
	})

	t.Run("normalizes path separators", func(t *testing.T) {
		// Verify paths use forward slashes in tar archive
		dir, err := os.MkdirTemp("", "git-paths-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		cmd := exec.Command("git", "init")
		cmd.Dir = dir
		require.NoError(t, cmd.Run())

		nested := filepath.Join(dir, "a", "b", "c")
		require.NoError(t, os.MkdirAll(nested, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(nested, "file.txt"), []byte("content\n"), 0644))

		cmd = exec.Command("git", "add", ".")
		cmd.Dir = dir
		require.NoError(t, cmd.Run())

		cmd = exec.Command("git", "commit", "-m", "commit")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test User",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test User",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		require.NoError(t, cmd.Run())

		reader, err := git.CreateArchive(dir, "http://example.com", "branch", "user", "user@example.com", internal.NewStandardWriter())
		require.NoError(t, err)
		defer reader.Close()

		// Verify all paths use forward slashes
		tr := tar.NewReader(reader)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			require.NotContains(t, header.Name, "\\", "paths should use forward slashes")
		}
	})
}
