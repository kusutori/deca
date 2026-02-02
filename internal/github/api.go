package github

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v60/github"
	"golang.org/x/sync/errgroup"
)

// Client wraps the GitHub client
type Client struct {
	client *github.Client
	token  string
}

// NewClient creates a new GitHub client
func NewClient() *Client {
	token := os.Getenv("GITHUB_TOKEN")
	var client *http.Client
	if token != "" {
		client = &http.Client{
			Transport: &tokenTransport{token: token},
		}
	}
	return &Client{
		client: github.NewClient(client),
		token:  token,
	}
}

// tokenTransport implements http.RoundTripper for authentication
type tokenTransport struct {
	token string
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)
	return http.DefaultTransport.RoundTrip(req)
}

// ReleaseInfo contains information about a release
type ReleaseInfo struct {
	TagName   string      `json:"tag_name"`
	Assets    []AssetInfo `json:"assets"`
	Repo      string      `json:"repo"`
	Owner     string      `json:"owner"`
	URL       string      `json:"url"`
	Published string      `json:"published_at"`
}

// AssetInfo contains information about a release asset
type AssetInfo struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int64  `json:"size"`
}

// GetLatestRelease returns the latest release for a repository
func (c *Client) GetLatestRelease(ctx context.Context, owner, repo string) (*ReleaseInfo, error) {
	release, _, err := c.client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest release: %w", err)
	}

	return toReleaseInfo(release, owner, repo), nil
}

// GetReleaseByTag returns a specific release by tag
func (c *Client) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*ReleaseInfo, error) {
	release, _, err := c.client.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil {
		return nil, fmt.Errorf("failed to get release %s: %w", tag, err)
	}

	return toReleaseInfo(release, owner, repo), nil
}

// toReleaseInfo converts GitHub release to our format
func toReleaseInfo(release *github.RepositoryRelease, owner, repo string) *ReleaseInfo {
	assets := make([]AssetInfo, len(release.Assets))
	for i, asset := range release.Assets {
		assets[i] = AssetInfo{
			Name:        asset.GetName(),
			DownloadURL: asset.GetBrowserDownloadURL(),
			Size:        int64(asset.GetSize()),
		}
	}

	published := ""
	if release.PublishedAt != nil {
		published = release.PublishedAt.Format(time.RFC3339)
	}

	return &ReleaseInfo{
		TagName:   release.GetTagName(),
		Assets:    assets,
		Repo:      repo,
		Owner:     owner,
		URL:       release.GetHTMLURL(),
		Published: published,
	}
}

// ParseRepo parses an owner/repo string
func ParseRepo(repo string) (owner, name string, err error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo format: %s (expected owner/repo)", repo)
	}
	return parts[0], parts[1], nil
}

// FindMatchingAsset finds an asset matching the given pattern
func FindMatchingAsset(release *ReleaseInfo, pattern string, os, arch string) (*AssetInfo, error) {
	if pattern == "" {
		// Auto-detect based on OS/arch
		pattern = guessAssetPattern(os, arch)
	}

	var candidates []*AssetInfo

	for i := range release.Assets {
		asset := &release.Assets[i]
		if matchesAsset(asset, pattern, os, arch) {
			candidates = append(candidates, asset)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no matching asset found for pattern: %s", pattern)
	}

	if len(candidates) > 1 {
		// Prefer exact os/arch matches
		for _, c := range candidates {
			if matchesOSArch(c.Name, os, arch) {
				return c, nil
			}
		}
		// Return first match
		return candidates[0], nil
	}

	return candidates[0], nil
}

// matchesAsset checks if an asset matches the given pattern
func matchesAsset(asset *AssetInfo, pattern string, os, arch string) bool {
	// Check pattern match
	if pattern != "" && !globMatch(pattern, asset.Name) {
		return false
	}

	// Check OS/arch match
	return matchesOSArch(asset.Name, os, arch)
}

// matchesOSArch checks if the asset name matches the expected OS/arch
func matchesOSArch(name string, os, arch string) bool {
	// If no constraints, accept all
	if os == "" && arch == "" {
		return true
	}

	nameLower := strings.ToLower(name)

	// Check OS
	if os != "" {
		osMatches := false
		switch os {
		case "linux":
			osMatches = strings.Contains(nameLower, "linux") || !containsAny(nameLower, "darwin", "windows")
		case "darwin", "macos":
			osMatches = strings.Contains(nameLower, "darwin") || strings.Contains(nameLower, "macos")
		case "windows":
			osMatches = strings.Contains(nameLower, "windows") || strings.Contains(nameLower, ".exe")
		}
		if !osMatches {
			return false
		}
	}

	// Check arch
	if arch != "" {
		archMatches := false
		switch arch {
		case "amd64", "x86_64":
			archMatches = strings.Contains(nameLower, "x86_64") || strings.Contains(nameLower, "amd64")
		case "arm64", "aarch64":
			archMatches = strings.Contains(nameLower, "arm64") || strings.Contains(nameLower, "aarch64")
		case "arm", "armv7":
			archMatches = strings.Contains(nameLower, "armv7") || strings.Contains(nameLower, "armhf")
		case "386", "i386":
			archMatches = strings.Contains(nameLower, "386") || strings.Contains(nameLower, "i386")
		}
		if !archMatches {
			return false
		}
	}

	return true
}

// globMatch does simple glob pattern matching
func globMatch(pattern, name string) bool {
	// Convert glob pattern to regex
	regexPattern := "^" + regexp.QuoteMeta(pattern) + "$"

	// Replace common glob wildcards
	regexPattern = strings.ReplaceAll(regexPattern, `\*`, ".*")
	regexPattern = strings.ReplaceAll(regexPattern, `\?`, ".")

	matched, _ := regexp.MatchString(regexPattern, name)
	return matched
}

// guessAssetPattern guesses the asset pattern based on OS and arch
func guessAssetPattern(os, arch string) string {
	var parts []string

	switch os {
	case "linux":
		parts = append(parts, "linux")
	case "darwin", "macos":
		parts = append(parts, "darwin")
	case "windows":
		parts = append(parts, "windows")
	}

	switch arch {
	case "amd64", "x86_64":
		parts = append(parts, "x86_64", "amd64")
	case "arm64", "aarch64":
		parts = append(parts, "arm64", "aarch64")
	}

	if len(parts) > 0 {
		return strings.Join(parts, ".*")
	}
	return ""
}

// containsAny checks if the string contains any of the given substrings
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// FetchMultipleReleases fetches releases for multiple repos in parallel
func (c *Client) FetchMultipleReleases(ctx context.Context, repos []struct{ Owner, Repo string }) ([]*ReleaseInfo, []error) {
	g, ctx := errgroup.WithContext(ctx)
	mu := &sync.Mutex{}
	results := make([]*ReleaseInfo, len(repos))
	var errors []error

	for i, r := range repos {
		i, r := i, r // Capture range variables
		g.Go(func() error {
			release, err := c.GetLatestRelease(ctx, r.Owner, r.Repo)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("%s/%s: %w", r.Owner, r.Repo, err))
				mu.Unlock()
				return nil // Don't fail the group
			}
			mu.Lock()
			results[i] = release
			mu.Unlock()
			return nil
		})
	}

	g.Wait()
	return results, errors
}
