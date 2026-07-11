package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kusutori/deca/internal/config"
	"github.com/kusutori/deca/internal/github"
	"github.com/kusutori/deca/internal/install"
)

func TestPackageFlowsRejectInvalidOrIneligiblePackages(t *testing.T) {
	ctx := context.Background()
	client := github.NewClient()
	installer := install.NewInstaller(t.TempDir())
	state := &config.State{Packages: map[string]config.InstalledPackage{}}

	invalidCondition := &config.Package{Repo: "owner/tool", OS: "("}
	if _, err := installPackage(ctx, client, installer, "tool", invalidCondition, state, runtime.GOOS, runtime.GOARCH, false); err == nil {
		t.Fatal("installPackage accepted invalid condition")
	}
	if err := doInstall(ctx, client, installer, "tool", invalidCondition, false); err == nil {
		t.Fatal("doInstall accepted invalid condition")
	}

	ineligible := &config.Package{Repo: "owner/tool", OS: "not-" + runtime.GOOS}
	result, err := installPackage(ctx, client, installer, "tool", ineligible, state, runtime.GOOS, runtime.GOARCH, false)
	if err != nil || result != "" {
		t.Fatalf("installPackage ineligible = %q, %v", result, err)
	}
	if err := doInstall(ctx, client, installer, "tool", ineligible, false); err != nil {
		t.Fatalf("doInstall ineligible: %v", err)
	}

	invalidRepo := &config.Package{Repo: "invalid-repository"}
	if _, err := installPackage(ctx, client, installer, "tool", invalidRepo, state, runtime.GOOS, runtime.GOARCH, false); err == nil {
		t.Fatal("installPackage accepted invalid repository")
	}
	if err := doInstall(ctx, client, installer, "tool", invalidRepo, false); err == nil {
		t.Fatal("doInstall accepted invalid repository")
	}

	if updated, err := checkUpdate(ctx, client, "tool", invalidCondition, state, runtime.GOOS, runtime.GOARCH); err == nil || updated {
		t.Fatalf("checkUpdate invalid condition = %v, %v", updated, err)
	}
	if updated, err := checkUpdate(ctx, client, "tool", ineligible, state, runtime.GOOS, runtime.GOARCH); err != nil || updated {
		t.Fatalf("checkUpdate ineligible = %v, %v", updated, err)
	}
	if updated, err := checkUpdate(ctx, client, "tool", invalidRepo, state, runtime.GOOS, runtime.GOARCH); err == nil || updated {
		t.Fatalf("checkUpdate invalid repository = %v, %v", updated, err)
	}
}

func TestPackageFlowsReportReleaseFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	originalTransport := http.DefaultTransport
	http.DefaultTransport = &rewriteTransport{base: originalTransport, apiURL: serverURL}
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	ctx := context.Background()
	client := github.NewClient()
	installer := install.NewInstaller(t.TempDir())
	state := &config.State{Packages: map[string]config.InstalledPackage{}}
	pkg := &config.Package{Repo: "owner/tool"}
	if _, err := installPackage(ctx, client, installer, "tool", pkg, state, runtime.GOOS, runtime.GOARCH, false); err == nil {
		t.Fatal("installPackage accepted failed release request")
	}
	if err := doInstall(ctx, client, installer, "tool", pkg, false); err == nil {
		t.Fatal("doInstall accepted failed release request")
	}
	if updated, err := checkUpdate(ctx, client, "tool", pkg, state, runtime.GOOS, runtime.GOARCH); err == nil || updated {
		t.Fatalf("checkUpdate release failure = %v, %v", updated, err)
	}
}

func TestInstallPackageReplacesExistingBinary(t *testing.T) {
	binDir := t.TempDir()
	installer := install.NewInstaller(binDir)
	name := "tool"
	binaryName := name
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	if err := os.WriteFile(filepath.Join(binDir, binaryName), []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}
	archiveName := assetNameForRuntime(name)
	server := newAPIServer(t, &apiServerState{
		tag:       "v2.0.0",
		assetName: archiveName,
		assetData: tarGzBytes(binaryName, []byte("new binary")),
	})
	defer server.Close()
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	originalTransport := http.DefaultTransport
	http.DefaultTransport = &rewriteTransport{base: originalTransport, apiURL: serverURL}
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	state := &config.State{Packages: map[string]config.InstalledPackage{
		name: {Repo: "owner/tool", Version: "v1.0.0", InstallType: config.InstallTypeBinary},
	}}
	pkg := &config.Package{Repo: "owner/tool", Asset: "*", InstallType: "auto"}
	result, err := installPackage(context.Background(), github.NewClient(), installer, name, pkg, state, runtime.GOOS, runtime.GOARCH, false)
	if err != nil {
		t.Fatalf("installPackage replacement: %v", err)
	}
	if result == "" {
		t.Fatal("expected replacement result")
	}
	installed, ok := state.GetPackage(name)
	if !ok || installed.Version != "v2.0.0" {
		t.Fatalf("unexpected state: %+v", installed)
	}
	content, err := os.ReadFile(filepath.Join(binDir, binaryName))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new binary" {
		t.Fatalf("binary content = %q", content)
	}
}

func TestInstallPackageRestoresBackupOnInstallFailure(t *testing.T) {
	binDir := t.TempDir()
	installer := install.NewInstaller(binDir)
	name := "tool"
	binaryName := name
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(binDir, binaryName)
	if err := os.WriteFile(binaryPath, []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}
	server := newAPIServer(t, &apiServerState{
		tag:       "v2.0.0",
		assetName: assetNameForRuntime(name),
		assetData: []byte("not a tar archive"),
	})
	defer server.Close()
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	originalTransport := http.DefaultTransport
	http.DefaultTransport = &rewriteTransport{base: originalTransport, apiURL: serverURL}
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	state := &config.State{Packages: map[string]config.InstalledPackage{
		name: {Repo: "owner/failing", Version: "v1.0.0", InstallType: config.InstallTypeBinary},
	}}
	pkg := &config.Package{Repo: "owner/failing", Asset: "*", InstallType: "auto"}
	if _, err := installPackage(context.Background(), github.NewClient(), installer, name, pkg, state, runtime.GOOS, runtime.GOARCH, false); err == nil {
		t.Fatal("expected installation failure")
	}
	content, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "old binary" {
		t.Fatalf("backup was not restored: %q", content)
	}
}
