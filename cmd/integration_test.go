package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/kusutori/deca/internal/config"
)

type apiServerState struct {
	mu        sync.RWMutex
	tag       string
	assetName string
	assetData []byte
}

func (s *apiServerState) setTag(tag string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tag = tag
}

func (s *apiServerState) getTag() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tag
}

func (s *apiServerState) getAsset() (string, []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.assetName, s.assetData
}

func newAPIServer(t *testing.T, state *apiServerState) *httptest.Server {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if strings.HasPrefix(path, "/download/") {
			_, data := state.getAsset()
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
			return
		}

		if strings.HasSuffix(path, "/releases/latest") {
			tag := state.getTag()
			assetName, assetData := state.getAsset()
			replyReleaseJSON(w, tag, r.Host, assetName, len(assetData))
			return
		}

		if strings.Contains(path, "/releases/tags/") {
			parts := strings.Split(path, "/releases/tags/")
			if len(parts) == 2 && parts[1] != "" {
				tag := parts[1]
				assetName, assetData := state.getAsset()
				replyReleaseJSON(w, tag, r.Host, assetName, len(assetData))
				return
			}
		}

		w.WriteHeader(http.StatusNotFound)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skipping integration test; cannot open listener: %v", err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.Listener = ln
	server.Start()
	return server
}

func replyReleaseJSON(w http.ResponseWriter, tag, host, assetName string, assetSize int) {
	type asset struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int    `json:"size"`
		ID                 int64  `json:"id"`
	}
	resp := struct {
		TagName string  `json:"tag_name"`
		Assets  []asset `json:"assets"`
	}{
		TagName: tag,
		Assets: []asset{
			{
				Name:               assetName,
				BrowserDownloadURL: "http://" + host + "/download/" + assetName,
				Size:               assetSize,
				ID:                 1,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type rewriteTransport struct {
	base   http.RoundTripper
	apiURL *url.URL
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "api.github.com" {
		clone := req.Clone(context.Background())
		clone.URL.Scheme = t.apiURL.Scheme
		clone.URL.Host = t.apiURL.Host
		clone.Host = t.apiURL.Host
		req = clone
	}
	return t.base.RoundTrip(req)
}

func runCmd(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	oldCmdOut := RootCmd.OutOrStdout()
	oldCmdErr := RootCmd.ErrOrStderr()
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr
	RootCmd.SetOut(wOut)
	RootCmd.SetErr(wErr)

	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		RootCmd.SetOut(oldCmdOut)
		RootCmd.SetErr(oldCmdErr)
	}()

	verbose = false
	dryRun = false

	RootCmd.SetArgs(args)
	err := RootCmd.Execute()

	wOut.Close()
	wErr.Close()

	outBytes, _ := io.ReadAll(rOut)
	errBytes, _ := io.ReadAll(rErr)

	return string(outBytes), string(errBytes), err
}

func assetNameForRuntime(name string) string {
	osToken := runtime.GOOS
	if osToken == "darwin" {
		osToken = "darwin"
	}
	archToken := runtime.GOARCH
	return name + "-" + osToken + "-" + archToken + ".tar.gz"
}

func tarGzBytes(filename string, content []byte) []byte {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	_ = tw.WriteHeader(&tar.Header{
		Name: filename,
		Mode: 0755,
		Size: int64(len(content)),
	})
	_, _ = tw.Write(content)

	_ = tw.Close()
	_ = gw.Close()
	return buf.Bytes()
}

func TestCmdIntegrationWorkflow(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")
	defer func() {
		_ = os.Setenv("HOME", origHome)
		_ = os.Setenv("USERPROFILE", origUserProfile)
	}()
	_ = os.Setenv("HOME", tmpHome)
	_ = os.Setenv("USERPROFILE", tmpHome)

	origConfigPath := configPath
	defer func() { configPath = origConfigPath }()
	configPath = ""
	cfgPath := filepath.Join(tmpHome, ".config", "deca", "deca.toml")
	run := func(args ...string) (string, string, error) {
		return runCmd(t, append([]string{"--config", cfgPath}, args...)...)
	}

	binName := "tool"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	assetName := assetNameForRuntime("tool")
	assetData := tarGzBytes(binName, []byte("#!/bin/sh\necho ok\n"))

	state := &apiServerState{
		tag:       "v1.0.0",
		assetName: assetName,
		assetData: assetData,
	}

	apiServer := newAPIServer(t, state)
	defer apiServer.Close()

	apiURL, err := url.Parse(apiServer.URL)
	if err != nil {
		t.Fatalf("failed to parse api server url: %v", err)
	}

	oldTransport := http.DefaultTransport
	http.DefaultTransport = &rewriteTransport{base: oldTransport, apiURL: apiURL}
	defer func() { http.DefaultTransport = oldTransport }()

	if _, _, err := run("init"); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	if _, _, err := run("add", "acme/tool"); err != nil {
		t.Fatalf("add failed: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if _, ok := cfg.Packages["tool"]; !ok {
		t.Fatalf("expected package 'tool' in config")
	}

	statePath := config.DefaultStatePath()
	installState, err := config.LoadState(statePath)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if pkg, ok := installState.GetPackage("tool"); !ok || pkg.Version != "v1.0.0" {
		t.Fatalf("expected tool v1.0.0 installed, got %+v", pkg)
	}

	binPath := filepath.Join(cfg.BinDir, "tool")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	if _, err := os.Stat(binPath); err != nil {
		t.Fatalf("expected binary installed at %s: %v", binPath, err)
	}

	state.setTag("v1.1.0")
	_, errOut, err := run("status")
	if err != nil {
		t.Fatalf("status failed: %v (%s)", err, errOut)
	}

	if _, _, err := run("update"); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	installState, err = config.LoadState(statePath)
	if err != nil {
		t.Fatalf("failed to load state after update: %v", err)
	}
	if pkg, ok := installState.GetPackage("tool"); !ok || pkg.Version != "v1.1.0" {
		t.Fatalf("expected tool v1.1.0 installed, got %+v", pkg)
	}

	if _, _, err := run("remove", "tool"); err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	installState, err = config.LoadState(statePath)
	if err != nil {
		t.Fatalf("failed to load state after remove: %v", err)
	}
	if _, ok := installState.GetPackage("tool"); ok {
		t.Fatalf("expected tool removed from state")
	}
	if _, err := os.Stat(binPath); !os.IsNotExist(err) {
		t.Fatalf("expected binary removed from %s", binPath)
	}

	if _, _, err := run("add", "acme/tool", "--no-install"); err != nil {
		t.Fatalf("add --no-install failed: %v", err)
	}

	state.setTag("v1.2.0")
	if _, _, err := run("apply"); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	installState, err = config.LoadState(statePath)
	if err != nil {
		t.Fatalf("failed to load state after apply: %v", err)
	}
	if pkg, ok := installState.GetPackage("tool"); !ok || pkg.Version != "v1.2.0" {
		t.Fatalf("expected tool v1.2.0 installed, got %+v", pkg)
	}
}
