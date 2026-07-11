package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/kusutori/deca/internal/ui"
)

func TestInteractiveSelectAssetNonInteractive(t *testing.T) {
	server := newAPIServer(t, &apiServerState{
		tag:       "v1.0.0",
		assetName: "tool-windows-amd64.zip",
		assetData: []byte("archive"),
	})
	defer server.Close()
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	originalTransport := http.DefaultTransport
	http.DefaultTransport = &rewriteTransport{base: originalTransport, apiURL: serverURL}
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	asset, err := interactiveSelectAsset("owner", "tool", false)
	if err != nil {
		t.Fatalf("interactiveSelectAsset: %v", err)
	}
	if asset == nil || asset.Name != "tool-windows-amd64.zip" {
		t.Fatalf("unexpected selected asset: %+v", asset)
	}

}

func TestInteractiveSelectAssetWithoutAssets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": "v1.0.0", "assets": []any{}})
	}))
	defer server.Close()
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	originalTransport := http.DefaultTransport
	http.DefaultTransport = &rewriteTransport{base: originalTransport, apiURL: serverURL}
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	asset, err := interactiveSelectAsset("owner", "tool", false)
	if err != nil {
		t.Fatalf("interactiveSelectAsset: %v", err)
	}
	if asset != nil {
		t.Fatalf("selected asset = %+v, want nil", asset)
	}
}

func TestInteractiveSelectAssetTerminalSelection(t *testing.T) {
	server := newAPIServer(t, &apiServerState{
		tag:       "v1.0.0",
		assetName: "tool.zip",
		assetData: []byte("archive"),
	})
	defer server.Close()
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	originalTransport := http.DefaultTransport
	http.DefaultTransport = &rewriteTransport{base: originalTransport, apiURL: serverURL}
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	restoreTerminal := ui.SetTerminalDetectorForTesting(func() bool { return true })
	t.Cleanup(restoreTerminal)

	originalStdin := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.WriteString("1\n"); err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = originalStdin
		_ = reader.Close()
	})

	asset, err := interactiveSelectAsset("owner", "tool", false)
	if err != nil {
		t.Fatalf("interactiveSelectAsset: %v", err)
	}
	if asset == nil || asset.Name != "tool.zip" {
		t.Fatalf("unexpected selected asset: %+v", asset)
	}
}
