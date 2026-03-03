package integration_test

import (
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"
)

var settings struct {
	Path string
}

// cleanEnv returns a copy of os.Environ() with config-related variables
// neutralized so that integration tests are isolated from the user's
// global and project configuration files. HOME is pointed at a test-scoped
// temp directory and CONTAGENT_GLOBAL_CONFIG_FILE / XDG_CONFIG_HOME are
// stripped.
func cleanEnv(t *testing.T) []string {
	t.Helper()

	var out []string
	for _, v := range os.Environ() {
		key, _, _ := strings.Cut(v, "=")
		switch key {
		case "HOME", "XDG_CONFIG_HOME", "CONTAGENT_GLOBAL_CONFIG_FILE":
			continue
		default:
			out = append(out, v)
		}
	}
	out = append(out, "HOME="+t.TempDir())
	return out
}

func TestMain(m *testing.M) {
	err := BeforeSuite()
	if err != nil {
		log.Fatal(err)
	}

	code := m.Run()

	err = AfterSuite()
	if err != nil {
		log.Fatal(err)
	}

	os.Exit(code)
}

func BeforeSuite() error {
	file, err := os.CreateTemp("", "contagent-*")
	if err != nil {
		return err
	}

	err = file.Close()
	if err != nil {
		return err
	}

	settings.Path = file.Name()

	cmd := exec.Command("go", "build", "-o", settings.Path, "../.") //nolint:gosec // G204: Test with controlled input
	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func AfterSuite() error {
	err := os.RemoveAll(settings.Path)
	if err != nil {
		return err
	}

	return nil
}
