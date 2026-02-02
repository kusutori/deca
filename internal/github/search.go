package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v60/github"
)

// SearchRepositories searches for repositories
func (c *Client) SearchRepositories(ctx context.Context, query string) ([]RepositoryResult, error) {
	result, _, err := c.client.Search.Repositories(ctx, query, &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 10},
	})
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	repos := make([]RepositoryResult, 0, len(result.Repositories))
	for _, r := range result.Repositories {
		repos = append(repos, RepositoryResult{
			FullName:  r.GetFullName(),
			Name:      r.GetName(),
			Desc:      r.GetDescription(),
			Stars:     r.GetStargazersCount(),
			UpdatedAt: r.GetUpdatedAt().Format("2006-01-02"),
		})
	}

	return repos, nil
}

// RepositoryResult represents a search result
type RepositoryResult struct {
	FullName  string `json:"full_name"`
	Name      string `json:"name"`
	Desc      string `json:"description"`
	Stars     int    `json:"stars"`
	UpdatedAt string `json:"updated_at"`
}
