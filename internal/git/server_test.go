package git_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryanmoran/contagent/internal"
	"github.com/ryanmoran/contagent/internal/git"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	setup := func(t *testing.T) (git.Server, string) {
		dir, err := os.MkdirTemp("", "git-server-test")
		require.NoError(t, err)
		t.Cleanup(func() {
			os.RemoveAll(dir)
		})

		cmd := exec.Command("git", "init")
		cmd.Dir = dir
		err = cmd.Run()
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(dir, "test.txt"), []byte("initial content\n"), 0644)
		require.NoError(t, err)

		cmd = exec.Command("git", "add", "test.txt")
		cmd.Dir = dir
		err = cmd.Run()
		require.NoError(t, err)

		cmd = exec.Command("git", "commit", "-m", "initial commit")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Some User",
			"GIT_AUTHOR_EMAIL=some@example.com",
			"GIT_COMMITTER_NAME=Some User",
			"GIT_COMMITTER_EMAIL=some@example.com",
		)
		require.NoError(t, cmd.Run())

		cmd = exec.Command("git", "config", "receive.denyCurrentBranch", "updateInstead")
		cmd.Dir = dir
		err = cmd.Run()
		require.NoError(t, err)

		server, err := git.NewServer(dir, internal.NewStandardWriter())
		require.NoError(t, err)
		t.Cleanup(func() {
			server.Close()
		})

		return server, dir
	}

	t.Run("allows fetch and push", func(t *testing.T) {
		server, remoteDir := setup(t)

		dir, err := os.MkdirTemp("", "git-client-test")
		require.NoError(t, err)
		defer os.RemoveAll(dir)

		cmd := exec.Command("git", "clone", fmt.Sprintf("http://127.0.0.1:%d/.git", server.Port()), dir)
		err = cmd.Run()
		require.NoError(t, err)

		path := filepath.Join(dir, "test.txt")
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "initial content\n", string(content))

		err = os.WriteFile(path, []byte("modified content\n"), 0644)
		require.NoError(t, err)

		cmd = exec.Command("git", "add", "test.txt")
		cmd.Dir = dir
		require.NoError(t, cmd.Run())

		cmd = exec.Command("git", "commit", "-m", "modify test file")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Other User",
			"GIT_AUTHOR_EMAIL=other@example.com",
			"GIT_COMMITTER_NAME=Other User",
			"GIT_COMMITTER_EMAIL=other@example.com",
		)
		require.NoError(t, cmd.Run())

		cmd = exec.Command("git", "branch", "--show-current")
		cmd.Dir = dir
		output, err := cmd.Output()
		require.NoError(t, err)

		branch := strings.TrimSpace(string(output))

		cmd = exec.Command("git", "push", "origin", branch)
		cmd.Dir = dir
		err = cmd.Run()
		require.NoError(t, err)

		content, err = os.ReadFile(filepath.Join(remoteDir, "test.txt"))
		require.NoError(t, err)
		require.Equal(t, "modified content\n", string(content))
	})

	t.Run("failure cases", func(t *testing.T) {
		t.Run("when the directory is not a git repo", func(t *testing.T) {
			_, err := git.NewServer("/tmp", internal.NewStandardWriter())
			require.ErrorContains(t, err, "not a git repository")
		})
	})
}
