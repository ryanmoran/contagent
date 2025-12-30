package internal_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ryanmoran/contagent/internal"
)

func TestConfig(t *testing.T) {
	t.Run("ParseConfig", func(t *testing.T) {
		t.Run("when given a program", func(t *testing.T) {
			args := []string{"some-command", "--some-option"}
			env := []string{
				"TERM=some-term",
				"COLORTERM=some-color-term",
				"ANTHROPIC_API_KEY=some-api-key",
				"OTHER_KEY=other-value",
			}

			config, err := internal.ParseConfig(args, env)
			require.NoError(t, err)
			require.Equal(t, internal.Command([]string{"some-command", "--some-option"}), config.Args)
			require.Equal(t, internal.Environment([]string{
				"TERM=some-term",
				"COLORTERM=some-color-term",
				"ANTHROPIC_API_KEY=some-api-key",
				"SSH_AUTH_SOCK=/run/host-services/ssh-auth.sock",
			}), config.Env)
			require.Equal(t, "default", config.Network)
		})

		t.Run("with --env flags", func(t *testing.T) {
			args := []string{"--env", "VAR1=value1", "--env", "VAR2=value2", "some-program", "--arg"}
			env := []string{
				"TERM=some-term",
				"COLORTERM=some-color-term",
				"ANTHROPIC_API_KEY=some-api-key",
			}

			config, err := internal.ParseConfig(args, env)
			require.NoError(t, err)
			require.Equal(t, internal.Command([]string{"some-program", "--arg"}), config.Args)
			require.ElementsMatch(t, []string{
				"TERM=some-term",
				"COLORTERM=some-color-term",
				"ANTHROPIC_API_KEY=some-api-key",
				"SSH_AUTH_SOCK=/run/host-services/ssh-auth.sock",
				"VAR1=value1",
				"VAR2=value2",
			}, config.Env)
		})

		t.Run("with --volume flags", func(t *testing.T) {
			args := []string{"--volume", "/host/path:/container/path", "some-program"}
			env := []string{
				"TERM=some-term",
			}

			config, err := internal.ParseConfig(args, env)
			require.NoError(t, err)
			require.Equal(t, internal.Command([]string{"some-program"}), config.Args)
			require.Equal(t, []string{
				"/var/run/docker.sock:/var/run/docker.sock",
				"/run/host-services/ssh-auth.sock:/run/host-services/ssh-auth.sock",
				"/host/path:/container/path",
			}, config.Volumes)
		})

		t.Run("with multiple --volume flags", func(t *testing.T) {
			args := []string{
				"--volume", "/host/path1:/container/path1",
				"--volume", "/host/path2:/container/path2",
				"some-program",
			}
			env := []string{
				"TERM=some-term",
			}

			config, err := internal.ParseConfig(args, env)
			require.NoError(t, err)
			require.Equal(t, internal.Command([]string{"some-program"}), config.Args)
			require.Equal(t, []string{
				"/var/run/docker.sock:/var/run/docker.sock",
				"/run/host-services/ssh-auth.sock:/run/host-services/ssh-auth.sock",
				"/host/path1:/container/path1",
				"/host/path2:/container/path2",
			}, config.Volumes)
		})

		t.Run("with mixed --env and --volume flags", func(t *testing.T) {
			args := []string{
				"--env", "VAR1=value1",
				"--volume", "/host/path:/container/path",
				"--env", "VAR2=value2",
				"some-program",
				"--arg",
			}
			env := []string{
				"TERM=some-term",
			}

			config, err := internal.ParseConfig(args, env)
			require.NoError(t, err)
			require.Equal(t, internal.Command([]string{"some-program", "--arg"}), config.Args)
			require.ElementsMatch(t, []string{
				"TERM=some-term",
				"COLORTERM=truecolor",
				"ANTHROPIC_API_KEY=",
				"SSH_AUTH_SOCK=/run/host-services/ssh-auth.sock",
				"VAR1=value1",
				"VAR2=value2",
			}, config.Env)
			require.Equal(t, []string{
				"/var/run/docker.sock:/var/run/docker.sock",
				"/run/host-services/ssh-auth.sock:/run/host-services/ssh-auth.sock",
				"/host/path:/container/path",
			}, config.Volumes)
		})

		t.Run("returns error when config loading fails with invalid retry delay", func(t *testing.T) {
			args := []string{
				"--retry-delay", "not-a-duration",
				"some-program",
			}
			env := []string{
				"TERM=some-term",
			}

			config, err := internal.ParseConfig(args, env)
			require.Error(t, err)
			require.Contains(t, err.Error(), "time: invalid duration")
			require.Equal(t, internal.Config{}, config)
		})

		t.Run("returns error when global config file is set but does not exist", func(t *testing.T) {
			args := []string{"some-program"}
			env := []string{
				"CONTAGENT_GLOBAL_CONFIG_FILE=/nonexistent/config.yaml",
				"TERM=some-term",
			}

			config, err := internal.ParseConfig(args, env)
			require.Error(t, err)
			require.Contains(t, err.Error(), "file does not exist")
			require.Equal(t, internal.Config{}, config)
		})

		t.Run("when given a --dockerfile flag", func(t *testing.T) {
			args := []string{
				"--dockerfile", "/some/path/to/a/Dockerfile",
				"some-program",
			}
			env := []string{
				"TERM=some-term",
			}

			config, err := internal.ParseConfig(args, env)
			require.NoError(t, err)
			require.Equal(t, "/some/path/to/a/Dockerfile", config.DockerfilePath)
		})

		t.Run("when given a --network flag", func(t *testing.T) {
			args := []string{
				"--network", "some-network",
				"some-program",
			}
			env := []string{
				"TERM=some-term",
			}

			config, err := internal.ParseConfig(args, env)
			require.NoError(t, err)
			require.Equal(t, "some-network", config.Network)
		})
	})
}
