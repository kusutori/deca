package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/kusutori/deca/internal/config"
	"github.com/kusutori/deca/internal/github"
)

func releaseForPackage(ctx context.Context, ghClient *github.Client, owner, repo string, pkg *config.Package, forcePrerelease bool) (*github.ReleaseInfo, error) {
	if pkg.Version == "" {
		return ghClient.GetLatestReleaseWithOptions(ctx, owner, repo, pkg.Prerelease || forcePrerelease)
	}

	var lastErr error
	for _, tag := range candidateTags(pkg.Version) {
		release, err := ghClient.GetReleaseByTag(ctx, owner, repo, tag)
		if err == nil {
			return release, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("failed to get release for %s/%s@%s: %w", owner, repo, pkg.Version, lastErr)
}

func candidateTags(version string) []string {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil
	}

	seen := make(map[string]struct{})
	add := func(tag string, tags *[]string) {
		if tag == "" {
			return
		}
		if _, ok := seen[tag]; ok {
			return
		}
		seen[tag] = struct{}{}
		*tags = append(*tags, tag)
	}

	var tags []string
	add(version, &tags)

	if strings.HasPrefix(version, "v") {
		add(strings.TrimPrefix(version, "v"), &tags)
	} else {
		add("v"+version, &tags)
	}

	return tags
}
