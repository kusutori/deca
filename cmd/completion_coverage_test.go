package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kusutori/deca/internal/config"
)

func TestDynamicCompletions(t *testing.T) {
	// Carapace attaches Cobra's completion callbacks on first command execution.
	if _, _, err := runCmd(t, "completion", "bash"); err != nil {
		t.Fatalf("initialize completions: %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "deca.toml")
	if err := config.Save(&config.Config{Packages: map[string]config.Package{
		"alpha": {Repo: "owner/alpha"},
		"beta":  {Repo: "owner/beta"},
	}}, path); err != nil {
		t.Fatal(err)
	}
	original := configPath
	configPath = path
	t.Cleanup(func() { configPath = original })

	values, _ := AddCmd.ValidArgsFunction(AddCmd, nil, "x")
	if len(values) != 0 {
		t.Fatalf("short repository query returned values: %v", values)
	}
	assetCompletion, ok := AddCmd.GetFlagCompletionFunc("asset")
	if !ok {
		t.Fatal("asset completion was not registered")
	}
	values, _ = assetCompletion(AddCmd, nil, "")
	if len(values) != 0 {
		t.Fatalf("asset completion without repository returned values: %v", values)
	}
	values, _ = assetCompletion(AddCmd, []string{"invalid"}, "")
	if len(values) != 0 {
		t.Fatalf("asset completion with invalid repository returned values: %v", values)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/repositories":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{
				"full_name": "owner/tool", "description": "A useful tool",
			}}})
		case "/repos/owner/tool/releases/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": "v1.0.0", "assets": []map[string]any{{
				"name": "tool.zip",
			}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	apiURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	originalTransport := http.DefaultTransport
	http.DefaultTransport = &rewriteTransport{base: originalTransport, apiURL: apiURL}
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	values, _ = AddCmd.ValidArgsFunction(AddCmd, nil, "tool")
	if !slices.Contains(values, "owner/tool\tA useful tool") {
		t.Fatalf("repository completion missing result: %v", values)
	}
	values, _ = assetCompletion(AddCmd, []string{"owner/tool"}, "")
	if !slices.Contains(values, "tool.zip") {
		t.Fatalf("asset completion missing result: %v", values)
	}

	values, _ = RemoveCmd.ValidArgsFunction(RemoveCmd, []string{"alpha"}, "")
	if slices.Contains(values, "alpha") || !slices.Contains(values, "beta") {
		t.Fatalf("remove completion did not exclude supplied args: %v", values)
	}
	values, _ = UpdateCmd.ValidArgsFunction(UpdateCmd, nil, "")
	if !slices.Contains(values, "alpha") || !slices.Contains(values, "beta") {
		t.Fatalf("update completion missing packages: %v", values)
	}
	values, _ = StatusCmd.ValidArgsFunction(StatusCmd, nil, "")
	if !slices.Contains(values, "alpha") || !slices.Contains(values, "beta") {
		t.Fatalf("status completion missing packages: %v", values)
	}
}
