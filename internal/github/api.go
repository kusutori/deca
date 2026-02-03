package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/deca-org/deca/internal/config"
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
	Digest      string `json:"digest"` // SHA256 digest in format "sha256:..."
	// TODO: When go-github library is updated to include the 'digest' field,
	// use asset.GetDigest() instead of the custom API call in GetAssetDigest.
	// Tracking: https://github.com/google/go-github/issues/XXXX
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
		originalURL := asset.GetBrowserDownloadURL()
		assets[i] = AssetInfo{
			Name:        asset.GetName(),
			DownloadURL: TransformDownloadURL(originalURL, owner, repo, release.GetTagName()),
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

// TransformDownloadURL transforms a download URL to use the current mirror
func TransformDownloadURL(originalURL, owner, repo, tag string) string {
	// Get current mirror
	mirror := config.DefaultMirrorConfig().GetCurrentMirror()
	if mirror == nil || mirror.Name == "GitHub (Official)" {
		return originalURL
	}

	// Get the asset name from the URL
	assetName := originalURL[strings.LastIndex(originalURL, "/")+1:]

	// Build the new URL using the mirror's download pattern
	downloadURL := mirror.DownloadURL
	downloadURL = strings.ReplaceAll(downloadURL, "{owner}", owner)
	downloadURL = strings.ReplaceAll(downloadURL, "{repo}", repo)
	downloadURL = strings.ReplaceAll(downloadURL, "{tag}", tag)
	downloadURL = strings.ReplaceAll(downloadURL, "{asset}", assetName)

	return downloadURL
}

// GetAssetDownloadURL returns the download URL for an asset, potentially transformed for mirror
func GetAssetDownloadURL(assetName, owner, repo, tag string) string {
	// Build the original GitHub URL
	originalURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", owner, repo, tag, assetName)
	return TransformDownloadURL(originalURL, owner, repo, tag)
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
	var candidates []*AssetInfo

	for i := range release.Assets {
		asset := &release.Assets[i]
		// Check OS/arch match first (primary constraint)
		if !matchesOSArch(asset.Name, os, arch) {
			continue
		}
		// If pattern is specified, also check pattern match
		if pattern != "" && !globMatch(pattern, asset.Name) {
			continue
		}
		candidates = append(candidates, asset)
	}

	if len(candidates) == 0 {
		// Provide helpful error message
		if pattern == "" {
			return nil, fmt.Errorf("no matching asset found for os=%s arch=%s", os, arch)
		}
		return nil, fmt.Errorf("no matching asset found for pattern: %s", pattern)
	}

	if len(candidates) > 1 {
		// Multiple matches - prefer native binaries over system packages
		// Priority: binary > tar.gz/zip > AppImage > deb > rpm
		best := selectBestAsset(candidates, os)
		return best, nil
	}

	return candidates[0], nil
}

// selectBestAsset selects the best asset from multiple candidates
// Priority: native binary > archive > deb > AppImage > rpm
func selectBestAsset(candidates []*AssetInfo, os string) *AssetInfo {
	// Priority order for package types
	// Higher priority number = more preferred
	priority := []struct {
		suffixes []string
		priority int
	}{
		// Native binaries (no archive, just the binary)
		{[]string{}, 10},
		// Archive formats (tar.gz, zip, etc.)
		{[]string{".tar.gz", ".tgz", ".tar.xz", ".txz", ".zip"}, 8},
		// Debian packages - prefer native package manager format
		{[]string{".deb"}, 6},
		// AppImage - portable but requires FUSE
		{[]string{".appimage"}, 4},
		// RPM packages - last resort on Debian/Ubuntu
		{[]string{".rpm"}, 2},
	}

	for _, p := range priority {
		for _, asset := range candidates {
			for _, suffix := range p.suffixes {
				if suffix == "" {
					// No suffix - could be a native binary
					// Accept if no known archive/package suffix
					name := strings.ToLower(asset.Name)
					if !hasArchiveSuffix(name) {
						return asset
					}
				} else if strings.HasSuffix(strings.ToLower(asset.Name), suffix) {
					return asset
				}
			}
		}
	}

	// Fallback to first candidate
	return candidates[0]
}

// hasArchiveSuffix checks if filename has archive/package suffix
func hasArchiveSuffix(name string) bool {
	archiveSuffixes := []string{".tar.gz", ".tgz", ".tar.xz", ".txz", ".zip", ".appimage", ".deb", ".rpm", ".exe"}
	for _, suffix := range archiveSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// matchesAsset checks if an asset matches the given pattern
func matchesAsset(asset *AssetInfo, pattern string, os, arch string) bool {
	// First check OS/arch match (this is the primary constraint)
	if !matchesOSArch(asset.Name, os, arch) {
		return false
	}

	// Then check pattern match if specified
	if pattern != "" && !globMatch(pattern, asset.Name) {
		return false
	}

	return true
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
			// Must contain "linux" OR (be an archive/binary without any OS indicator AND not contain other OS names)
			// Explicit check: must have "linux" in name, or no OS indicator at all
			if strings.Contains(nameLower, "linux") {
				osMatches = true
			} else if !containsAny(nameLower, "darwin", "macos", "windows", "freebsd", "openbsd", "netbsd", "dragonfly", "solaris", "illumos") {
				// Only accept if it doesn't contain ANY known OS name
				// This handles cases where binaries are named without OS suffix
				osMatches = true
			}
		case "darwin", "macos":
			osMatches = strings.Contains(nameLower, "darwin") || strings.Contains(nameLower, "macos") || strings.Contains(nameLower, "apple")
		case "windows":
			// Windows files may contain "windows" or end with ".exe"
			osMatches = strings.Contains(nameLower, "windows") || strings.HasSuffix(nameLower, ".exe")
		case "freebsd":
			osMatches = strings.Contains(nameLower, "freebsd")
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
		// For Windows, if no arch is found in filename, still accept it
		// Many Windows binaries don't include arch in the filename
		if !archMatches && (os == "windows" || !strings.HasSuffix(nameLower, ".exe")) {
			// On Windows, .exe files without arch are acceptable
			// On other platforms, require arch match
			if os != "windows" && strings.HasSuffix(nameLower, ".exe") {
				return false
			}
			if os == "windows" && strings.HasSuffix(nameLower, ".exe") {
				archMatches = true // Accept .exe files without explicit arch
			}
		}
		if !archMatches {
			return false
		}
	}

	return true
}

// globMatch does simple glob pattern matching
func globMatch(pattern, name string) bool {
	// Convert glob pattern to regex - the pattern should be treated as a contains match
	// not exact match, so we don't use ^ and $
	regexPattern := regexp.QuoteMeta(pattern)

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
		// Windows binaries often don't include arch in filename
		return strings.Join(parts, ".*")
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

// GetAssetDigest fetches the SHA256 digest for a specific asset from GitHub API
// Returns the digest in format "sha256:..." or empty string if not available
func (c *Client) GetAssetDigest(ctx context.Context, owner, repo string, assetID int64) string {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/assets/%d", owner, repo, assetID)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return ""
	}

	// Accept header for release asset
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	// Parse the response to get the digest field
	var result struct {
		Digest string `json:"digest"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}

	return result.Digest
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
