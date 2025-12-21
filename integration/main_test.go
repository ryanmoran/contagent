package integration_test

import (
	"log"
	"os"
	"os/exec"
	"testing"
)

var settings struct {
	Path string
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

	cmd := exec.Command("go", "build", "-o", settings.Path, "../.")
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
