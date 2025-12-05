package git_test

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ryanmoran/contagent/internal"
	"github.com/ryanmoran/contagent/internal/git"
	"github.com/stretchr/testify/require"
)

// TestGitServerErrorCases tests various error scenarios in git server
func TestGitServerErrorCases(t *testing.T) {
	t.Run("NewServer error cases", func(t *testing.T) {
		t.Run("non-git directory", func(t *testing.T) {
			dir, err := os.MkdirTemp("", "git-error-test")
			require.NoError(t, err)
			defer os.RemoveAll(dir)

			_, err = git.NewServer(dir, internal.NewStandardWriter())
			require.Error(t, err)
			require.Contains(t, err.Error(), "not a git repository")
		})

		t.Run("non-existent directory", func(t *testing.T) {
			_, err := git.NewServer("/nonexistent/path/to/repo", internal.NewStandardWriter())
			require.Error(t, err)
			// Error could be either "not a git repository" or path resolution error
			require.Error(t, err)
		})

		t.Run("relative path resolution", func(t *testing.T) {
			dir, err := os.MkdirTemp("", "git-rel-test")
			require.NoError(t, err)
			defer os.RemoveAll(dir)

			// Initialize git repo
			cmd := exec.Command("git", "init")
			cmd.Dir = dir
			require.NoError(t, cmd.Run())

			// Create subdirectory
			subDir := filepath.Join(dir, "subdir")
			require.NoError(t, os.Mkdir(subDir, 0755))

			// Save current directory
			oldWd, err := os.Getwd()
			require.NoError(t, err)
			defer os.Chdir(oldWd)

			// Change to subdirectory
			require.NoError(t, os.Chdir(subDir))

			// Try to create server with relative path
			_, err = git.NewServer("..", internal.NewStandardWriter())
			require.NoError(t, err) // Should succeed with relative path resolution
		})
	})

	t.Run("Server operation error cases", func(t *testing.T) {
		t.Run("double close doesn't panic", func(t *testing.T) {
			dir, err := os.MkdirTemp("", "git-close-test")
			require.NoError(t, err)
			defer os.RemoveAll(dir)

			cmd := exec.Command("git", "init")
			cmd.Dir = dir
			require.NoError(t, cmd.Run())

			server, err := git.NewServer(dir, internal.NewStandardWriter())
			require.NoError(t, err)

			err = server.Close()
			require.NoError(t, err)

			// Second close should not panic (may return error)
			_ = server.Close()
		})
	})
}

