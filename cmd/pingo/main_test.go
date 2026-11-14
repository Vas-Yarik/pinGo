package main

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func runWithConfig(t *testing.T, configPath string, extra ...string) (int, string) {
	t.Helper()
	args := append(append([]string{}, extra...), "-config", configPath)

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	code := run(args)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = oldStdout

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return code, string(out)
}

func TestRunAllTargetsUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	code, out := runWithConfig(t, writeConfig(t, "targets:\n  - name: test\n    url: "+srv.URL+"\n"))
	if code != exitOK {
		t.Fatalf("want exit code %d, got %d\noutput:\n%s", exitOK, code, out)
	}
	if !strings.Contains(out, "1/1 targets up") {
		t.Errorf("want summary \"1/1 targets up\", got:\n%s", out)
	}
}

func TestRunCheckSubcommand(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	code, out := runWithConfig(t, writeConfig(t, "targets:\n  - name: test\n    url: "+srv.URL+"\n"), "check")
	if code != exitOK {
		t.Fatalf("want exit code %d, got %d\noutput:\n%s", exitOK, code, out)
	}
	if !strings.Contains(out, "HTTP 200") {
		t.Errorf("want detail column \"HTTP 200\", got:\n%s", out)
	}
}

func TestRunSomeTargetsDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dead := "http://" + l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	const content = `
targets:
  - name: up
    url: UP_URL
  - name: down
    url: DOWN_URL
    timeout: 2s
`
	code, out := runWithConfig(t, writeConfig(t,
		strings.NewReplacer("UP_URL", srv.URL, "DOWN_URL", dead).Replace(content)))
	if code != exitDown {
		t.Fatalf("want exit code %d, got %d\noutput:\n%s", exitDown, code, out)
	}
	if !strings.Contains(out, "1/2 targets up") {
		t.Errorf("want summary \"1/2 targets up\", got:\n%s", out)
	}
}

func TestRunTCPTargetUp(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	code, out := runWithConfig(t, writeConfig(t,
		"targets:\n  - name: db\n    type: tcp\n    addr: "+l.Addr().String()+"\n"))
	if code != exitOK {
		t.Fatalf("want exit code %d, got %d\noutput:\n%s", exitOK, code, out)
	}
	if !strings.Contains(out, "tcp "+l.Addr().String()) {
		t.Errorf("want tcp detail, got:\n%s", out)
	}
}

func TestRunWatchRequiresTerminal(t *testing.T) {
	code, _ := runWithConfig(t, writeConfig(t, "targets:\n  - name: test\n    url: https://example.com\n"), "watch")
	if code != exitError {
		t.Fatalf("want exit code %d for watch without a TTY, got %d", exitError, code)
	}
}

func TestRunBadConfig(t *testing.T) {
	code, _ := runWithConfig(t, filepath.Join(t.TempDir(), "missing.yaml"))
	if code != exitError {
		t.Fatalf("want exit code %d, got %d", exitError, code)
	}
}
