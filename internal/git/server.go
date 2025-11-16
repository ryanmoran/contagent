package git

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/cgi"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/ryanmoran/contagent/internal"
)

type Server struct {
	server   *http.Server
	listener net.Listener
	port     int
	writer   internal.Writer
}

// NewServer creates and starts a Git HTTP server that serves the repository at the specified path.
// The server listens on a random port on localhost and uses git-http-backend CGI to handle
// Git protocol requests. It enables push and pull operations. Returns a Server handle or an error
// if the path is invalid, not a Git repository, the TCP listener cannot be created, or git is not
// found in PATH. The server starts immediately in a background goroutine.
func NewServer(path string, w internal.Writer) (Server, error) {
	var err error
	path, err = filepath.Abs(path)
	if err != nil {
		return Server{}, fmt.Errorf("failed to resolve absolute path for %q: %w\nCheck that the path exists and is accessible", path, err)
	}

	if _, err := os.Stat(filepath.Join(path, ".git")); os.IsNotExist(err) {
		return Server{}, fmt.Errorf("not a git repository: %q\nRun 'git init' to initialize a repository or navigate to an existing one", path)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return Server{}, fmt.Errorf("failed to create TCP listener on localhost: %w\nAnother process may be using network resources", err)
	}

	git, err := exec.LookPath("git")
	if err != nil {
		return Server{}, fmt.Errorf("git binary not found in PATH: %w\nInstall git or ensure it's in your PATH environment variable", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		h := &cgi.Handler{
			Path: git,
			Args: []string{
				"-c", "http.receivepack",
				"http-backend",
			},
			Dir: path,
			Env: []string{
				"GIT_PROJECT_ROOT=" + path,
				"PATH_INFO=" + r.URL.Path,
				"QUERY_STRING=" + r.URL.RawQuery,
				"REQUEST_METHOD=" + r.Method,
				"GIT_HTTP_EXPORT_ALL=true",
				"GIT_HTTP_ALLOW_REPACK=true",
				"GIT_HTTP_ALLOW_PUSH=true",
				"GIT_HTTP_VERBOSE=1",
				"SSH_AUTH_SOCK=" + os.Getenv("SSH_AUTH_SOCK"),
			},
			Logger: log.New(os.Stdout, "[GIT SERVER}", 0),
			Stderr: os.Stderr,
		}

		h.ServeHTTP(w, r)
	})

	server := &http.Server{
		Handler: mux,
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			w.Warningf("Git server error: %v", err)
		}
	}()

	_, portString, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return Server{}, fmt.Errorf("failed to split listener host/port: %w", err)
	}

	port, err := strconv.ParseInt(portString, 10, 64)
	if err != nil {
		return Server{}, fmt.Errorf("failed to parse listener port: %w", err)
	}

	return Server{
		listener: listener,
		server:   server,
		port:     int(port),
		writer:   w,
	}, nil
}

// Port returns the TCP port number that the Git server is listening on.
func (s Server) Port() int {
	return s.port
}

// Close stops the Git HTTP server and closes the TCP listener.
// Returns an error if the server or listener cannot be closed cleanly.
func (s Server) Close() error {
	err := s.server.Close()
	if err != nil {
		return err
	}

	return s.listener.Close()
}