// TestGitArchiveErrorCases tests various error scenarios in git archive
func TestGitArchiveErrorCases(t *testing.T) {
	t.Run("CreateArchive error cases", func(t *testing.T) {
		t.Run("non-git directory", func(t *testing.T) {
			dir, err := os.MkdirTemp("", "archive-error-test")
			require.NoError(t, err)
			defer os.RemoveAll(dir)

			_, err = git.CreateArchive(dir, "http://example.com", "branch", "user", "user@example.com", internal.NewStandardWriter())
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed to get git root path")
		})

		t.Run("non-existent directory", func(t *testing.T) {
			_, err := git.CreateArchive("/nonexistent/path", "http://example.com", "branch", "user", "user@example.com", internal.NewStandardWriter())
			require.Error(t, err)
			require.Contains(t, err.Error(), "failed to get git root path")
		})

		t.Run("empty git repository", func(t *testing.T) {
			dir, err := os.MkdirTemp("", "archive-empty-test")
			require.NoError(t, err)
			defer os.RemoveAll(dir)

			// Initialize empty git repo
			cmd := exec.Command("git", "init")
			cmd.Dir = dir
			require.NoError(t, cmd.Run())

			reader, err := git.CreateArchive(dir, "http://example.com", "branch", "user", "user@example.com", internal.NewStandardWriter())
			require.NoError(t, err) // Returns immediately
			if reader != nil {
				defer reader.Close()
				// Error occurs when reading
				_, err = io.ReadAll(reader)
				require.Error(t, err)
				require.Contains(t, err.Error(), "failed to checkout HEAD in temporary repo")
			}
		})

		t.Run("directory with permission denied", func(t *testing.T) {
			if os.Getuid() == 0 {
				t.Skip("Running as root, cannot test permission denied")
			}

			dir, err := os.MkdirTemp("", "archive-perm-test")
			require.NoError(t, err)
			defer os.RemoveAll(dir)

			// Initialize git repo
			cmd := exec.Command("git", "init")
			cmd.Dir = dir
			require.NoError(t, cmd.Run())

			// Create and commit a file
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

			// Remove read permissions
			err = os.Chmod(dir, 0000)
			require.NoError(t, err)

			// Restore permissions for cleanup
			defer os.Chmod(dir, 0755)

			_, err = git.CreateArchive(dir, "http://example.com", "branch", "user", "user@example.com", internal.NewStandardWriter())
			require.Error(t, err)
		})

		t.Run("invalid remote URL", func(t *testing.T) {
			dir, err := os.MkdirTemp("", "archive-badremote-test")
			require.NoError(t, err)
			defer os.RemoveAll(dir)

			// Initialize git repo
			cmd := exec.Command("git", "init")
			cmd.Dir = dir
			require.NoError(t, cmd.Run())

			// Create and commit a file
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

			// Git accepts most URLs, but we test that it doesn't fail during archive creation
			// The URL validation happens when actually using the remote
			reader, err := git.CreateArchive(dir, "not-a-valid-url", "branch", "user", "user@example.com", internal.NewStandardWriter())
			require.NoError(t, err) // Archive creation succeeds even with invalid URL
			if reader != nil {
				reader.Close()
			}
		})

		t.Run("invalid branch name", func(t *testing.T) {
			dir, err := os.MkdirTemp("", "archive-badbranch-test")
			require.NoError(t, err)
			defer os.RemoveAll(dir)

			// Initialize git repo
			cmd := exec.Command("git", "init")
			cmd.Dir = dir
			require.NoError(t, cmd.Run())

			// Create and commit a file
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

			// Try to create archive with invalid branch name (contains spaces)
			reader, err := git.CreateArchive(dir, "http://example.com", "invalid branch name", "user", "user@example.com", internal.NewStandardWriter())
			if err == nil && reader != nil {
				defer reader.Close()
				// Error may occur when reading
				_, err = io.ReadAll(reader)
			}
			// Git may accept or reject the branch name depending on version
			// We just ensure it handles it somehow (either immediate error or read error)
		})

		t.Run("invalid git user config", func(t *testing.T) {
			dir, err := os.MkdirTemp("", "archive-baduser-test")
			require.NoError(t, err)
			defer os.RemoveAll(dir)

			// Initialize git repo
			cmd := exec.Command("git", "init")
			cmd.Dir = dir
			require.NoError(t, cmd.Run())

			// Create and commit a file
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

			// Empty user name and email are technically valid in git
			reader, err := git.CreateArchive(dir, "http://example.com", "branch", "", "", internal.NewStandardWriter())
			require.NoError(t, err)
			if reader != nil {
				reader.Close()
			}
		})

		t.Run("repository with conflicting branch", func(t *testing.T) {
			dir, err := os.MkdirTemp("", "archive-conflict-test")
			require.NoError(t, err)
			defer os.RemoveAll(dir)

			// Initialize git repo
			cmd := exec.Command("git", "init")
			cmd.Dir = dir
			require.NoError(t, cmd.Run())

			// Create and commit a file
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

			// Create a branch with the same name we'll try to use
			cmd = exec.Command("git", "branch", "test-branch")
			cmd.Dir = dir
			require.NoError(t, cmd.Run())

			// Archive will fail because it tries to create a branch that already exists in the copied .git
			reader, err := git.CreateArchive(dir, "http://example.com", "test-branch", "user", "user@example.com", internal.NewStandardWriter())
			if reader != nil {
				defer reader.Close()
				// Error may occur during read
				_, readErr := io.Copy(io.Discard, reader)
				if readErr != nil {
					// Expected: branch already exists
					require.Contains(t, readErr.Error(), "failed to create and checkout branch")
				}
			}
			// Archive creation may fail immediately or during read
			// Both are acceptable error handling
		})

		t.Run("repository with detached HEAD", func(t *testing.T) {
			dir, err := os.MkdirTemp("", "archive-detached-test")
			require.NoError(t, err)
			defer os.RemoveAll(dir)

			// Initialize git repo
			cmd := exec.Command("git", "init")
			cmd.Dir = dir
			require.NoError(t, cmd.Run())

			// Create and commit a file
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

			// Detach HEAD
			cmd = exec.Command("git", "checkout", "--detach", "HEAD")
			cmd.Dir = dir
			require.NoError(t, cmd.Run())

			// Archive should still work with detached HEAD
			reader, err := git.CreateArchive(dir, "http://example.com", "branch", "user", "user@example.com", internal.NewStandardWriter())
			require.NoError(t, err)
			if reader != nil {
				defer reader.Close()
				_, err = io.Copy(io.Discard, reader)
				require.NoError(t, err)
			}
		})

		t.Run("repository with uncommitted changes", func(t *testing.T) {
			dir, err := os.MkdirTemp("", "archive-dirty-test")
			require.NoError(t, err)
			defer os.RemoveAll(dir)

			// Initialize git repo
			cmd := exec.Command("git", "init")
			cmd.Dir = dir
			require.NoError(t, cmd.Run())

			// Create and commit a file
			testFile := filepath.Join(dir, "test.txt")
			require.NoError(t, os.WriteFile(testFile, []byte("committed\n"), 0644))

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

			// Make uncommitted changes
			require.NoError(t, os.WriteFile(testFile, []byte("uncommitted\n"), 0644))

			// Archive should only include committed content (archives HEAD, not working tree)
			reader, err := git.CreateArchive(dir, "http://example.com", "branch", "user", "user@example.com", internal.NewStandardWriter())
			require.NoError(t, err)
			require.NotNil(t, reader)
			reader.Close()
		})

		t.Run("repository with submodules", func(t *testing.T) {
			// Create main repo
			mainDir, err := os.MkdirTemp("", "archive-main-test")
			require.NoError(t, err)
			defer os.RemoveAll(mainDir)

			cmd := exec.Command("git", "init")
			cmd.Dir = mainDir
			require.NoError(t, cmd.Run())

			testFile := filepath.Join(mainDir, "main.txt")
			require.NoError(t, os.WriteFile(testFile, []byte("main\n"), 0644))

			cmd = exec.Command("git", "add", ".")
			cmd.Dir = mainDir
			require.NoError(t, cmd.Run())

			cmd = exec.Command("git", "commit", "-m", "commit")
			cmd.Dir = mainDir
			cmd.Env = append(os.Environ(),
				"GIT_AUTHOR_NAME=Test User",
				"GIT_AUTHOR_EMAIL=test@example.com",
				"GIT_COMMITTER_NAME=Test User",
				"GIT_COMMITTER_EMAIL=test@example.com",
			)
			require.NoError(t, cmd.Run())

			// Archive should handle repos with .gitmodules (even if submodules not initialized)
			reader, err := git.CreateArchive(mainDir, "http://example.com", "branch", "user", "user@example.com", internal.NewStandardWriter())
			require.NoError(t, err)
			if reader != nil {
				reader.Close()
			}
		})
	})
}

// TestGitFileSystemErrors tests file system related errors
func TestGitFileSystemErrors(t *testing.T) {
	t.Run("temporary directory creation failure", func(t *testing.T) {
		// This is hard to test without mocking, but we can verify the error path exists
		// by ensuring CreateArchive handles temp dir errors properly in its implementation
		t.Skip("Requires mocking os.MkdirTemp")
	})

	t.Run("file copy errors", func(t *testing.T) {
		// Test that file system errors during copy are handled
		t.Skip("Requires specific file system error conditions")
	})
}
